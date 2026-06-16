package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestClientStreamsThinkingCompleteToolCallsUsageAndEnd(t *testing.T) {
	var requestBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requestBody = string(body)

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: message_start\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":11,\"output_tokens\":1}}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_start\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"thinking\",\"thinking\":\"\"}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_delta\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"hidden\"}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_delta\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"signature_delta\",\"signature\":\"sig_123\"}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_stop\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"content_block_stop\",\"index\":0}\n\n"))
		_, _ = w.Write([]byte("event: content_block_start\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_1\",\"name\":\"read_file\",\"input\":{}}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_delta\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"path\\\":\\\"\"}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_delta\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"main.go\\\"}\"}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_stop\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"content_block_stop\",\"index\":1}\n\n"))
		_, _ = w.Write([]byte("event: message_delta\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"tool_use\",\"stop_sequence\":null},\"usage\":{\"output_tokens\":12}}\n\n"))
		_, _ = w.Write([]byte("event: message_stop\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"message_stop\"}\n\n"))
	}))
	defer server.Close()

	client := New(config.Config{
		Protocol:  config.ProtocolAnthropic,
		Model:     "m",
		BaseURL:   server.URL,
		APIKey:    "k",
		MaxTokens: 4096,
	})

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

	assertHasEvent(t, events, provider.EventThinkingDelta, func(event provider.StreamEvent) bool {
		return event.Thinking == "hidden"
	})
	assertHasEvent(t, events, provider.EventThinkingComplete, func(event provider.StreamEvent) bool {
		return event.ThinkingSignature == "sig_123"
	})
	assertHasEvent(t, events, provider.EventToolCallStart, func(event provider.StreamEvent) bool {
		return event.ToolCall.ID == "toolu_1" && event.ToolCall.Name == "read_file"
	})
	assertHasEvent(t, events, provider.EventToolCallDelta, func(event provider.StreamEvent) bool {
		return event.ToolCall.ArgumentsDelta == `{"path":"`
	})
	assertHasEvent(t, events, provider.EventToolCallComplete, func(event provider.StreamEvent) bool {
		return event.ToolCall.Arguments == `{"path":"main.go"}`
	})
	assertHasEvent(t, events, provider.EventUsage, func(event provider.StreamEvent) bool {
		return event.Usage != nil && event.Usage.InputTokens == 11
	})
	assertHasEvent(t, events, provider.EventStreamEnd, func(event provider.StreamEvent) bool {
		return event.StopReason == "tool_use" && event.Usage != nil && event.Usage.OutputTokens == 12
	})
	if !strings.Contains(requestBody, `"tools"`) || !strings.Contains(requestBody, `"read_file"`) {
		t.Fatalf("request body does not include tool schema: %s", requestBody)
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

func TestClientReturnsIdleTimeoutWhenStreamStalls(t *testing.T) {
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)

		w.Header().Set("Content-Type", "text/event-stream")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer server.Close()

	client := New(config.Config{
		Protocol: config.ProtocolAnthropic,
		Model:    "m",
		BaseURL:  server.URL,
		APIKey:   "k",
		Stream: &config.StreamConfig{
			IdleTimeout: 20 * time.Millisecond,
		},
	})

	err := client.Stream(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	}, func(provider.StreamEvent) error {
		return nil
	})
	if err == nil {
		t.Fatal("Stream() error = nil, want idle timeout")
	}
	if !strings.Contains(err.Error(), "network idle timeout") {
		t.Fatalf("error = %q, want idle timeout", err)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("server request was not canceled after idle timeout")
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
