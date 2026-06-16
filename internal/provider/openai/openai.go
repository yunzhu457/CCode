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
	"github.com/yunzhu457/CCode/internal/sse"
)

type Client struct {
	cfg        config.Config
	httpClient *http.Client
}

func New(cfg config.Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 0,
		},
	}
}

func (c *Client) Name() string {
	return "openai"
}

func (c *Client) Stream(ctx context.Context, req provider.ChatRequest, emit provider.EmitFunc) error {
	body, err := json.Marshal(chatCompletionsRequest{
		Model:    c.cfg.Model,
		Messages: openAIMessages(req.Messages),
		Stream:   true,
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
		return fmt.Errorf("openai request failed: status %d: %s", resp.StatusCode, readLimited(resp.Body))
	}

	reader := sse.NewReader(resp.Body)
	for {
		event, err := reader.Next()
		if err == io.EOF {
			return emit(provider.StreamEvent{Type: provider.EventDone})
		}
		if err != nil {
			return fmt.Errorf("read openai stream: %w", err)
		}
		if strings.TrimSpace(event.Data) == "[DONE]" {
			return emit(provider.StreamEvent{Type: provider.EventDone})
		}
		if event.Data == "" {
			continue
		}

		var chunk chatCompletionChunk
		if err := json.Unmarshal([]byte(event.Data), &chunk); err != nil {
			return fmt.Errorf("decode openai stream event: %w", err)
		}
		for _, choice := range chunk.Choices {
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
		}
	}
}

type chatCompletionsRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
	Stream   bool            `json:"stream"`
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
		} `json:"delta"`
	} `json:"choices"`
}

func openAIMessages(messages []provider.Message) []openAIMessage {
	out := make([]openAIMessage, 0, len(messages))
	for _, msg := range messages {
		out = append(out, openAIMessage{Role: string(msg.Role), Content: msg.Content})
	}
	return out
}

func openAIEndpoint(baseURL string) string {
	base := strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	return base + "/chat/completions"
}

func readLimited(r io.Reader) string {
	const max = 4096
	data, err := io.ReadAll(io.LimitReader(r, max))
	if err != nil {
		return "failed to read response body"
	}
	return strings.TrimSpace(string(data))
}
