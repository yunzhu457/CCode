package factory

import (
	"fmt"

	"github.com/yunzhu457/CCode/internal/config"
	"github.com/yunzhu457/CCode/internal/provider"
	"github.com/yunzhu457/CCode/internal/provider/anthropic"
	"github.com/yunzhu457/CCode/internal/provider/capability"
	"github.com/yunzhu457/CCode/internal/provider/openai"
)

func New(cfg config.Config) (provider.Provider, error) {
	mode, err := capability.Resolve(cfg)
	if err != nil {
		return nil, err
	}

	switch mode {
	case capability.ModeOpenAICompatibleChatCompletions:
		return openai.New(cfg), nil
	case capability.ModeAnthropicCompatibleMessages:
		return anthropic.New(cfg), nil
	case capability.ModeOpenAIOfficialResponses:
		if requiresOfficialImplementation(cfg, mode) {
			return nil, officialModeError(mode)
		}
		return openai.New(cfg), nil
	case capability.ModeAnthropicOfficialMessages:
		if requiresOfficialImplementation(cfg, mode) {
			return nil, officialModeError(mode)
		}
		return anthropic.New(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported provider mode %q", mode)
	}
}

func requiresOfficialImplementation(cfg config.Config, mode capability.Mode) bool {
	if cfg.Compatibility == config.CompatibilityOfficial {
		return true
	}
	return mode == capability.ModeOpenAIOfficialResponses && cfg.API == config.APIOpenAIResponses
}

func officialModeError(mode capability.Mode) error {
	return fmt.Errorf(
		"provider mode %q is not implemented yet; use compatibility: auto/compatible with a compatible api for now",
		mode,
	)
}
