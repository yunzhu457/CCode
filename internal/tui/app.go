package tui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/yunzhu457/CCode/internal/chat"
	"github.com/yunzhu457/CCode/internal/llm"
	"github.com/yunzhu457/CCode/internal/provider"
)

type App struct {
	in          io.Reader
	out         io.Writer
	session     *chat.Session
	client      llm.Client
	style       style
	inputEchoes bool
	width       int
	liveOutput  bool
}

func New(in io.Reader, out io.Writer, session *chat.Session, client llm.Client) *App {
	return &App{
		in:          in,
		out:         out,
		session:     session,
		client:      client,
		style:       defaultStyle(),
		inputEchoes: isTerminalFile(in),
		width:       detectBoxWidth(out),
		liveOutput:  isTerminalFile(out),
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
	events, errs := a.client.Stream(ctx, a.session, nil)
	for event := range events {
		switch event.Type {
		case provider.EventTextDelta:
			response.WriteString(event.Text)
			if err := stream.Write(event.Text); err != nil {
				stream.Close()
				return err
			}
		case provider.EventThinkingDelta:
		}
	}
	stream.Close()
	for err := range errs {
		if err != nil {
			return err
		}
	}
	a.session.AddAssistantMessage(response.String())
	return nil
}

const (
	boxWidth        = defaultBoxWidth
	defaultBoxWidth = 88
	minBoxWidth     = 64
	maxBoxWidth     = 118
	resetANSI       = "\x1b[0m"
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
		"C CODE  ·  streaming chat  ·  provider: " + a.client.Name(),
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
		a.renderWrappedLine(line, color)
	}
	a.renderBottom(color)
}

func (a *App) renderTop(title string, color string) {
	label := "─ " + title + " "
	fill := a.width - 2 - cellWidth(label)
	if fill < 1 {
		fill = 1
	}
	fmt.Fprintf(a.out, "%s╭%s%s╮%s\n", color, label, strings.Repeat("─", fill), a.style.reset)
}

func (a *App) renderBottom(color string) {
	fmt.Fprintf(a.out, "%s╰%s╯%s\n", color, strings.Repeat("─", a.width-2), a.style.reset)
}

func (a *App) renderWrappedLine(line string, color string) {
	for _, wrapped := range wrapCells(line, a.innerWidth()) {
		a.renderLine(wrapped, color)
	}
}

func (a *App) renderLine(line string, color string) {
	line = trimToCells(line, a.innerWidth())
	padding := a.innerWidth() - cellWidth(line)
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
		width: a.width,
		inner: a.innerWidth(),
		live:  a.liveOutput,
	}
}

func (a *App) innerWidth() int {
	return a.width - 4
}

type streamBox struct {
	out       io.Writer
	color     string
	reset     string
	width     int
	inner     int
	live      bool
	line      strings.Builder
	lineWidth int
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

		runeWidth := runeCellWidth(r)
		if b.lineWidth > 0 && b.lineWidth+runeWidth > b.inner {
			if err := b.flushLine(); err != nil {
				return err
			}
		}
		b.line.WriteRune(r)
		b.lineWidth += runeWidth
		if b.live {
			if err := b.redrawText(b.line.String()); err != nil {
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
	fmt.Fprintf(b.out, "%s╰%s╯%s\n", b.color, strings.Repeat("─", b.width-2), b.reset)
}

func (b *streamBox) flushLine() error {
	line := b.line.String()
	b.line.Reset()
	b.lineWidth = 0

	var err error
	if b.live {
		err = b.redrawText(line)
		if err == nil {
			_, err = fmt.Fprintln(b.out)
		}
	} else {
		_, err = fmt.Fprintln(b.out, b.formatLine(line))
	}
	b.wroteLine = true
	return err
}

func (b *streamBox) redrawText(line string) error {
	_, err := fmt.Fprintf(b.out, "\r\x1b[2K%s", b.formatLine(line))
	return err
}

func (b *streamBox) formatLine(line string) string {
	line = trimToCells(line, b.inner)
	padding := b.inner - cellWidth(line)
	if padding < 0 {
		padding = 0
	}
	return fmt.Sprintf("%s│ %s%s │%s", b.color, line, strings.Repeat(" ", padding), b.reset)
}

func trimToCells(value string, max int) string {
	if cellWidth(value) <= max {
		return value
	}
	if max <= 0 {
		return ""
	}
	if max == 1 {
		return "…"
	}

	var out strings.Builder
	width := 0
	for _, r := range value {
		runeWidth := runeCellWidth(r)
		if width+runeWidth > max-1 {
			break
		}
		out.WriteRune(r)
		width += runeWidth
	}
	out.WriteRune('…')
	return out.String()
}

func wrapCells(value string, max int) []string {
	if value == "" {
		return []string{""}
	}
	if max <= 0 {
		return []string{value}
	}

	var lines []string
	var current strings.Builder
	width := 0
	for _, r := range value {
		runeWidth := runeCellWidth(r)
		if width > 0 && width+runeWidth > max {
			lines = append(lines, current.String())
			current.Reset()
			width = 0
		}
		current.WriteRune(r)
		width += runeWidth
	}
	lines = append(lines, current.String())
	return lines
}

func cellWidth(value string) int {
	width := 0
	for _, r := range value {
		width += runeCellWidth(r)
	}
	return width
}

func runeCellWidth(r rune) int {
	if r == '\t' {
		return 4
	}
	if r == 0 || r < 32 || (r >= 0x7f && r < 0xa0) {
		return 0
	}
	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) {
		return 0
	}
	if isWideRune(r) {
		return 2
	}
	return 1
}

func isWideRune(r rune) bool {
	return (r >= 0x1100 && r <= 0x115f) ||
		(r >= 0x2e80 && r <= 0xa4cf) ||
		(r >= 0xac00 && r <= 0xd7a3) ||
		(r >= 0xf900 && r <= 0xfaff) ||
		(r >= 0xfe10 && r <= 0xfe6f) ||
		(r >= 0xff00 && r <= 0xff60) ||
		(r >= 0xffe0 && r <= 0xffe6) ||
		(r >= 0x1f300 && r <= 0x1faff)
}

func detectBoxWidth(out io.Writer) int {
	if isTerminalFile(out) {
		if columns := terminalColumns(); columns > 0 {
			return clampBoxWidth(columns - 2)
		}
	}
	return defaultBoxWidth
}

func terminalColumns() int {
	var columns int
	if _, err := fmt.Sscanf(os.Getenv("COLUMNS"), "%d", &columns); err != nil {
		return 0
	}
	return columns
}

func clampBoxWidth(width int) int {
	if width < minBoxWidth {
		return minBoxWidth
	}
	if width > maxBoxWidth {
		return maxBoxWidth
	}
	return width
}

func isTerminalFile(value any) bool {
	file, ok := value.(*os.File)
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
