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
	assertContains(t, out, "\x1b[")
	assertContains(t, out, "C CODE")
	assertContains(t, out, "streaming chat")
	assertContains(t, out, "╭─ input")
	assertContains(t, out, "│ › ")
	assertContains(t, out, "╭─ assistant")
	assertContains(t, out, "hi there")
	assertContains(t, out, "╭─ session")
	assertContains(t, out, "bye")
	if strings.Contains(out, "You> ") {
		t.Fatalf("output still uses plain prompt: %q", out)
	}
	if strings.Contains(out, "assistant>") {
		t.Fatalf("output still uses plain assistant prefix: %q", out)
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
	assertContains(t, out, "╭─ error")
	assertContains(t, out, "provider failed")
	assertContains(t, out, "bye")
}

func TestAppWrapsMultilineAssistantOutputInBox(t *testing.T) {
	fake := &fakeProvider{chunks: []string{"first line\nsecond ", "line"}}
	input := strings.NewReader("hello\n/exit\n")
	output := new(strings.Builder)

	app := New(input, output, chat.NewSession(), fake)
	if err := app.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	out := output.String()
	if got := strings.Count(out, "│ "); got < 4 {
		t.Fatalf("output should render multiple bordered lines, got %d in %q", got, out)
	}
	assertContains(t, out, "first line")
	assertContains(t, out, "second line")
}

func TestTrimToRunes(t *testing.T) {
	if got := trimToRunes("hello", 10); got != "hello" {
		t.Fatalf("trimToRunes short = %q", got)
	}
	if got := trimToRunes("abcdef", 4); got != "abc…" {
		t.Fatalf("trimToRunes long = %q", got)
	}
}

type fakeProvider struct {
	requests []provider.ChatRequest
	err      error
	chunks   []string
}

func (f *fakeProvider) Name() string {
	return "fake"
}

func (f *fakeProvider) Stream(_ context.Context, req provider.ChatRequest, emit provider.EmitFunc) error {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return f.err
	}
	chunks := f.chunks
	if len(chunks) == 0 {
		chunks = []string{"hi", " there"}
	}
	for _, chunk := range chunks {
		if err := emit(provider.StreamEvent{Type: provider.EventTextDelta, Text: chunk}); err != nil {
			return err
		}
	}
	return nil
}

var errProviderFailure = providerFailureError{}

type providerFailureError struct{}

func (providerFailureError) Error() string {
	return "provider failed"
}

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("output missing %q in %q", want, got)
	}
}
