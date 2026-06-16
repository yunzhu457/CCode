package provider

import "context"

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role    Role
	Content string
}

type ChatRequest struct {
	Messages []Message
}

type EventType string

const (
	EventTextDelta     EventType = "text_delta"
	EventThinkingDelta EventType = "thinking_delta"
	EventDone          EventType = "done"
)

type StreamEvent struct {
	Type     EventType
	Text     string
	Thinking string
}

type EmitFunc func(StreamEvent) error

type Provider interface {
	Name() string
	Stream(ctx context.Context, req ChatRequest, emit EmitFunc) error
}
