package stream

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/yunzhu457/CCode/internal/sse"
)

func TestNextEventReturnsStreamEvent(t *testing.T) {
	reader, writer := io.Pipe()
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = writer.Write([]byte("data: hello\n\n"))
		_ = writer.Close()
	}()

	event, err := NextEvent(context.Background(), sse.NewReader(reader), time.Second, nil)
	if err != nil {
		t.Fatalf("NextEvent() error = %v", err)
	}
	if event.Data != "hello" {
		t.Fatalf("event.Data = %q, want %q", event.Data, "hello")
	}

	<-done
}

func TestNextEventReturnsIdleTimeout(t *testing.T) {
	reader, writer := io.Pipe()
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})

	interrupted := make(chan struct{}, 1)
	_, err := NextEvent(context.Background(), sse.NewReader(reader), 20*time.Millisecond, func() error {
		select {
		case interrupted <- struct{}{}:
		default:
		}
		return reader.Close()
	})
	if err == nil {
		t.Fatal("NextEvent() error = nil, want idle timeout")
	}

	var timeoutErr *IdleTimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("error = %T %v, want IdleTimeoutError", err, err)
	}
	if timeoutErr.Duration != 20*time.Millisecond {
		t.Fatalf("timeout duration = %s", timeoutErr.Duration)
	}
	if !timeoutErr.Timeout() || !timeoutErr.Temporary() {
		t.Fatal("idle timeout error should behave like a network timeout")
	}

	select {
	case <-interrupted:
	case <-time.After(time.Second):
		t.Fatal("interrupt was not called")
	}
}

func TestNextEventReturnsContextError(t *testing.T) {
	reader, writer := io.Pipe()
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	interrupted := make(chan struct{}, 1)
	_, err := NextEvent(ctx, sse.NewReader(reader), time.Second, func() error {
		select {
		case interrupted <- struct{}{}:
		default:
		}
		return reader.Close()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}

	select {
	case <-interrupted:
	case <-time.After(time.Second):
		t.Fatal("interrupt was not called")
	}
}
