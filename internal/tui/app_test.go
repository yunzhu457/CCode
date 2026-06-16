package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/yunzhu457/CCode/internal/chat"
	"github.com/yunzhu457/CCode/internal/provider"
)

func TestAppRunsConversationAndExit(t *testing.T) {
	fake := &fakeProvider{}
	input := strings.NewReader("hello\n/exit\n")
	output := new(strings.Builder)

	app := New(input, output, chat.NewSession(), fake)
	if err := app.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	out := output.String()
	if !strings.Contains(out, "You> ") {
		t.Fatalf("output missing prompt: %q", out)
	}
	if !strings.Contains(out, "assistant> hi there") {
		t.Fatalf("output missing assistant response: %q", out)
	}
	if !strings.Contains(out, "bye") {
		t.Fatalf("output missing exit message: %q", out)
	}
	if len(fake.requests) != 1 {
		t.Fatalf("provider requests = %d, want 1", len(fake.requests))
	}
	if fake.requests[0].Messages[0].Content != "hello" {
		t.Fatalf("request message = %+v", fake.requests[0].Messages[0])
	}
}

func TestAppContinuesAfterProviderError(t *testing.T) {
	fake := &fakeProvider{err: errProviderFailure}
	input := strings.NewReader("hello\n/exit\n")
	output := new(strings.Builder)

	app := New(input, output, chat.NewSession(), fake)
	if err := app.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	out := output.String()
	if !strings.Contains(out, "error: provider failed") {
		t.Fatalf("output missing provider error: %q", out)
	}
	if !strings.Contains(out, "bye") {
		t.Fatalf("output missing exit message: %q", out)
	}
}

type fakeProvider struct {
	requests []provider.ChatRequest
	err      error
}

func (f *fakeProvider) Name() string {
	return "fake"
}

func (f *fakeProvider) Stream(_ context.Context, req provider.ChatRequest, emit provider.EmitFunc) error {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return f.err
	}
	if err := emit(provider.StreamEvent{Type: provider.EventTextDelta, Text: "hi"}); err != nil {
		return err
	}
	return emit(provider.StreamEvent{Type: provider.EventTextDelta, Text: " there"})
}

var errProviderFailure = providerFailureError{}

type providerFailureError struct{}

func (providerFailureError) Error() string {
	return "provider failed"
}
