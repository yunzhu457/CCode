package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Protocol string

const (
	ProtocolOpenAI    Protocol = "openai"
	ProtocolAnthropic Protocol = "anthropic"
)

type ProviderName string

const (
	ProviderAuto      ProviderName = "auto"
	ProviderOpenAI    ProviderName = "openai"
	ProviderAnthropic ProviderName = "anthropic"
	ProviderDeepSeek  ProviderName = "deepseek"
	ProviderCustom    ProviderName = "custom"
)

type Compatibility string

const (
	CompatibilityAuto       Compatibility = "auto"
	CompatibilityOfficial   Compatibility = "official"
	CompatibilityCompatible Compatibility = "compatible"
)

type API string

const (
	APIAuto                  API = "auto"
	APIOpenAIResponses       API = "responses"
	APIOpenAIChatCompletions API = "chat_completions"
	APIAnthropicMessages     API = "messages"
)

type Config struct {
	Provider      ProviderName    `yaml:"provider"`
	Protocol      Protocol        `yaml:"protocol"`
	Compatibility Compatibility   `yaml:"compatibility"`
	API           API             `yaml:"api"`
	Model         string          `yaml:"model"`
	BaseURL       string          `yaml:"base_url"`
	APIKey        string          `yaml:"api_key"`
	MaxTokens     int             `yaml:"max_tokens"`
	Thinking      *ThinkingConfig `yaml:"thinking"`
}

type ThinkingConfig struct {
	Enabled      bool   `yaml:"enabled"`
	BudgetTokens int    `yaml:"budget_tokens"`
	Display      string `yaml:"display"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("load config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate config %s: %w", path, err)
	}
	return cfg, nil
}

func (c Config) Validate() error {
	var missing []string
	if strings.TrimSpace(string(c.Protocol)) == "" {
		missing = append(missing, "protocol")
	}
	if strings.TrimSpace(c.Model) == "" {
		missing = append(missing, "model")
	}
	if strings.TrimSpace(c.BaseURL) == "" {
		missing = append(missing, "base_url")
	}
	if strings.TrimSpace(c.APIKey) == "" {
		missing = append(missing, "api_key")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required field(s): %s", strings.Join(missing, ", "))
	}
	if c.Protocol != ProtocolOpenAI && c.Protocol != ProtocolAnthropic {
		return fmt.Errorf("unsupported protocol %q", c.Protocol)
	}
	switch c.Provider {
	case "", ProviderAuto, ProviderOpenAI, ProviderAnthropic, ProviderDeepSeek, ProviderCustom:
	default:
		return fmt.Errorf("unsupported provider %q", c.Provider)
	}
	switch c.Compatibility {
	case "", CompatibilityAuto, CompatibilityOfficial, CompatibilityCompatible:
	default:
		return fmt.Errorf("unsupported compatibility %q", c.Compatibility)
	}
	switch c.API {
	case "", APIAuto, APIOpenAIResponses, APIOpenAIChatCompletions, APIAnthropicMessages:
	default:
		return fmt.Errorf("unsupported api %q", c.API)
	}
	if c.Thinking != nil {
		if c.Thinking.BudgetTokens < 0 {
			return fmt.Errorf("thinking budget_tokens must be non-negative")
		}
		switch c.Thinking.Display {
		case "", "omitted", "summarized":
		default:
			return fmt.Errorf("thinking display must be omitted or summarized")
		}
	}
	return nil
}

func (c Config) Redacted() string {
	apiKey := "<unset>"
	if strings.TrimSpace(c.APIKey) != "" {
		apiKey = "<set>"
	}
	return fmt.Sprintf(
		"provider:%s protocol:%s compatibility:%s api:%s model:%s base_url:%s api_key:%s",
		c.Provider,
		c.Protocol,
		c.Compatibility,
		c.API,
		c.Model,
		c.BaseURL,
		apiKey,
	)
}
