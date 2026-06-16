package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/yunzhu457/CCode/internal/chat"
	"github.com/yunzhu457/CCode/internal/provider"
)

func TestStreamRendererClosesPendingStatusesBeforeAssistant(t *testing.T) {
	output := new(strings.Builder)
	app := New(strings.NewReader(""), output, chat.NewSession(), &fakeProvider{})
	renderer := newStreamRenderer(app)

	events := []provider.StreamEvent{
		{Type: provider.EventThinkingDelta, Thinking: "hidden"},
		{Type: provider.EventToolCallStart, ToolCall: provider.ToolCallEvent{Index: 2, Name: "read_file"}},
		{Type: provider.EventToolCallDelta, ToolCall: provider.ToolCallEvent{Index: 2, ArgumentsDelta: `{"path":"main.go"}`}},
		{Type: provider.EventTextDelta, Text: "answer"},
		{Type: provider.EventStreamEnd, StopReason: "end_turn"},
	}
	for _, event := range events {
		if err := renderer.Handle(event); err != nil {
			t.Fatalf("Handle(%s) error = %v", event.Type, err)
		}
	}
	if err := renderer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	out := output.String()
	assertContains(t, out, "╭─ thinking")
	assertContains(t, out, "thinking...")
	assertContains(t, out, "╭─ tool · read_file")
	assertContains(t, out, `args: {"path":"main.go"}`)
	assertContains(t, out, "╭─ assistant")
	assertContains(t, out, "answer")
	assertContains(t, out, "╭─ usage")
	assertContains(t, out, "stop: end_turn")
	if renderer.Response() != "answer" {
		t.Fatalf("Response() = %q", renderer.Response())
	}
}

func TestStatusBoxLiveUpdatesCurrentLine(t *testing.T) {
	output := new(strings.Builder)
	app := New(strings.NewReader(""), output, chat.NewSession(), &fakeProvider{})
	app.liveOutput = true
	app.width = 40

	box := newStatusBox(app, "thinking", "")
	if err := box.Update("thinking..."); err != nil {
		t.Fatalf("Update(thinking) error = %v", err)
	}
	if err := box.Update("completed"); err != nil {
		t.Fatalf("Update(completed) error = %v", err)
	}
	if err := box.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := box.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}

	out := output.String()
	assertContains(t, out, "\r\x1b[2K")
	assertContains(t, out, "completed")
	assertContains(t, out, "╰──────────────────────────────────────╯")
}

func TestTUIWidthHelpers(t *testing.T) {
	t.Setenv("COLUMNS", "100")
	if got := terminalColumns(); got != 100 {
		t.Fatalf("terminalColumns() = %d, want 100", got)
	}
	if got := clampBoxWidth(1); got != minBoxWidth {
		t.Fatalf("clampBoxWidth(low) = %d", got)
	}
	if got := clampBoxWidth(500); got != maxBoxWidth {
		t.Fatalf("clampBoxWidth(high) = %d", got)
	}
	if got := clampBoxWidth(90); got != 90 {
		t.Fatalf("clampBoxWidth(mid) = %d", got)
	}
	if got := runeCellWidth('\t'); got != 4 {
		t.Fatalf("tab width = %d", got)
	}
	if got := runeCellWidth('\x00'); got != 0 {
		t.Fatalf("nul width = %d", got)
	}
	if got := runeCellWidth('\u0301'); got != 0 {
		t.Fatalf("combining mark width = %d", got)
	}
	if got := runeCellWidth('界'); got != 2 {
		t.Fatalf("wide rune width = %d", got)
	}
	if got := wrapCells("", 10); len(got) != 1 || got[0] != "" {
		t.Fatalf("wrapCells(empty) = %#v", got)
	}
	if got := wrapCells("hello", 0); len(got) != 1 || got[0] != "hello" {
		t.Fatalf("wrapCells(no max) = %#v", got)
	}
}

func TestFakeProviderSatisfiesClientContract(t *testing.T) {
	fake := &fakeProvider{events: []provider.StreamEvent{{Type: provider.EventStreamEnd}}}
	events, errs := fake.Stream(context.Background(), chat.NewSession(), nil)
	for range events {
	}
	for err := range errs {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}
