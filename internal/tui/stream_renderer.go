package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yunzhu457/CCode/internal/provider"
)

type streamRenderer struct {
	app        *App
	assistant  *streamBox
	response   strings.Builder
	thinking   *statusBox
	tools      map[int]*toolRenderState
	usage      *provider.Usage
	stopReason string
	closed     bool
}

func newStreamRenderer(app *App) *streamRenderer {
	return &streamRenderer{
		app:   app,
		tools: make(map[int]*toolRenderState),
	}
}

func (r *streamRenderer) Handle(event provider.StreamEvent) error {
	switch event.Type {
	case provider.EventTextDelta:
		return r.writeAssistant(event.Text)
	case provider.EventThinkingDelta:
		return r.showThinking("thinking...")
	case provider.EventThinkingComplete:
		return r.completeThinking()
	case provider.EventToolCallStart:
		return r.startTool(event.ToolCall)
	case provider.EventToolCallDelta:
		return r.updateTool(event.ToolCall)
	case provider.EventToolCallComplete:
		return r.completeTool(event.ToolCall)
	case provider.EventUsage:
		r.usage = event.Usage
	case provider.EventStreamEnd:
		r.stopReason = event.StopReason
		if event.Usage != nil {
			r.usage = event.Usage
		}
		return r.closeStatuses()
	}
	return nil
}

func (r *streamRenderer) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true

	if err := r.closeStatuses(); err != nil {
		return err
	}
	if r.assistant != nil {
		r.assistant.Close()
	}
	if r.shouldRenderUsage() {
		r.app.renderBox("usage", []string{r.usageLine()}, r.app.style.muted)
	}
	return nil
}

func (r *streamRenderer) Response() string {
	return r.response.String()
}

func (r *streamRenderer) writeAssistant(text string) error {
	if err := r.closeStatuses(); err != nil {
		return err
	}
	if r.assistant == nil {
		r.assistant = r.app.beginStreamBox("assistant")
	}
	r.response.WriteString(text)
	return r.assistant.Write(text)
}

func (r *streamRenderer) showThinking(status string) error {
	if r.assistant != nil {
		return nil
	}
	if r.thinking == nil {
		r.thinking = newStatusBox(r.app, "thinking", r.app.style.muted)
	}
	return r.thinking.Update(status)
}

func (r *streamRenderer) completeThinking() error {
	if r.assistant != nil {
		return nil
	}
	if r.thinking == nil {
		r.thinking = newStatusBox(r.app, "thinking", r.app.style.muted)
	}
	if err := r.thinking.Update("completed"); err != nil {
		return err
	}
	if err := r.thinking.Close(); err != nil {
		return err
	}
	r.thinking = nil
	return nil
}

func (r *streamRenderer) startTool(call provider.ToolCallEvent) error {
	if err := r.closeThinking(); err != nil {
		return err
	}
	state := r.toolState(call)
	state.merge(call)
	return state.box.Update(toolArgsLine(state.arguments, false))
}

func (r *streamRenderer) updateTool(call provider.ToolCallEvent) error {
	if err := r.closeThinking(); err != nil {
		return err
	}
	state := r.toolState(call)
	state.merge(call)
	if call.ArgumentsDelta != "" {
		state.arguments += call.ArgumentsDelta
	}
	return state.box.Update(toolArgsLine(state.arguments, false))
}

func (r *streamRenderer) completeTool(call provider.ToolCallEvent) error {
	if err := r.closeThinking(); err != nil {
		return err
	}
	state := r.toolState(call)
	state.merge(call)
	if call.Arguments != "" {
		state.arguments = call.Arguments
	}
	if err := state.box.Update(toolArgsLine(state.arguments, true)); err != nil {
		return err
	}
	if err := state.box.Close(); err != nil {
		return err
	}
	delete(r.tools, call.Index)
	return nil
}

func (r *streamRenderer) toolState(call provider.ToolCallEvent) *toolRenderState {
	state := r.tools[call.Index]
	if state != nil {
		return state
	}

	title := "tool"
	if call.Name != "" {
		title += " · " + call.Name
	}
	state = &toolRenderState{box: newStatusBox(r.app, title, r.app.style.muted)}
	r.tools[call.Index] = state
	return state
}

func (r *streamRenderer) closeStatuses() error {
	if err := r.closeThinking(); err != nil {
		return err
	}
	return r.closeTools()
}

func (r *streamRenderer) closeThinking() error {
	if r.thinking == nil {
		return nil
	}
	if err := r.thinking.Close(); err != nil {
		return err
	}
	r.thinking = nil
	return nil
}

func (r *streamRenderer) closeTools() error {
	if len(r.tools) == 0 {
		return nil
	}

	indexes := make([]int, 0, len(r.tools))
	for index := range r.tools {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)

	for _, index := range indexes {
		state := r.tools[index]
		if err := state.box.Close(); err != nil {
			return err
		}
		delete(r.tools, index)
	}
	return nil
}

func (r *streamRenderer) shouldRenderUsage() bool {
	return r.stopReason != "" || r.usage != nil
}

func (r *streamRenderer) usageLine() string {
	stop := r.stopReason
	if stop == "" {
		stop = "unknown"
	}
	if r.usage == nil {
		return "stop: " + stop
	}

	total := r.usage.TotalTokens
	if total == 0 {
		total = r.usage.InputTokens + r.usage.OutputTokens
	}
	return fmt.Sprintf("stop: %s · input: %d · output: %d · total: %d", stop, r.usage.InputTokens, r.usage.OutputTokens, total)
}

type toolRenderState struct {
	box       *statusBox
	id        string
	name      string
	arguments string
}

func (s *toolRenderState) merge(call provider.ToolCallEvent) {
	if call.ID != "" {
		s.id = call.ID
	}
	if call.Name != "" {
		s.name = call.Name
	}
}

func toolArgsLine(arguments string, completed bool) string {
	line := "args: " + arguments
	if completed {
		line += " · completed"
	}
	return line
}

type statusBox struct {
	app      *App
	title    string
	color    string
	line     string
	live     bool
	active   bool
	rendered bool
}

func newStatusBox(app *App, title string, color string) *statusBox {
	return &statusBox{
		app:   app,
		title: title,
		color: color,
		live:  app.liveOutput,
	}
}

func (b *statusBox) Update(line string) error {
	b.line = line
	if !b.live {
		return nil
	}
	if !b.active {
		b.app.renderTop(b.title, b.color)
		b.active = true
	}
	return b.redrawLine()
}

func (b *statusBox) Close() error {
	if b.rendered {
		return nil
	}
	if b.line == "" {
		b.line = "completed"
	}

	if b.live {
		if !b.active {
			b.app.renderTop(b.title, b.color)
			b.active = true
		}
		if err := b.redrawLine(); err != nil {
			return err
		}
		fmt.Fprintln(b.app.out)
		b.app.renderBottom(b.color)
	} else {
		b.app.renderBox(b.title, []string{b.line}, b.color)
	}
	b.rendered = true
	return nil
}

func (b *statusBox) redrawLine() error {
	_, err := fmt.Fprintf(b.app.out, "\r\x1b[2K%s", b.formatLine())
	return err
}

func (b *statusBox) formatLine() string {
	line := trimToCells(b.line, b.app.innerWidth())
	padding := b.app.innerWidth() - cellWidth(line)
	if padding < 0 {
		padding = 0
	}
	return fmt.Sprintf("%s│ %s%s │%s", b.color, line, strings.Repeat(" ", padding), b.app.style.reset)
}
