package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yunzhu457/CCode/internal/config"
	"github.com/yunzhu457/CCode/internal/provider"
	providerstream "github.com/yunzhu457/CCode/internal/provider/stream"
	"github.com/yunzhu457/CCode/internal/sse"
)

const defaultMaxTokens = 1024

type Client struct {
	cfg        config.Config
	httpClient *http.Client
}

func New(cfg config.Config) *Client {
	return &Client{cfg: cfg, httpClient: &http.Client{}}
}

func (c *Client) Name() string {
	return "anthropic"
}

func (c *Client) Stream(ctx context.Context, req provider.ChatRequest, emit provider.EmitFunc) error {
	payload := anthropicRequest{
		Model:     c.cfg.Model,
		MaxTokens: maxTokens(c.cfg.MaxTokens),
		Messages:  anthropicMessages(req.Messages),
		Tools:     anthropicTools(req.Tools),
		Stream:    true,
	}
	if system := systemPrompt(req.Messages); system != "" {
		payload.System = system
	}
	if c.cfg.Thinking != nil && c.cfg.Thinking.Enabled {
		payload.Thinking = &thinkingRequest{
			Type:         "enabled",
			BudgetTokens: c.cfg.Thinking.BudgetTokens,
			Display:      c.cfg.Thinking.Display,
		}
		if payload.Thinking.BudgetTokens == 0 {
			payload.Thinking.BudgetTokens = defaultMaxTokens
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode anthropic request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicEndpoint(c.cfg.BaseURL), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create anthropic request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("x-api-key", c.cfg.APIKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("send anthropic request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("anthropic request failed: status %d: %s", resp.StatusCode, readLimited(resp.Body))
	}

	reader := sse.NewReader(resp.Body)
	blockTypes := make(map[int]string)
	toolCalls := make(map[int]*toolCallState)
	var stopReason string
	var usage *provider.Usage
	for {
		event, err := providerstream.NextEvent(ctx, reader, idleTimeout(c.cfg), resp.Body.Close)
		if err == io.EOF {
			return emit(provider.StreamEvent{Type: provider.EventStreamEnd, StopReason: stopReason, Usage: usage})
		}
		if err != nil {
			return fmt.Errorf("read anthropic stream: %w", err)
		}
		if event.Data == "" {
			continue
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(event.Data), &chunk); err != nil {
			return fmt.Errorf("decode anthropic stream event: %w", err)
		}
		if chunk.Type == "error" && chunk.Error != nil {
			return fmt.Errorf("anthropic stream error: %s: %s", chunk.Error.Type, chunk.Error.Message)
		}
		switch chunk.Type {
		case "message_start":
			usage = anthropicUsage(chunk.Message.Usage)
			if usage != nil {
				if err := emit(provider.StreamEvent{Type: provider.EventUsage, Usage: usage}); err != nil {
					return err
				}
			}
		case "message_delta":
			if chunk.Delta.StopReason != "" {
				stopReason = chunk.Delta.StopReason
			}
			nextUsage := anthropicUsage(chunk.Usage)
			if nextUsage != nil {
				if usage != nil && nextUsage.InputTokens == 0 {
					nextUsage.InputTokens = usage.InputTokens
				}
				usage = nextUsage
				if err := emit(provider.StreamEvent{Type: provider.EventUsage, Usage: usage}); err != nil {
					return err
				}
			}
		case "message_stop":
			return emit(provider.StreamEvent{Type: provider.EventStreamEnd, StopReason: stopReason, Usage: usage})
		case "content_block_start":
			blockTypes[chunk.Index] = chunk.ContentBlock.Type
			if chunk.ContentBlock.Type == "tool_use" {
				state := &toolCallState{
					event: provider.ToolCallEvent{
						Index: chunk.Index,
						ID:    chunk.ContentBlock.ID,
						Name:  chunk.ContentBlock.Name,
					},
				}
				toolCalls[chunk.Index] = state
				if err := emit(provider.StreamEvent{Type: provider.EventToolCallStart, ToolCall: state.event}); err != nil {
					return err
				}
			}
		case "content_block_delta":
			switch chunk.Delta.Type {
			case "text_delta":
				if chunk.Delta.Text != "" {
					if err := emit(provider.StreamEvent{Type: provider.EventTextDelta, Text: chunk.Delta.Text}); err != nil {
						return err
					}
				}
			case "thinking_delta":
				if chunk.Delta.Thinking != "" {
					if err := emit(provider.StreamEvent{Type: provider.EventThinkingDelta, Thinking: chunk.Delta.Thinking}); err != nil {
						return err
					}
				}
			case "signature_delta":
				if chunk.Delta.Signature != "" {
					if err := emit(provider.StreamEvent{
						Type:              provider.EventThinkingComplete,
						ThinkingSignature: chunk.Delta.Signature,
					}); err != nil {
						return err
					}
				}
			case "input_json_delta":
				if chunk.Delta.PartialJSON != "" {
					state := toolCalls[chunk.Index]
					if state == nil {
						state = &toolCallState{event: provider.ToolCallEvent{Index: chunk.Index}}
						toolCalls[chunk.Index] = state
					}
					state.arguments.WriteString(chunk.Delta.PartialJSON)
					event := state.event
					event.ArgumentsDelta = chunk.Delta.PartialJSON
					if err := emit(provider.StreamEvent{Type: provider.EventToolCallDelta, ToolCall: event}); err != nil {
						return err
					}
				}
			}
		case "content_block_stop":
			if blockTypes[chunk.Index] == "tool_use" {
				state := toolCalls[chunk.Index]
				if state != nil {
					event := state.event
					event.Arguments = state.arguments.String()
					if err := emit(provider.StreamEvent{Type: provider.EventToolCallComplete, ToolCall: event}); err != nil {
						return err
					}
					delete(toolCalls, chunk.Index)
				}
			}
			delete(blockTypes, chunk.Index)
		}
	}
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
	Stream    bool               `json:"stream"`
	Thinking  *thinkingRequest   `json:"thinking,omitempty"`
}

type thinkingRequest struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
	Display      string `json:"display,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type streamChunk struct {
	Type    string `json:"type"`
	Index   int    `json:"index"`
	Message struct {
		Usage *anthropicUsagePayload `json:"usage"`
	} `json:"message"`
	ContentBlock struct {
		Type  string          `json:"type"`
		ID    string          `json:"id"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	} `json:"content_block"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		Thinking    string `json:"thinking"`
		Signature   string `json:"signature"`
		PartialJSON string `json:"partial_json"`
		StopReason  string `json:"stop_reason"`
	} `json:"delta"`
	Usage *anthropicUsagePayload `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

type anthropicUsagePayload struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func anthropicMessages(messages []provider.Message) []anthropicMessage {
	out := make([]anthropicMessage, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == provider.RoleSystem {
			continue
		}
		out = append(out, anthropicMessage{Role: string(msg.Role), Content: msg.Content})
	}
	return out
}

func anthropicTools(tools []provider.ToolSchema) []anthropicTool {
	if len(tools) == 0 {
		return nil
	}

	out := make([]anthropicTool, 0, len(tools))
	for _, tool := range tools {
		out = append(out, anthropicTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: toolInputSchema(tool.InputSchema),
		})
	}
	return out
}

type toolCallState struct {
	event     provider.ToolCallEvent
	arguments strings.Builder
}

func systemPrompt(messages []provider.Message) string {
	var parts []string
	for _, msg := range messages {
		if msg.Role == provider.RoleSystem {
			parts = append(parts, msg.Content)
		}
	}
	return strings.Join(parts, "\n\n")
}

func maxTokens(value int) int {
	if value > 0 {
		return value
	}
	return defaultMaxTokens
}

func idleTimeout(cfg config.Config) time.Duration {
	if cfg.Stream != nil {
		return cfg.Stream.IdleTimeout
	}
	return 0
}

func anthropicEndpoint(baseURL string) string {
	base := strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(base, "/v1/messages") {
		return base
	}
	return base + "/v1/messages"
}

func anthropicUsage(usage *anthropicUsagePayload) *provider.Usage {
	if usage == nil {
		return nil
	}
	return &provider.Usage{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		TotalTokens:  usage.InputTokens + usage.OutputTokens,
	}
}

func toolInputSchema(schema json.RawMessage) json.RawMessage {
	if len(schema) > 0 {
		return schema
	}
	return json.RawMessage(`{"type":"object"}`)
}

func readLimited(r io.Reader) string {
	const max = 4096
	data, err := io.ReadAll(io.LimitReader(r, max))
	if err != nil {
		return "failed to read response body"
	}
	return strings.TrimSpace(string(data))
}
