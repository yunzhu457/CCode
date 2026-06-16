package capability

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/yunzhu457/CCode/internal/config"
)

type Mode string

const (
	ModeOpenAIOfficialResponses         Mode = "openai_official_responses"
	ModeOpenAICompatibleChatCompletions Mode = "openai_compatible_chat_completions"
	ModeAnthropicOfficialMessages       Mode = "anthropic_official_messages"
	ModeAnthropicCompatibleMessages     Mode = "anthropic_compatible_messages"
)

func Resolve(cfg config.Config) (Mode, error) {
	if err := validateProtocolAPI(cfg.Protocol, cfg.API); err != nil {
		return "", err
	}
	if cfg.Compatibility == config.CompatibilityOfficial && !isOfficialProvider(cfg) {
		return "", fmt.Errorf("compatibility %q requires an official provider and base_url", cfg.Compatibility)
	}

	compatibility := cfg.Compatibility
	if compatibility == "" || compatibility == config.CompatibilityAuto {
		if isOfficialProvider(cfg) {
			compatibility = config.CompatibilityOfficial
		} else {
			compatibility = config.CompatibilityCompatible
		}
	}

	switch cfg.Protocol {
	case config.ProtocolOpenAI:
		return resolveOpenAI(compatibility, cfg.API), nil
	case config.ProtocolAnthropic:
		return resolveAnthropic(compatibility, cfg.API), nil
	default:
		return "", fmt.Errorf("unsupported protocol %q", cfg.Protocol)
	}
}

func validateProtocolAPI(protocol config.Protocol, api config.API) error {
	if api == "" || api == config.APIAuto {
		return nil
	}
	switch protocol {
	case config.ProtocolOpenAI:
		if api == config.APIOpenAIResponses || api == config.APIOpenAIChatCompletions {
			return nil
		}
	case config.ProtocolAnthropic:
		if api == config.APIAnthropicMessages {
			return nil
		}
	}
	return fmt.Errorf("api %q is not compatible with protocol %q", api, protocol)
}

func resolveOpenAI(compatibility config.Compatibility, api config.API) Mode {
	if api == config.APIOpenAIChatCompletions {
		return ModeOpenAICompatibleChatCompletions
	}
	if api == config.APIOpenAIResponses {
		return ModeOpenAIOfficialResponses
	}
	if compatibility == config.CompatibilityOfficial {
		return ModeOpenAIOfficialResponses
	}
	return ModeOpenAICompatibleChatCompletions
}

func resolveAnthropic(compatibility config.Compatibility, _ config.API) Mode {
	if compatibility == config.CompatibilityOfficial {
		return ModeAnthropicOfficialMessages
	}
	return ModeAnthropicCompatibleMessages
}

func isOfficialProvider(cfg config.Config) bool {
	switch cfg.Provider {
	case config.ProviderCustom, config.ProviderDeepSeek:
		return false
	case config.ProviderOpenAI:
		return cfg.Protocol == config.ProtocolOpenAI && isOfficialHost(cfg.BaseURL, "api.openai.com")
	case config.ProviderAnthropic:
		return cfg.Protocol == config.ProtocolAnthropic && isOfficialHost(cfg.BaseURL, "api.anthropic.com")
	case "", config.ProviderAuto:
		switch cfg.Protocol {
		case config.ProtocolOpenAI:
			return isOfficialHost(cfg.BaseURL, "api.openai.com")
		case config.ProtocolAnthropic:
			return isOfficialHost(cfg.BaseURL, "api.anthropic.com")
		}
	}
	return false
}

func isOfficialHost(rawURL string, wantHost string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == wantHost
}
