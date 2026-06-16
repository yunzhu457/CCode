package capability

import (
	"strings"
	"testing"

	"github.com/yunzhu457/CCode/internal/config"
)

func TestResolveAutoRoutesOfficialProvidersToOfficialModes(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
		want Mode
	}{
		{
			name: "openai official responses",
			cfg: config.Config{
				Provider: config.ProviderOpenAI,
				Protocol: config.ProtocolOpenAI,
				BaseURL:  "https://api.openai.com/v1",
				API:      config.APIAuto,
			},
			want: ModeOpenAIOfficialResponses,
		},
		{
			name: "anthropic official sdk",
			cfg: config.Config{
				Provider: config.ProviderAnthropic,
				Protocol: config.ProtocolAnthropic,
				BaseURL:  "https://api.anthropic.com",
				API:      config.APIAuto,
			},
			want: ModeAnthropicOfficialMessages,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(tt.cfg)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("Resolve() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveAutoDowngradesNonOfficialProvidersToCompatibleModes(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
		want Mode
	}{
		{
			name: "deepseek anthropic compatible",
			cfg: config.Config{
				Provider: config.ProviderDeepSeek,
				Protocol: config.ProtocolAnthropic,
				BaseURL:  "https://api.deepseek.com/anthropic",
			},
			want: ModeAnthropicCompatibleMessages,
		},
		{
			name: "custom openai compatible relay",
			cfg: config.Config{
				Provider: config.ProviderCustom,
				Protocol: config.ProtocolOpenAI,
				BaseURL:  "https://relay.example.com/v1",
			},
			want: ModeOpenAICompatibleChatCompletions,
		},
		{
			name: "missing provider uses base url to downgrade",
			cfg: config.Config{
				Protocol: config.ProtocolOpenAI,
				BaseURL:  "https://api.deepseek.com",
			},
			want: ModeOpenAICompatibleChatCompletions,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(tt.cfg)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("Resolve() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveHonorsExplicitCompatibilityOverride(t *testing.T) {
	cfg := config.Config{
		Provider:      config.ProviderOpenAI,
		Protocol:      config.ProtocolOpenAI,
		Compatibility: config.CompatibilityCompatible,
		BaseURL:       "https://api.openai.com/v1",
	}

	got, err := Resolve(cfg)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got != ModeOpenAICompatibleChatCompletions {
		t.Fatalf("Resolve() = %q", got)
	}
}

func TestResolveRejectsMismatchedExplicitAPI(t *testing.T) {
	_, err := Resolve(config.Config{
		Protocol: config.ProtocolOpenAI,
		BaseURL:  "https://api.openai.com/v1",
		API:      config.APIAnthropicMessages,
	})
	if err == nil {
		t.Fatal("Resolve() error = nil, want mismatch error")
	}
}

func TestResolveRejectsOfficialCompatibilityForNonOfficialProvider(t *testing.T) {
	_, err := Resolve(config.Config{
		Provider:      config.ProviderDeepSeek,
		Protocol:      config.ProtocolAnthropic,
		Compatibility: config.CompatibilityOfficial,
		BaseURL:       "https://api.deepseek.com/anthropic",
	})
	if err == nil {
		t.Fatal("Resolve() error = nil, want official provider error")
	}
	if !strings.Contains(err.Error(), "official provider") {
		t.Fatalf("Resolve() error = %q", err)
	}
}
