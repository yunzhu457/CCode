package provider

import (
	"context"
	"encoding/json"
)

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

type ToolSchema struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

type ChatRequest struct {
	Messages []Message
	Tools    []ToolSchema
}

type EventType string

const (
	EventTextDelta        EventType = "text_delta"
	EventThinkingDelta    EventType = "thinking_delta"
	EventThinkingComplete EventType = "thinking_complete"
	EventToolCallStart    EventType = "tool_call_start"
	EventToolCallDelta    EventType = "tool_call_delta"
	EventToolCallComplete EventType = "tool_call_complete"
	EventStreamEnd        EventType = "stream_end"
	EventUsage            EventType = "usage"
	EventDone             EventType = EventStreamEnd
)

type StreamEvent struct {
	Type              EventType
	Text              string
	Thinking          string
	ThinkingSignature string
	ToolCall          ToolCallEvent
	StopReason        string
	Usage             *Usage
}

type ToolCallEvent struct {
	Index          int
	ID             string
	Name           string
	ArgumentsDelta string
	Arguments      string
}

type Usage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

type EmitFunc func(StreamEvent) error

type Provider interface {
	Name() string
	Stream(ctx context.Context, req ChatRequest, emit EmitFunc) error
}
