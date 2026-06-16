package anthropic

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

func TestClientStreamsTextAndThinkingDeltas(t *testing.T) {
	var requestBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "test-key" {
			t.Fatalf("x-api-key = %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		requestBody = string(body)

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: content_block_delta\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"hidden\"}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_delta\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"text_delta\",\"text\":\"visible\"}}\n\n"))
		_, _ = w.Write([]byte("event: message_stop\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"message_stop\"}\n\n"))
	}))
	defer server.Close()

	client := New(config.Config{
		Protocol:  config.ProtocolAnthropic,
		Model:     "claude-sonnet-4-6",
		BaseURL:   server.URL,
		APIKey:    "test-key",
		MaxTokens: 4096,
		Thinking: &config.ThinkingConfig{
			Enabled:      true,
			BudgetTokens: 1024,
			Display:      "omitted",
		},
	})

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

	if thinking.String() != "hidden" || text.String() != "visible" {
		t.Fatalf("thinking=%q text=%q", thinking.String(), text.String())
	}
	if !strings.Contains(requestBody, `"thinking"`) {
		t.Fatalf("request body does not include thinking config: %s", requestBody)
	}
	if !strings.Contains(requestBody, `"stream":true`) {
		t.Fatalf("request body does not enable streaming: %s", requestBody)
	}
}

func TestClientReturnsStreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: error\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"Overloaded\"}}\n\n"))
	}))
	defer server.Close()

	client := New(config.Config{Protocol: config.ProtocolAnthropic, Model: "m", BaseURL: server.URL, APIKey: "k"})

	err := client.Stream(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	}, func(provider.StreamEvent) error {
		return nil
	})
	if err == nil {
		t.Fatal("Stream() error = nil, want stream error")
	}
	if !strings.Contains(err.Error(), "overloaded_error") {
		t.Fatalf("error = %q, want overloaded_error", err)
	}
}

func TestClientReturnsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad key", http.StatusUnauthorized)
	}))
	defer server.Close()

	client := New(config.Config{Protocol: config.ProtocolAnthropic, Model: "m", BaseURL: server.URL, APIKey: "secret"})

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

func TestAnthropicHelpers(t *testing.T) {
	messages := []provider.Message{
		{Role: provider.RoleSystem, Content: "sys one"},
		{Role: provider.RoleUser, Content: "hello"},
		{Role: provider.RoleSystem, Content: "sys two"},
	}

	if got := systemPrompt(messages); got != "sys one\n\nsys two" {
		t.Fatalf("systemPrompt() = %q", got)
	}
	if got := anthropicMessages(messages); len(got) != 1 || got[0].Role != "user" || got[0].Content != "hello" {
		t.Fatalf("anthropicMessages() = %+v", got)
	}
	if got := maxTokens(0); got != defaultMaxTokens {
		t.Fatalf("maxTokens(0) = %d", got)
	}
	if got := anthropicEndpoint("https://api.anthropic.com"); got != "https://api.anthropic.com/v1/messages" {
		t.Fatalf("anthropicEndpoint() = %q", got)
	}
	if got := anthropicEndpoint("https://api.anthropic.com/v1/messages"); got != "https://api.anthropic.com/v1/messages" {
		t.Fatalf("anthropicEndpoint(existing) = %q", got)
	}
}
