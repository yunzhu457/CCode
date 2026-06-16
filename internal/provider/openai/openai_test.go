package openai

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yunzhu457/CCode/internal/config"
	"github.com/yunzhu457/CCode/internal/provider"
)

func TestClientStreamsTextDeltas(t *testing.T) {
	var requestBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		requestBody = string(body)

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hel\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := New(config.Config{
		Protocol: config.ProtocolOpenAI,
		Model:    "deepseek-v4-flash",
		BaseURL:  server.URL,
		APIKey:   "test-key",
	})

	var got strings.Builder
	err := client.Stream(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	}, func(event provider.StreamEvent) error {
		if event.Type == provider.EventTextDelta {
			got.WriteString(event.Text)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	if got.String() != "Hello" {
		t.Fatalf("streamed text = %q", got.String())
	}
	if !strings.Contains(requestBody, `"stream":true`) {
		t.Fatalf("request body does not enable streaming: %s", requestBody)
	}
}

func TestClientStreamsReasoningAsThinking(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"think\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"answer\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := New(config.Config{Protocol: config.ProtocolOpenAI, Model: "m", BaseURL: server.URL, APIKey: "k"})

	var thinking, text strings.Builder
	err := client.Stream(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	}, func(event provider.StreamEvent) error {
		switch event.Type {
		case provider.EventThinkingDelta:
			thinking.WriteString(event.Thinking)
		case provider.EventTextDelta:
			text.WriteString(event.Text)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	if thinking.String() != "think" || text.String() != "answer" {
		t.Fatalf("thinking=%q text=%q", thinking.String(), text.String())
	}
}

func TestClientReturnsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad key", http.StatusUnauthorized)
	}))
	defer server.Close()

	client := New(config.Config{Protocol: config.ProtocolOpenAI, Model: "m", BaseURL: server.URL, APIKey: "secret"})

	err := client.Stream(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	}, func(provider.StreamEvent) error {
		return nil
	})
	if err == nil {
		t.Fatal("Stream() error = nil, want HTTP error")
	}
	if !strings.Contains(err.Error(), "status 401") {
		t.Fatalf("error = %q, want status", err)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("error leaked API key: %v", err)
	}
}

func TestClientRejectsInvalidStreamJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: not-json\n\n"))
	}))
	defer server.Close()

	client := New(config.Config{Protocol: config.ProtocolOpenAI, Model: "m", BaseURL: server.URL, APIKey: "k"})

	err := client.Stream(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	}, func(provider.StreamEvent) error {
		return nil
	})
	if err == nil {
		t.Fatal("Stream() error = nil, want JSON error")
	}
}

func TestOpenAIEndpoint(t *testing.T) {
	tests := []struct {
		base string
		want string
	}{
		{base: "https://api.deepseek.com", want: "https://api.deepseek.com/chat/completions"},
		{base: "https://api.openai.com/v1", want: "https://api.openai.com/v1/chat/completions"},
		{base: "https://relay.test/chat/completions", want: "https://relay.test/chat/completions"},
	}

	for _, tt := range tests {
		if got := openAIEndpoint(tt.base); got != tt.want {
			t.Fatalf("openAIEndpoint(%q) = %q, want %q", tt.base, got, tt.want)
		}
	}
}
