package shared

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yunzhu457/CCode/internal/config"
	"github.com/yunzhu457/CCode/internal/llmerr"
)

func TestNewHTTPClientDisablesTotalTimeout(t *testing.T) {
	client := NewHTTPClient()
	if client.Timeout != 0 {
		t.Fatalf("Timeout = %s", client.Timeout)
	}
}

func TestConfigHelpers(t *testing.T) {
	schema := ToolInputSchema(nil)
	if string(schema) != `{"type":"object"}` {
		t.Fatalf("ToolInputSchema(nil) = %s", schema)
	}

	custom := json.RawMessage(`{"type":"string"}`)
	if got := ToolInputSchema(custom); string(got) != string(custom) {
		t.Fatalf("ToolInputSchema(custom) = %s", got)
	}

	cfg := configWithIdle(45 * time.Second)
	if got := IdleTimeout(cfg); got != 45*time.Second {
		t.Fatalf("IdleTimeout() = %s", got)
	}
	if got := IdleTimeout(configWithIdle(0)); got != 0 {
		t.Fatalf("IdleTimeout(zero) = %s", got)
	}
}

func TestHTTPStatusErrorClassifiesAuthentication(t *testing.T) {
	err := HTTPStatusError("openai", response(http.StatusUnauthorized, "", "bad key"))

	var authErr *llmerr.AuthenticationError
	if !errors.As(err, &authErr) {
		t.Fatalf("error = %T, want AuthenticationError", err)
	}
	if authErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("StatusCode = %d", authErr.StatusCode)
	}
}

func TestHTTPStatusErrorClassifiesRateLimitWithRetryAfter(t *testing.T) {
	resp := response(http.StatusTooManyRequests, "slow down", "rate limited")
	resp.Header.Set("Retry-After", "12")

	err := HTTPStatusError("anthropic", resp)

	var rateErr *llmerr.RateLimitError
	if !errors.As(err, &rateErr) {
		t.Fatalf("error = %T, want RateLimitError", err)
	}
	if rateErr.RetryAfter != 12*time.Second {
		t.Fatalf("RetryAfter = %s", rateErr.RetryAfter)
	}
}

func TestHTTPStatusErrorClassifiesContextTooLong(t *testing.T) {
	err := HTTPStatusError("openai", response(http.StatusBadRequest, "", "maximum context length exceeded"))

	var contextErr *llmerr.ContextTooLongError
	if !errors.As(err, &contextErr) {
		t.Fatalf("error = %T, want ContextTooLongError", err)
	}
}

func TestHTTPStatusErrorClassifiesRequestEntityTooLargeAsContextTooLong(t *testing.T) {
	err := HTTPStatusError("anthropic", response(http.StatusRequestEntityTooLarge, "", "payload too large"))

	var contextErr *llmerr.ContextTooLongError
	if !errors.As(err, &contextErr) {
		t.Fatalf("error = %T, want ContextTooLongError", err)
	}
}

func TestHTTPStatusErrorClassifiesGatewayAsNetwork(t *testing.T) {
	err := HTTPStatusError("openai", response(http.StatusBadGateway, "", "upstream closed"))

	var networkErr *llmerr.NetworkError
	if !errors.As(err, &networkErr) {
		t.Fatalf("error = %T, want NetworkError", err)
	}
}

func TestHTTPStatusErrorFallsBackToLLMError(t *testing.T) {
	err := HTTPStatusError("anthropic", response(http.StatusInternalServerError, "", "server broke"))

	var llmErr *llmerr.LLMError
	if !errors.As(err, &llmErr) {
		t.Fatalf("error = %T, want LLMError", err)
	}
}

func TestRetryAfterHTTPDate(t *testing.T) {
	resp := response(http.StatusTooManyRequests, "", "rate limited")
	resp.Header.Set("Retry-After", time.Now().Add(time.Minute).UTC().Format(http.TimeFormat))

	err := HTTPStatusError("openai", resp)

	var rateErr *llmerr.RateLimitError
	if !errors.As(err, &rateErr) {
		t.Fatalf("error = %T, want RateLimitError", err)
	}
	if rateErr.RetryAfter <= 0 {
		t.Fatalf("RetryAfter = %s, want positive duration", rateErr.RetryAfter)
	}
}

func TestWrapNetworkErrorPreservesContextCancellation(t *testing.T) {
	err := WrapNetworkError("openai", "read stream", context.Canceled)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %T, want original context error", err)
	}
}

func TestWrapErrors(t *testing.T) {
	cause := errors.New("bad json")
	llmErr := WrapLLMError("openai", "decode", cause)

	var typedLLMErr *llmerr.LLMError
	if !errors.As(llmErr, &typedLLMErr) {
		t.Fatalf("error = %T, want LLMError", llmErr)
	}
	if !errors.Is(llmErr, cause) {
		t.Fatal("LLMError does not unwrap cause")
	}
	if WrapLLMError("openai", "decode", nil) != nil {
		t.Fatal("WrapLLMError(nil) should return nil")
	}
	if WrapNetworkError("openai", "send", nil) != nil {
		t.Fatal("WrapNetworkError(nil) should return nil")
	}
}

func TestProviderErrorClassifiesStreamErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want any
	}{
		{name: "auth", err: ProviderError("anthropic", "authentication_error", "bad key"), want: &llmerr.AuthenticationError{}},
		{name: "rate", err: ProviderError("anthropic", "rate_limit_error", "slow down"), want: &llmerr.RateLimitError{}},
		{name: "context", err: ProviderError("anthropic", "invalid_request_error", "context length exceeded"), want: &llmerr.ContextTooLongError{}},
		{name: "generic", err: ProviderError("anthropic", "overloaded_error", "overloaded"), want: &llmerr.LLMError{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch want := tt.want.(type) {
			case *llmerr.AuthenticationError:
				if !errors.As(tt.err, &want) {
					t.Fatalf("error = %T, want AuthenticationError", tt.err)
				}
			case *llmerr.RateLimitError:
				if !errors.As(tt.err, &want) {
					t.Fatalf("error = %T, want RateLimitError", tt.err)
				}
			case *llmerr.ContextTooLongError:
				if !errors.As(tt.err, &want) {
					t.Fatalf("error = %T, want ContextTooLongError", tt.err)
				}
			case *llmerr.LLMError:
				if !errors.As(tt.err, &want) {
					t.Fatalf("error = %T, want LLMError", tt.err)
				}
			}
		})
	}
}

func response(status int, retryAfter string, body string) *http.Response {
	resp := &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	if retryAfter != "" {
		resp.Header.Set("Retry-After", retryAfter)
	}
	return resp
}

func configWithIdle(timeout time.Duration) config.Config {
	if timeout == 0 {
		return config.Config{}
	}
	return config.Config{Stream: &config.StreamConfig{IdleTimeout: timeout}}
}
