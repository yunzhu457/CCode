package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
protocol: anthropic
model: claude-sonnet-4-6
base_url: https://api.anthropic.com
api_key: test-key
max_tokens: 4096
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
