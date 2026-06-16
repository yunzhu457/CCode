package factory

import (
	"testing"

	"github.com/yunzhu457/CCode/internal/config"
)

func TestNewSelectsProvider(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
		want string
	}{
		{
			name: "openai compatible by custom base url",
			cfg: config.Config{
				Protocol: config.ProtocolOpenAI,
				Model:    "model",
				BaseURL:  "https://example.test",
				APIKey:   "key",
			},
			want: "openai",
		},
		{
			name: "anthropic compatible by deepseek provider",
			cfg: config.Config{
				Provider: config.ProviderDeepSeek,
				Protocol: config.ProtocolAnthropic,
				Model:    "model",
				BaseURL:  "https://api.deepseek.com/anthropic",
				APIKey:   "key",
			},
			want: "anthropic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := New(tt.cfg)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			if p.Name() != tt.want {
				t.Fatalf("Name() = %q, want %q", p.Name(), tt.want)
			}
		})
	}
}

func TestNewFallsBackToCompatibleProviderWhenOfficialModeIsAuto(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
		want string
	}{
		{
			name: "openai official host auto",
			cfg: config.Config{
				Provider: config.ProviderOpenAI,
				Protocol: config.ProtocolOpenAI,
				Model:    "gpt-5.5",
				BaseURL:  "https://api.openai.com/v1",
				APIKey:   "key",
			},
			want: "openai",
		},
		{
			name: "anthropic official host auto",
			cfg: config.Config{
				Provider: config.ProviderAnthropic,
				Protocol: config.ProtocolAnthropic,
				Model:    "claude-sonnet-4-6",
				BaseURL:  "https://api.anthropic.com",
				APIKey:   "key",
			},
			want: "anthropic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := New(tt.cfg)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			if p.Name() != tt.want {
				t.Fatalf("Name() = %q, want %q", p.Name(), tt.want)
			}
		})
	}
}

func TestNewReportsOfficialModesNotImplemented(t *testing.T) {
	tests := []config.Config{
		{
			Provider:      config.ProviderOpenAI,
			Protocol:      config.ProtocolOpenAI,
			Compatibility: config.CompatibilityOfficial,
			Model:         "gpt-5.5",
			BaseURL:       "https://api.openai.com/v1",
			APIKey:        "key",
		},
		{
			Provider:      config.ProviderAnthropic,
			Protocol:      config.ProtocolAnthropic,
			Compatibility: config.CompatibilityOfficial,
			Model:         "claude-sonnet-4-6",
			BaseURL:       "https://api.anthropic.com",
			APIKey:        "key",
		},
		{
			Provider: config.ProviderOpenAI,
			Protocol: config.ProtocolOpenAI,
			API:      config.APIOpenAIResponses,
			Model:    "gpt-5.5",
			BaseURL:  "https://api.openai.com/v1",
			APIKey:   "key",
		},
	}

	for _, cfg := range tests {
		_, err := New(cfg)
		if err == nil {
			t.Fatal("New() error = nil, want official mode placeholder error")
		}
	}
}

func TestNewRejectsUnknownProtocol(t *testing.T) {
	_, err := New(config.Config{Protocol: "unknown", Model: "m", BaseURL: "https://example.test", APIKey: "k"})
	if err == nil {
		t.Fatal("New() error = nil, want unknown protocol error")
	}
}
