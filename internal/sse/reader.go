package sse

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

type Event struct {
	Event string
	Data  string
}

type Reader struct {
	r *bufio.Reader
}

func NewReader(r io.Reader) *Reader {
	return &Reader{r: bufio.NewReader(r)}
}

func (r *Reader) Next() (Event, error) {
	var event Event
	var data []string
	seenField := false

	for {
		line, err := r.r.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return Event{}, fmt.Errorf("read sse line: %w", err)
		}

		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")

		if line == "" {
			if seenField {
				event.Data = strings.Join(data, "\n")
				return event, nil
			}
			if errors.Is(err, io.EOF) {
				return Event{}, io.EOF
			}
			continue
		}

		if strings.HasPrefix(line, ":") {
			if errors.Is(err, io.EOF) && seenField {
				event.Data = strings.Join(data, "\n")
				return event, nil
			}
			if errors.Is(err, io.EOF) {
				return Event{}, io.EOF
			}
			continue
		}

		field, value, ok := strings.Cut(line, ":")
		if ok {
			value = strings.TrimPrefix(value, " ")
		} else {
			value = ""
		}

		switch field {
		case "event":
			event.Event = value
			seenField = true
		case "data":
			data = append(data, value)
			seenField = true
		}

		if errors.Is(err, io.EOF) {
			if seenField {
				event.Data = strings.Join(data, "\n")
				return event, nil
			}
			return Event{}, io.EOF
		}
	}
}
