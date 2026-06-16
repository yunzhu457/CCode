package tui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/yunzhu457/CCode/internal/chat"
	"github.com/yunzhu457/CCode/internal/provider"
)

type App struct {
	in          io.Reader
	out         io.Writer
	session     *chat.Session
	provider    provider.Provider
	style       style
	inputEchoes bool
}

func New(in io.Reader, out io.Writer, session *chat.Session, p provider.Provider) *App {
	return &App{
		in:          in,
		out:         out,
		session:     session,
		provider:    p,
		style:       defaultStyle(),
		inputEchoes: detectsTerminalEcho(in),
	}
}

func (a *App) Run(ctx context.Context) error {
	reader := bufio.NewReader(a.in)
	a.renderWelcome()

	for {
		a.renderInputPrompt()
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("read input: %w", err)
		}
		text := strings.TrimSpace(line)
		a.renderInputEnd()
		if text == "/exit" || text == "/quit" {
			a.renderMessageBox("session", []string{"bye"})
			return nil
		}
		if text != "" {
			if err := a.send(ctx, text); err != nil {
				a.renderMessageBox("error", []string{err.Error()})
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
	}
}

func (a *App) send(ctx context.Context, text string) error {
	a.session.AddUserMessage(text)
	var response strings.Builder

	stream := a.beginStreamBox("assistant")
	err := a.provider.Stream(ctx, provider.ChatRequest{Messages: a.session.Messages()}, func(event provider.StreamEvent) error {
		switch event.Type {
		case provider.EventTextDelta:
			response.WriteString(event.Text)
			return stream.Write(event.Text)
		case provider.EventThinkingDelta:
			return nil
		default:
			return nil
		}
	})
	stream.Close()
	if err != nil {
		return err
	}
	a.session.AddAssistantMessage(response.String())
	return nil
}

const (
	boxWidth  = 74
	boxInner  = boxWidth - 4
	resetANSI = "\x1b[0m"
)

type style struct {
	accent string
	muted  string
	user   string
	error  string
	reset  string
}

func defaultStyle() style {
	return style{
		accent: "\x1b[38;5;111m",
		muted:  "\x1b[38;5;244m",
		user:   "\x1b[38;5;156m",
		error:  "\x1b[38;5;203m",
		reset:  resetANSI,
	}
}

func (a *App) renderWelcome() {
	lines := []string{
		"  ####    ####    ####   #####   #####",
		" #       #       #    #  #    #  #    ",
		" #       #       #    #  #    #  #### ",
		" #       #       #    #  #    #  #    ",
		"  ####    ####    ####   #####   #####",
		"",
		"C CODE  ·  streaming chat  ·  provider: " + a.provider.Name(),
		"Type /exit or /quit to leave the session.",
	}
	a.renderBox("c code terminal", lines, a.style.accent)
}

func (a *App) renderInputPrompt() {
	a.renderTop("input", a.style.user)
	fmt.Fprintf(a.out, "%s│ › %s", a.style.user, a.style.reset)
}

func (a *App) renderInputEnd() {
	if !a.inputEchoes {
		fmt.Fprintln(a.out)
	}
	a.renderBottom(a.style.user)
}

func (a *App) renderMessageBox(title string, lines []string) {
	color := a.style.accent
	if title == "error" {
		color = a.style.error
	}
	a.renderBox(title, lines, color)
}

func (a *App) renderBox(title string, lines []string, color string) {
	a.renderTop(title, color)
	for _, line := range lines {
		a.renderLine(line, color)
	}
	a.renderBottom(color)
}

func (a *App) renderTop(title string, color string) {
	label := "─ " + title + " "
	fill := boxWidth - 2 - utf8.RuneCountInString(label)
	if fill < 1 {
		fill = 1
	}
	fmt.Fprintf(a.out, "%s╭%s%s╮%s\n", color, label, strings.Repeat("─", fill), a.style.reset)
}

func (a *App) renderBottom(color string) {
	fmt.Fprintf(a.out, "%s╰%s╯%s\n", color, strings.Repeat("─", boxWidth-2), a.style.reset)
}

func (a *App) renderLine(line string, color string) {
	line = trimToRunes(line, boxInner)
	padding := boxInner - utf8.RuneCountInString(line)
	if padding < 0 {
		padding = 0
	}
	fmt.Fprintf(a.out, "%s│ %s%s │%s\n", color, line, strings.Repeat(" ", padding), a.style.reset)
}

func (a *App) beginStreamBox(title string) *streamBox {
	a.renderTop(title, a.style.accent)
	return &streamBox{
		out:   a.out,
		color: a.style.accent,
		reset: a.style.reset,
	}
}

type streamBox struct {
	out       io.Writer
	color     string
	reset     string
	line      strings.Builder
	wroteLine bool
}

func (b *streamBox) Write(text string) error {
	for _, r := range cleanStreamText(text) {
		switch r {
		case '\r':
			continue
		case '\n':
			if err := b.flushLine(); err != nil {
				return err
			}
			continue
		}

		b.line.WriteRune(r)
		if utf8.RuneCountInString(b.line.String()) >= boxInner {
			if err := b.flushLine(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *streamBox) Close() {
	if b.line.Len() > 0 || !b.wroteLine {
		_ = b.flushLine()
	}
	fmt.Fprintf(b.out, "%s╰%s╯%s\n", b.color, strings.Repeat("─", boxWidth-2), b.reset)
}

func (b *streamBox) flushLine() error {
	line := b.line.String()
	b.line.Reset()
	padding := boxInner - utf8.RuneCountInString(line)
	if padding < 0 {
		padding = 0
	}
	_, err := fmt.Fprintf(b.out, "%s│ %s%s │%s\n", b.color, line, strings.Repeat(" ", padding), b.reset)
	b.wroteLine = true
	return err
}

func trimToRunes(value string, max int) string {
	if utf8.RuneCountInString(value) <= max {
		return value
	}
	var out strings.Builder
	count := 0
	for _, r := range value {
		if count >= max-1 {
			break
		}
		out.WriteRune(r)
		count++
	}
	out.WriteRune('…')
	return out.String()
}

func detectsTerminalEcho(in io.Reader) bool {
	file, ok := in.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func cleanStreamText(value string) string {
	value = strings.ReplaceAll(value, "**", "")
	value = strings.ReplaceAll(value, "__", "")
	value = strings.ReplaceAll(value, "`", "")

	var out strings.Builder
	for _, r := range value {
		if isEmojiRune(r) {
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

func isEmojiRune(r rune) bool {
	return (r >= 0x1F000 && r <= 0x1FAFF) || r == 0xFE0F
}
