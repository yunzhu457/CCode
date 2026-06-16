package shared

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/yunzhu457/CCode/internal/config"
	"github.com/yunzhu457/CCode/internal/llmerr"
)

func NewHTTPClient() *http.Client {
	return &http.Client{Timeout: 0}
}

func HTTPStatusError(providerName string, resp *http.Response) error {
	body := ReadLimited(resp.Body)
	status := resp.StatusCode
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return llmerr.NewAuthentication(providerName, status, body, nil)
	case status == http.StatusTooManyRequests:
		return llmerr.NewRateLimit(providerName, status, body, retryAfter(resp.Header.Get("Retry-After")), nil)
	case isContextTooLong(status, body):
		return llmerr.NewContextTooLong(providerName, status, body, nil)
	case isNetworkStatus(status):
		return llmerr.NewNetworkWithStatus(providerName, status, body, nil)
	default:
		return llmerr.NewLLM(providerName, status, body, nil)
	}
}

func WrapNetworkError(providerName string, operation string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return llmerr.NewNetwork(providerName, operation, err)
}

func WrapLLMError(providerName string, operation string, err error) error {
	if err == nil {
		return nil
	}
	return llmerr.NewLLM(providerName, 0, operation, err)
}

func ProviderError(providerName string, errorType string, message string) error {
	text := strings.TrimSpace(strings.TrimSpace(errorType) + ": " + strings.TrimSpace(message))
	if strings.HasPrefix(text, ":") {
		text = strings.TrimSpace(strings.TrimPrefix(text, ":"))
	}
	lower := strings.ToLower(errorType + " " + message)
	switch {
	case strings.Contains(lower, "auth"):
		return llmerr.NewAuthentication(providerName, 0, text, nil)
	case strings.Contains(lower, "rate_limit") || strings.Contains(lower, "rate limit"):
		return llmerr.NewRateLimit(providerName, 0, text, 0, nil)
	case isContextTooLong(400, lower):
		return llmerr.NewContextTooLong(providerName, 0, text, nil)
	default:
		return llmerr.NewLLM(providerName, 0, text, nil)
	}
}

func IdleTimeout(cfg config.Config) time.Duration {
	if cfg.Stream != nil {
		return cfg.Stream.IdleTimeout
	}
	return 0
}

func ToolInputSchema(schema json.RawMessage) json.RawMessage {
	if len(schema) > 0 {
		return schema
	}
	return json.RawMessage(`{"type":"object"}`)
}

func ReadLimited(r io.Reader) string {
	const max = 4096
	data, err := io.ReadAll(io.LimitReader(r, max))
	if err != nil {
		return "failed to read response body"
	}
	return strings.TrimSpace(string(data))
}

func retryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		delay := time.Until(when)
		if delay > 0 {
			return delay
		}
	}
	return 0
}

func isNetworkStatus(status int) bool {
	switch status {
	case http.StatusRequestTimeout, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func isContextTooLong(status int, body string) bool {
	if status == http.StatusRequestEntityTooLarge {
		return true
	}
	if status != http.StatusBadRequest && status != http.StatusUnprocessableEntity {
		return false
	}
	lower := strings.ToLower(body)
	for _, marker := range []string{
		"context length",
		"context too long",
		"context_length_exceeded",
		"maximum context",
		"context window",
		"too many tokens",
		"token limit",
		"prompt is too long",
		"input is too long",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
