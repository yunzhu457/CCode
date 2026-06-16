package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/yunzhu457/CCode/internal/config"
	"github.com/yunzhu457/CCode/internal/provider"
	providershared "github.com/yunzhu457/CCode/internal/provider/shared"
	providerstream "github.com/yunzhu457/CCode/internal/provider/stream"
	"github.com/yunzhu457/CCode/internal/sse"
)

type Client struct {
	cfg        config.Config
	httpClient *http.Client
}

func New(cfg config.Config) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: providershared.NewHTTPClient(),
	}
}

func (c *Client) Name() string {
	return "openai"
}

func (c *Client) Stream(ctx context.Context, req provider.ChatRequest, emit provider.EmitFunc) error {
	body, err := json.Marshal(chatCompletionsRequest{
		Model:         c.cfg.Model,
		Messages:      openAIMessages(req.Messages),
		Tools:         openAITools(req.Tools),
		Stream:        true,
		StreamOptions: streamOptions{IncludeUsage: true},
	})
	if err != nil {
		return fmt.Errorf("encode openai request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIEndpoint(c.cfg.BaseURL), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create openai request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("send openai request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return providershared.HTTPStatusError("openai", resp)
	}

	reader := sse.NewReader(resp.Body)
	toolCalls := make(map[int]*toolCallState)
	var stopReason string
	var usage *provider.Usage
	for {
		event, err := providerstream.NextEvent(ctx, reader, providershared.IdleTimeout(c.cfg), resp.Body.Close)
		if err == io.EOF {
			if err := completeOpenAIToolCalls(toolCalls, emit); err != nil {
				return err
			}
			return emit(provider.StreamEvent{Type: provider.EventStreamEnd, StopReason: stopReason, Usage: usage})
		}
		if err != nil {
			return fmt.Errorf("read openai stream: %w", err)
		}
		if strings.TrimSpace(event.Data) == "[DONE]" {
			if err := completeOpenAIToolCalls(toolCalls, emit); err != nil {
				return err
			}
			return emit(provider.StreamEvent{Type: provider.EventStreamEnd, StopReason: stopReason, Usage: usage})
		}
		if event.Data == "" {
			continue
		}

		var chunk chatCompletionChunk
		if err := json.Unmarshal([]byte(event.Data), &chunk); err != nil {
			return fmt.Errorf("decode openai stream event: %w", err)
		}
		if chunk.Usage != nil {
			usage = openAIUsage(chunk.Usage)
			if err := emit(provider.StreamEvent{Type: provider.EventUsage, Usage: usage}); err != nil {
				return err
			}
		}
		for _, choice := range chunk.Choices {
			if choice.FinishReason != "" {
				stopReason = choice.FinishReason
			}
			if choice.Delta.ReasoningContent != "" {
				if err := emit(provider.StreamEvent{Type: provider.EventThinkingDelta, Thinking: choice.Delta.ReasoningContent}); err != nil {
					return err
				}
			}
			if choice.Delta.Content != "" {
				if err := emit(provider.StreamEvent{Type: provider.EventTextDelta, Text: choice.Delta.Content}); err != nil {
					return err
				}
			}
			for _, toolCall := range choice.Delta.ToolCalls {
				if err := emitOpenAIToolCall(toolCalls, toolCall, emit); err != nil {
					return err
				}
			}
			if choice.FinishReason != "" {
				if err := completeOpenAIToolCalls(toolCalls, emit); err != nil {
					return err
				}
			}
		}
	}
}

type chatCompletionsRequest struct {
	Model         string          `json:"model"`
	Messages      []openAIMessage `json:"messages"`
	Tools         []openAITool    `json:"tools,omitempty"`
	Stream        bool            `json:"stream"`
	StreamOptions streamOptions   `json:"stream_options"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionChunk struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
			ToolCalls        []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *openAIUsagePayload `json:"usage"`
}

type openAIUsagePayload struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openAITool struct {
	Type     string         `json:"type"`
	Function openAIFunction `json:"function"`
}

type openAIFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

func openAIMessages(messages []provider.Message) []openAIMessage {
	out := make([]openAIMessage, 0, len(messages))
	for _, msg := range messages {
		out = append(out, openAIMessage{Role: string(msg.Role), Content: msg.Content})
	}
	return out
}

func openAITools(tools []provider.ToolSchema) []openAITool {
	if len(tools) == 0 {
		return nil
	}

	out := make([]openAITool, 0, len(tools))
	for _, tool := range tools {
		out = append(out, openAITool{
			Type: "function",
			Function: openAIFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  providershared.ToolInputSchema(tool.InputSchema),
			},
		})
	}
	return out
}

type toolCallState struct {
	event     provider.ToolCallEvent
	arguments strings.Builder
	started   bool
}

func emitOpenAIToolCall(states map[int]*toolCallState, delta struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}, emit provider.EmitFunc) error {
	state := states[delta.Index]
	if state == nil {
		state = &toolCallState{event: provider.ToolCallEvent{Index: delta.Index}}
		states[delta.Index] = state
	}
	if delta.ID != "" {
		state.event.ID = delta.ID
	}
	if delta.Function.Name != "" {
		state.event.Name = delta.Function.Name
	}
	if !state.started && (state.event.ID != "" || state.event.Name != "") {
		state.started = true
		if err := emit(provider.StreamEvent{Type: provider.EventToolCallStart, ToolCall: state.event}); err != nil {
			return err
		}
	}
	if delta.Function.Arguments != "" {
		state.arguments.WriteString(delta.Function.Arguments)
		event := state.event
		event.ArgumentsDelta = delta.Function.Arguments
		if err := emit(provider.StreamEvent{Type: provider.EventToolCallDelta, ToolCall: event}); err != nil {
			return err
		}
	}
	return nil
}

func completeOpenAIToolCalls(states map[int]*toolCallState, emit provider.EmitFunc) error {
	for index, state := range states {
		event := state.event
		event.Arguments = state.arguments.String()
		if err := emit(provider.StreamEvent{Type: provider.EventToolCallComplete, ToolCall: event}); err != nil {
			return err
		}
		delete(states, index)
	}
	return nil
}

func openAIUsage(usage *openAIUsagePayload) *provider.Usage {
	if usage == nil {
		return nil
	}
	total := usage.TotalTokens
	if total == 0 {
		total = usage.PromptTokens + usage.CompletionTokens
	}
	return &provider.Usage{
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
		TotalTokens:  total,
	}
}

func openAIEndpoint(baseURL string) string {
	base := strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	return base + "/chat/completions"
}
