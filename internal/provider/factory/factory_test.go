package factory

import (
	"testing"

	"github.com/yunzhu457/CCode/internal/config"
)

func TestNewSelectsProvider(t *testing.T) {
	tests := []struct {
		name     string
		protocol config.Protocol
		want     string
	}{
		{name: "openai", protocol: config.ProtocolOpenAI, want: "openai"},
		{name: "anthropic", protocol: config.ProtocolAnthropic, want: "anthropic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := New(config.Config{
				Protocol: tt.protocol,
				Model:    "model",
				BaseURL:  "https://example.test",
				APIKey:   "key",
			})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			if p.Name() != tt.want {
				t.Fatalf("Name() = %q, want %q", p.Name(), tt.want)
			}
		})
	}
}

func TestNewRejectsUnknownProtocol(t *testing.T) {
	_, err := New(config.Config{Protocol: "unknown", Model: "m", BaseURL: "https://example.test", APIKey: "k"})
	if err == nil {
		t.Fatal("New() error = nil, want unknown protocol error")
	}
}
