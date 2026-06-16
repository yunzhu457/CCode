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

func TestAppWrapsLongAssistantTextAndCleansMarkdown(t *testing.T) {
	long := "我是 **DeepSeek**，这是一个很长很长的回答，用来验证输出框不会被撑破，也不会把 markdown 标记和 emoji 原样丢给用户 😊。"
	fake := &fakeProvider{chunks: []string{long}}
	input := strings.NewReader("hello\n/exit\n")
	output := new(strings.Builder)

	app := New(input, output, chat.NewSession(), fake)
	if err := app.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	out := output.String()
	assertContains(t, out, "DeepSeek")
	if strings.Contains(out, "**") {
		t.Fatalf("output leaked markdown emphasis markers: %q", out)
	}
	if strings.Contains(out, "😊") {
		t.Fatalf("output leaked emoji: %q", out)
	}

	lines := strings.Split(stripANSI(out), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "│ ") && strings.Contains(line, "DeepSeek") && !strings.HasSuffix(line, " │") {
			t.Fatalf("assistant line is missing right border: %q", line)
		}
		if strings.HasPrefix(line, "│ ") && cellWidth(line) > boxWidth {
			t.Fatalf("assistant line too wide: %q", line)
		}
	}
}

func TestAppShowsCompactStreamStatusEvents(t *testing.T) {
	fake := &fakeProvider{events: []provider.StreamEvent{
		{Type: provider.EventThinkingDelta, Thinking: "hidden reasoning"},
		{Type: provider.EventThinkingComplete, ThinkingSignature: "sig_123"},
		{Type: provider.EventToolCallStart, ToolCall: provider.ToolCallEvent{Index: 0, ID: "toolu_1", Name: "read_file"}},
		{Type: provider.EventToolCallDelta, ToolCall: provider.ToolCallEvent{Index: 0, ArgumentsDelta: `{"path":"`}},
		{Type: provider.EventToolCallComplete, ToolCall: provider.ToolCallEvent{Index: 0, Arguments: `{"path":"main.go"}`}},
		{Type: provider.EventTextDelta, Text: "Done"},
		{Type: provider.EventUsage, Usage: &provider.Usage{InputTokens: 10, OutputTokens: 4, TotalTokens: 14}},
		{Type: provider.EventStreamEnd, StopReason: "tool_use", Usage: &provider.Usage{InputTokens: 10, OutputTokens: 4, TotalTokens: 14}},
	}}
	input := strings.NewReader("hello\n/exit\n")
	output := new(strings.Builder)

	app := New(input, output, chat.NewSession(), fake)
	if err := app.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	out := output.String()
	assertContains(t, out, "╭─ thinking")
	assertContains(t, out, "completed")
	assertContains(t, out, "╭─ tool · read_file")
	assertContains(t, out, `args: {"path":"main.go"} · completed`)
	assertContains(t, out, "╭─ assistant")
	assertContains(t, out, "Done")
	assertContains(t, out, "╭─ usage")
	assertContains(t, out, "stop: tool_use · input: 10 · output: 4 · total: 14")
	if strings.Contains(out, "hidden reasoning") {
		t.Fatalf("compact UI should not show raw thinking: %q", out)
	}
	if strings.Contains(out, "sig_123") {
		t.Fatalf("compact UI should not show thinking signature: %q", out)
	}
}

func TestInteractiveInputEndDoesNotAddExtraBlankLine(t *testing.T) {
	output := new(strings.Builder)
	app := New(strings.NewReader(""), output, chat.NewSession(), &fakeProvider{})
	app.inputEchoes = true

	app.renderInputPrompt()
	app.renderInputEnd()

	out := output.String()
	if strings.Contains(out, "│ › \x1b[0m\n") {
		t.Fatalf("interactive input end should not add a second newline after terminal echo: %q", out)
	}
}

func TestTrimToCellsAndCellWidth(t *testing.T) {
	if got := cellWidth("你好"); got != 4 {
		t.Fatalf("cellWidth Chinese = %d, want 4", got)
	}
	if got := trimToCells("hello", 10); got != "hello" {
		t.Fatalf("trimToCells short = %q", got)
	}
	if got := trimToCells("你好abc", 5); got != "你好…" {
		t.Fatalf("trimToCells wide = %q", got)
	}
}

func TestLiveStreamBoxRedrawsCurrentLine(t *testing.T) {
	output := new(strings.Builder)
	stream := &streamBox{
		out:   output,
		color: "",
		reset: "",
		width: 20,
		inner: 16,
		live:  true,
	}

	if err := stream.Write("hello"); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := stream.Write(" world"); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	stream.Close()

	out := output.String()
	assertContains(t, out, "\r\x1b[2K")
	assertContains(t, out, "│ hello world")
	assertContains(t, out, "╰──────────────────╯")
}

type fakeProvider struct {
	requests []provider.ChatRequest
	err      error
	chunks   []string
	events   []provider.StreamEvent
}

func (f *fakeProvider) Name() string {
	return "fake"
}

func (f *fakeProvider) Stream(_ context.Context, session *chat.Session, tools []provider.ToolSchema) (<-chan provider.StreamEvent, <-chan error) {
	events := make(chan provider.StreamEvent, len(f.chunks)+len(f.events)+4)
	errs := make(chan error, 1)

	f.stream(session, tools, events, errs)
	close(events)
	close(errs)

	return events, errs
}

func (f *fakeProvider) stream(session *chat.Session, tools []provider.ToolSchema, events chan<- provider.StreamEvent, errs chan<- error) {
	req := provider.ChatRequest{Messages: session.Messages(), Tools: tools}
	f.requests = append(f.requests, req)
	if f.err != nil {
		errs <- f.err
		return
	}
	if len(f.events) > 0 {
		for _, event := range f.events {
			events <- event
		}
		return
	}
	chunks := f.chunks
	if len(chunks) == 0 {
		chunks = []string{"hi", " there"}
	}
	for _, chunk := range chunks {
		events <- provider.StreamEvent{Type: provider.EventTextDelta, Text: chunk}
	}
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

func stripANSI(value string) string {
	var out strings.Builder
	inEscape := false
	for _, r := range value {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}
