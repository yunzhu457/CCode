package shared

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yunzhu457/CCode/internal/config"
)

func NewHTTPClient() *http.Client {
	return &http.Client{Timeout: 0}
}

func HTTPStatusError(providerName string, resp *http.Response) error {
	return fmt.Errorf("%s request failed: status %d: %s", providerName, resp.StatusCode, ReadLimited(resp.Body))
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
