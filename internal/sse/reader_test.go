package sse

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestReaderReadsEvents(t *testing.T) {
	input := strings.NewReader("event: content_block_delta\ndata: {\"hello\":\"wor\ndata: ld\"}\n\n: keepalive\n\ndata: [DONE]\n\n")
	reader := NewReader(input)

	first, err := reader.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if first.Event != "content_block_delta" {
		t.Fatalf("Event = %q", first.Event)
	}
	if first.Data != "{\"hello\":\"wor\nld\"}" {
		t.Fatalf("Data = %q", first.Data)
	}

	second, err := reader.Next()
	if err != nil {
		t.Fatalf("Next() second error = %v", err)
	}
	if second.Event != "" || second.Data != "[DONE]" {
		t.Fatalf("second = %+v", second)
	}

	_, err = reader.Next()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("Next() final error = %v, want io.EOF", err)
	}
}

func TestReaderSkipsCommentOnlyEvents(t *testing.T) {
	reader := NewReader(strings.NewReader(": ping\n\n"))

	_, err := reader.Next()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("Next() error = %v, want io.EOF", err)
	}
}

func TestReaderReturnsPendingEventAtEOF(t *testing.T) {
	reader := NewReader(strings.NewReader("event: done\ndata: final"))

	event, err := reader.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if event.Event != "done" || event.Data != "final" {
		t.Fatalf("event = %+v", event)
	}
}
