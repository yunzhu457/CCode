package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadValidConfig(t *testing.T) {
	path := writeConfig(t, `
protocol: openai
model: deepseek-v4-flash
base_url: https://api.deepseek.com
api_key: test-key
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Protocol != ProtocolOpenAI {
		t.Fatalf("Protocol = %q, want %q", cfg.Protocol, ProtocolOpenAI)
	}
	if cfg.Model != "deepseek-v4-flash" {
		t.Fatalf("Model = %q", cfg.Model)
	}
	if cfg.BaseURL != "https://api.deepseek.com" {
		t.Fatalf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.APIKey != "test-key" {
		t.Fatal("APIKey was not loaded")
	}
}

func TestLoadRejectsMissingRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name: "protocol",
			content: `
model: deepseek-v4-flash
base_url: https://api.deepseek.com
api_key: test-key
`,
			want: "protocol",
		},
		{
			name: "model",
			content: `
protocol: openai
base_url: https://api.deepseek.com
api_key: test-key
`,
			want: "model",
		},
		{
			name: "base_url",
			content: `
protocol: openai
model: deepseek-v4-flash
api_key: test-key
`,
			want: "base_url",
		},
		{
			name: "api_key",
			content: `
protocol: openai
model: deepseek-v4-flash
base_url: https://api.deepseek.com
`,
			want: "api_key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(writeConfig(t, tt.content))
			if err == nil {
				t.Fatal("Load() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Load() error = %q, want field %q", err, tt.want)
			}
		})
	}
}

func TestLoadSupportsAnthropicThinkingOptions(t *testing.T) {
	path := writeConfig(t, `
provider: anthropic
protocol: anthropic
compatibility: official
api: messages
model: claude-sonnet-4-6
base_url: https://api.anthropic.com
api_key: test-key
max_tokens: 4096
stream:
  idle_timeout: 45s
thinking:
  enabled: true
  budget_tokens: 1024
  display: omitted
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Thinking == nil || !cfg.Thinking.Enabled {
		t.Fatal("Thinking config was not enabled")
	}
	if cfg.Thinking.BudgetTokens != 1024 {
		t.Fatalf("BudgetTokens = %d", cfg.Thinking.BudgetTokens)
	}
	if cfg.Thinking.Display != "omitted" {
		t.Fatalf("Display = %q", cfg.Thinking.Display)
	}
	if cfg.MaxTokens != 4096 {
		t.Fatalf("MaxTokens = %d", cfg.MaxTokens)
	}
	if cfg.Stream == nil {
		t.Fatal("Stream config was not loaded")
	}
	if cfg.Stream.IdleTimeout != 45*time.Second {
		t.Fatalf("IdleTimeout = %s", cfg.Stream.IdleTimeout)
	}
	if cfg.Provider != ProviderAnthropic {
		t.Fatalf("Provider = %q", cfg.Provider)
	}
	if cfg.Compatibility != CompatibilityOfficial {
		t.Fatalf("Compatibility = %q", cfg.Compatibility)
	}
	if cfg.API != APIAnthropicMessages {
		t.Fatalf("API = %q", cfg.API)
	}
}

func TestLoadSupportsCompatibilityRoutingFields(t *testing.T) {
	path := writeConfig(t, `
provider: deepseek
protocol: anthropic
compatibility: auto
api: auto
model: deepseek-v4-flash
base_url: https://api.deepseek.com/anthropic
api_key: test-key
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Provider != ProviderDeepSeek {
		t.Fatalf("Provider = %q", cfg.Provider)
	}
	if cfg.Compatibility != CompatibilityAuto {
		t.Fatalf("Compatibility = %q", cfg.Compatibility)
	}
	if cfg.API != APIAuto {
		t.Fatalf("API = %q", cfg.API)
	}
}

func TestLoadRejectsInvalidRoutingFields(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name: "provider",
			content: `
provider: mystery
protocol: openai
model: m
base_url: https://example.test
api_key: k
`,
			want: "unsupported provider",
		},
		{
			name: "compatibility",
			content: `
protocol: openai
compatibility: maybe
model: m
base_url: https://example.test
api_key: k
`,
			want: "unsupported compatibility",
		},
		{
			name: "api",
			content: `
protocol: openai
api: legacy
model: m
base_url: https://example.test
api_key: k
`,
			want: "unsupported api",
		},
		{
			name: "stream idle timeout",
			content: `
protocol: openai
model: m
base_url: https://example.test
api_key: k
stream:
  idle_timeout: -1s
`,
			want: "stream idle_timeout must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(writeConfig(t, tt.content))
			if err == nil {
				t.Fatal("Load() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Load() error = %q, want %q", err, tt.want)
			}
		})
	}
}

func TestRedactHidesAPIKey(t *testing.T) {
	cfg := Config{APIKey: "secret-value"}
	redacted := cfg.Redacted()
	if strings.Contains(redacted, "secret-value") {
		t.Fatalf("Redacted config leaked API key: %s", redacted)
	}
	if !strings.Contains(redacted, "api_key:<set>") {
		t.Fatalf("Redacted config did not show set marker: %s", redacted)
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
