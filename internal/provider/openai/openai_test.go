package openai

import (
	"context"
	"encoding/json"
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
	if !strings.Contains(requestBody, `"include_usage":true`) {
		t.Fatalf("request body does not request streamed usage: %s", requestBody)
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

func TestClientStreamsToolCallsUsageAndEnd(t *testing.T) {
	var requestBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requestBody = string(body)

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"read_file\",\"arguments\":\"{\\\"pa\"}}]}}],\"usage\":null}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"th\\\":\\\"main.go\\\"}\"}}]},\"finish_reason\":\"tool_calls\"}],\"usage\":null}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":3,\"total_tokens\":10}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := New(config.Config{Protocol: config.ProtocolOpenAI, Model: "m", BaseURL: server.URL, APIKey: "k"})

	events, err := collectStream(client, provider.ChatRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "read"}},
		Tools: []provider.ToolSchema{{
			Name:        "read_file",
			Description: "Read a file",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	assertHasEvent(t, events, provider.EventToolCallStart, func(event provider.StreamEvent) bool {
		return event.ToolCall.ID == "call_1" && event.ToolCall.Name == "read_file"
	})
	assertHasEvent(t, events, provider.EventToolCallDelta, func(event provider.StreamEvent) bool {
		return event.ToolCall.ArgumentsDelta == `{"pa`
	})
	assertHasEvent(t, events, provider.EventToolCallComplete, func(event provider.StreamEvent) bool {
		return event.ToolCall.Arguments == `{"path":"main.go"}`
	})
	assertHasEvent(t, events, provider.EventUsage, func(event provider.StreamEvent) bool {
		return event.Usage != nil && event.Usage.InputTokens == 7 && event.Usage.OutputTokens == 3 && event.Usage.TotalTokens == 10
	})
	assertHasEvent(t, events, provider.EventStreamEnd, func(event provider.StreamEvent) bool {
		return event.StopReason == "tool_calls" && event.Usage != nil && event.Usage.TotalTokens == 10
	})
	if !strings.Contains(requestBody, `"tools"`) || !strings.Contains(requestBody, `"read_file"`) {
		t.Fatalf("request body does not include tool schema: %s", requestBody)
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

func collectStream(client *Client, req provider.ChatRequest) ([]provider.StreamEvent, error) {
	var events []provider.StreamEvent
	err := client.Stream(context.Background(), req, func(event provider.StreamEvent) error {
		events = append(events, event)
		return nil
	})
	return events, err
}

func assertHasEvent(t *testing.T, events []provider.StreamEvent, eventType provider.EventType, match func(provider.StreamEvent) bool) {
	t.Helper()

	for _, event := range events {
		if event.Type == eventType && match(event) {
			return
		}
	}
	t.Fatalf("missing event %s in %+v", eventType, events)
}
