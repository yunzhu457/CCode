package anthropic

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
	for {
		event, err := reader.Next()
		if err == io.EOF {
			return emit(provider.StreamEvent{Type: provider.EventDone})
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
		case "message_stop":
			return emit(provider.StreamEvent{Type: provider.EventDone})
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
			}
		}
	}
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
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

type streamChunk struct {
	Type  string `json:"type"`
	Delta struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		Thinking string `json:"thinking"`
	} `json:"delta"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
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

func anthropicEndpoint(baseURL string) string {
	base := strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(base, "/v1/messages") {
		return base
	}
	return base + "/v1/messages"
}

func readLimited(r io.Reader) string {
	const max = 4096
	data, err := io.ReadAll(io.LimitReader(r, max))
	if err != nil {
		return "failed to read response body"
	}
	return strings.TrimSpace(string(data))
}
