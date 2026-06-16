package llm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/yunzhu457/CCode/internal/chat"
	"github.com/yunzhu457/CCode/internal/config"
	"github.com/yunzhu457/CCode/internal/provider"
)

func TestClientStreamsSessionAndToolsAsChannels(t *testing.T) {
	fake := &fakeProvider{
		events: []provider.StreamEvent{
			{Type: provider.EventTextDelta, Text: "hello"},
			{Type: provider.EventStreamEnd, StopReason: "end_turn"},
		},
	}
	session := chat.NewSession()
	session.AddUserMessage("hi")
	tools := []provider.ToolSchema{{
		Name:        "read_file",
		Description: "Read a file",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	}}

	events, errs := New(fake).Stream(context.Background(), session, tools)

	var got []provider.StreamEvent
	for event := range events {
		got = append(got, event)
	}
	if err := readErr(errs); err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	if len(got) != 2 || got[0].Text != "hello" || got[1].StopReason != "end_turn" {
		t.Fatalf("events = %+v", got)
	}
	if len(fake.last.Messages) != 1 || fake.last.Messages[0].Content != "hi" {
		t.Fatalf("messages = %+v", fake.last.Messages)
	}
	if len(fake.last.Tools) != 1 || fake.last.Tools[0].Name != "read_file" {
		t.Fatalf("tools = %+v", fake.last.Tools)
	}
}

func TestClientStreamsErrorsOnErrorChannel(t *testing.T) {
	wantErr := errors.New("provider failed")
	fake := &fakeProvider{err: wantErr}

	events, errs := New(fake).Stream(context.Background(), chat.NewSession(), nil)
	for range events {
		t.Fatal("unexpected event")
	}
	if err := readErr(errs); !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

func TestNewFromConfigSelectsProvider(t *testing.T) {
	client, err := NewFromConfig(config.Config{
		Protocol: config.ProtocolAnthropic,
		Model:    "model",
		BaseURL:  "https://example.test",
		APIKey:   "key",
	})
	if err != nil {
		t.Fatalf("NewFromConfig() error = %v", err)
	}
	if client.Name() != "anthropic" {
		t.Fatalf("Name() = %q", client.Name())
	}
}

func TestNewFromConfigRejectsUnknownProtocol(t *testing.T) {
	_, err := NewFromConfig(config.Config{Protocol: "unknown", Model: "m", BaseURL: "https://example.test", APIKey: "k"})
	if err == nil {
		t.Fatal("NewFromConfig() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "unsupported protocol") {
		t.Fatalf("error = %q", err)
	}
}

type fakeProvider struct {
	last   provider.ChatRequest
	events []provider.StreamEvent
	err    error
}

func (f *fakeProvider) Name() string {
	return "fake"
}

func (f *fakeProvider) Stream(_ context.Context, req provider.ChatRequest, emit provider.EmitFunc) error {
	f.last = req
	if f.err != nil {
		return f.err
	}
	for _, event := range f.events {
		if err := emit(event); err != nil {
			return err
		}
	}
	return nil
}

func readErr(errs <-chan error) error {
	for err := range errs {
		return err
	}
	return nil
}
