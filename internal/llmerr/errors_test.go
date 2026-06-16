package llmerr

import (
	"errors"
	"testing"
	"time"
)

func TestTypedErrorsExposeCommonDetails(t *testing.T) {
	cause := errors.New("socket closed")
	err := NewNetwork("openai", "read stream", cause)

	if err.Provider != "openai" {
		t.Fatalf("Provider = %q", err.Provider)
	}
	if !errors.Is(err, cause) {
		t.Fatal("network error does not unwrap cause")
	}
	if got := err.Error(); got != "openai network error: read stream: socket closed" {
		t.Fatalf("Error() = %q", got)
	}
}

func TestRateLimitErrorCarriesRetryAfter(t *testing.T) {
	err := NewRateLimit("anthropic", 429, "too many requests", 15*time.Second, nil)

	if err.RetryAfter != 15*time.Second {
		t.Fatalf("RetryAfter = %s", err.RetryAfter)
	}
	if got := err.Error(); got != "anthropic rate limit error: status 429: too many requests; retry after 15s" {
		t.Fatalf("Error() = %q", got)
	}
}

func TestSpecificErrorsFormatWithoutCause(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "llm",
			err:  NewLLM("openai", 500, "server error", nil),
			want: "openai llm error: status 500: server error",
		},
		{
			name: "authentication",
			err:  NewAuthentication("anthropic", 401, "bad key", nil),
			want: "anthropic authentication error: status 401: bad key",
		},
		{
			name: "context too long",
			err:  NewContextTooLong("openai", 400, "maximum context length exceeded", nil),
			want: "openai context too long error: status 400: maximum context length exceeded",
		},
		{
			name: "network with status",
			err:  NewNetworkWithStatus("openai", 502, "bad gateway", nil),
			want: "openai network error: status 502: bad gateway",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Fatalf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestErrorFormattingHandlesSparseDetails(t *testing.T) {
	if got := NewLLM("", 0, "", nil).Error(); got != "llm error" {
		t.Fatalf("empty LLMError = %q", got)
	}
	if got := NewLLM("openai", 500, "", nil).Error(); got != "openai llm error: status 500" {
		t.Fatalf("status-only LLMError = %q", got)
	}
}
