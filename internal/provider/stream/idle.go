package stream

import (
	"context"
	"fmt"
	"time"

	"github.com/yunzhu457/CCode/internal/sse"
)

const DefaultIdleTimeout = 30 * time.Second

type IdleTimeoutError struct {
	Duration time.Duration
}

func (e *IdleTimeoutError) Error() string {
	return fmt.Sprintf("network idle timeout after %s", e.Duration)
}

func (e *IdleTimeoutError) Timeout() bool {
	return true
}

func (e *IdleTimeoutError) Temporary() bool {
	return true
}

type readResult struct {
	event sse.Event
	err   error
}

func NextEvent(ctx context.Context, reader *sse.Reader, idle time.Duration, interrupt func() error) (sse.Event, error) {
	if idle <= 0 {
		idle = DefaultIdleTimeout
	}

	results := make(chan readResult, 1)
	go func() {
		event, err := reader.Next()
		results <- readResult{event: event, err: err}
	}()

	timer := time.NewTimer(idle)
	defer timer.Stop()

	select {
	case result := <-results:
		return result.event, result.err
	case <-ctx.Done():
		if interrupt != nil {
			_ = interrupt()
		}
		return sse.Event{}, ctx.Err()
	case <-timer.C:
		if interrupt != nil {
			_ = interrupt()
		}
		return sse.Event{}, &IdleTimeoutError{Duration: idle}
	}
}
