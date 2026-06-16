package shared

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yunzhu457/CCode/internal/config"
)

func TestIdleTimeout(t *testing.T) {
	cfg := config.Config{
		Stream: &config.StreamConfig{
			IdleTimeout: 45 * time.Second,
		},
	}
	if got := IdleTimeout(cfg); got != 45*time.Second {
		t.Fatalf("IdleTimeout() = %s", got)
	}
	if got := IdleTimeout(config.Config{}); got != 0 {
		t.Fatalf("IdleTimeout(default) = %s", got)
	}
}

func TestToolInputSchema(t *testing.T) {
	schema := ToolInputSchema(nil)
	if string(schema) != `{"type":"object"}` {
		t.Fatalf("ToolInputSchema(nil) = %s", schema)
	}

	custom := ToolInputSchema([]byte(`{"type":"string"}`))
	if string(custom) != `{"type":"string"}` {
		t.Fatalf("ToolInputSchema(custom) = %s", custom)
	}
}

func TestHTTPStatusError(t *testing.T) {
	resp := &http.Response{
		StatusCode: 502,
		Body:       io.NopCloser(strings.NewReader("bad gateway")),
	}

	err := HTTPStatusError("openai", resp)
	if err == nil {
		t.Fatal("HTTPStatusError() = nil")
	}
	if got := err.Error(); got != "openai request failed: status 502: bad gateway" {
		t.Fatalf("error = %q", got)
	}
}
