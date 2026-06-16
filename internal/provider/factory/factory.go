package factory

import (
	"fmt"

	"github.com/yunzhu457/CCode/internal/config"
	"github.com/yunzhu457/CCode/internal/provider"
	"github.com/yunzhu457/CCode/internal/provider/anthropic"
	"github.com/yunzhu457/CCode/internal/provider/openai"
)

func New(cfg config.Config) (provider.Provider, error) {
	switch cfg.Protocol {
	case config.ProtocolOpenAI:
		return openai.New(cfg), nil
	case config.ProtocolAnthropic:
		return anthropic.New(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported protocol %q", cfg.Protocol)
	}
}
