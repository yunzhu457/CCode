package llm

import (
	"context"

	"github.com/yunzhu457/CCode/internal/chat"
	"github.com/yunzhu457/CCode/internal/config"
	"github.com/yunzhu457/CCode/internal/provider"
	"github.com/yunzhu457/CCode/internal/provider/factory"
)

type Client interface {
	Name() string
	Stream(ctx context.Context, session *chat.Session, tools []provider.ToolSchema) (<-chan provider.StreamEvent, <-chan error)
}

type providerClient struct {
	provider provider.Provider
}

func New(p provider.Provider) Client {
	return &providerClient{provider: p}
}

func NewFromConfig(cfg config.Config) (Client, error) {
	p, err := factory.New(cfg)
	if err != nil {
		return nil, err
	}
	return New(p), nil
}

func (c *providerClient) Name() string {
	return c.provider.Name()
}

func (c *providerClient) Stream(ctx context.Context, session *chat.Session, tools []provider.ToolSchema) (<-chan provider.StreamEvent, <-chan error) {
	events := make(chan provider.StreamEvent)
	errs := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errs)

		err := c.provider.Stream(ctx, provider.ChatRequest{
			Messages: session.Messages(),
			Tools:    tools,
		}, func(event provider.StreamEvent) error {
			select {
			case events <- event:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		if err != nil {
			errs <- err
		}
	}()

	return events, errs
}
