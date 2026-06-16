package tui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/yunzhu457/CCode/internal/chat"
	"github.com/yunzhu457/CCode/internal/provider"
)

type App struct {
	in       io.Reader
	out      io.Writer
	session  *chat.Session
	provider provider.Provider
}

func New(in io.Reader, out io.Writer, session *chat.Session, p provider.Provider) *App {
	return &App{
		in:       in,
		out:      out,
		session:  session,
		provider: p,
	}
}

func (a *App) Run(ctx context.Context) error {
	reader := bufio.NewReader(a.in)
	fmt.Fprintf(a.out, "C Code (%s). Type /exit to quit.\n", a.provider.Name())

	for {
		fmt.Fprint(a.out, "You> ")
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("read input: %w", err)
		}
		text := strings.TrimSpace(line)
		if text == "/exit" || text == "/quit" {
			fmt.Fprintln(a.out, "bye")
			return nil
		}
		if text != "" {
			if err := a.send(ctx, text); err != nil {
				fmt.Fprintf(a.out, "error: %v\n", err)
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

	fmt.Fprint(a.out, "assistant> ")
	err := a.provider.Stream(ctx, provider.ChatRequest{Messages: a.session.Messages()}, func(event provider.StreamEvent) error {
		switch event.Type {
		case provider.EventTextDelta:
			response.WriteString(event.Text)
			_, err := fmt.Fprint(a.out, event.Text)
			return err
		case provider.EventThinkingDelta:
			return nil
		default:
			return nil
		}
	})
	fmt.Fprintln(a.out)
	if err != nil {
		return err
	}
	a.session.AddAssistantMessage(response.String())
	return nil
}
