package llmerr

import (
	"fmt"
	"strings"
	"time"
)

type Details struct {
	Provider   string
	StatusCode int
	Message    string
	Cause      error
}

func (d Details) Unwrap() error {
	return d.Cause
}

func (d Details) format(label string) string {
	prefix := label
	if d.Provider != "" {
		prefix = d.Provider + " " + label
	}

	message := strings.TrimSpace(d.Message)
	if d.Cause != nil {
		cause := strings.TrimSpace(d.Cause.Error())
		if message == "" {
			message = cause
		} else if cause != "" {
			message += ": " + cause
		}
	}

	if d.StatusCode > 0 {
		if message == "" {
			return fmt.Sprintf("%s: status %d", prefix, d.StatusCode)
		}
		return fmt.Sprintf("%s: status %d: %s", prefix, d.StatusCode, message)
	}
	if message == "" {
		return prefix
	}
	return prefix + ": " + message
}

type LLMError struct {
	Details
}

func NewLLM(provider string, statusCode int, message string, cause error) *LLMError {
	return &LLMError{Details: Details{
		Provider:   provider,
		StatusCode: statusCode,
		Message:    message,
		Cause:      cause,
	}}
}

func (e *LLMError) Error() string {
	return e.Details.format("llm error")
}

type AuthenticationError struct {
	Details
}

func NewAuthentication(provider string, statusCode int, message string, cause error) *AuthenticationError {
	return &AuthenticationError{Details: Details{
		Provider:   provider,
		StatusCode: statusCode,
		Message:    message,
		Cause:      cause,
	}}
}

func (e *AuthenticationError) Error() string {
	return e.Details.format("authentication error")
}

type RateLimitError struct {
	Details
	RetryAfter time.Duration
}

func NewRateLimit(provider string, statusCode int, message string, retryAfter time.Duration, cause error) *RateLimitError {
	return &RateLimitError{
		Details: Details{
			Provider:   provider,
			StatusCode: statusCode,
			Message:    message,
			Cause:      cause,
		},
		RetryAfter: retryAfter,
	}
}

func (e *RateLimitError) Error() string {
	message := e.Details.format("rate limit error")
	if e.RetryAfter > 0 {
		message += "; retry after " + e.RetryAfter.String()
	}
	return message
}

type NetworkError struct {
	Details
}

func NewNetwork(provider string, message string, cause error) *NetworkError {
	return NewNetworkWithStatus(provider, 0, message, cause)
}

func NewNetworkWithStatus(provider string, statusCode int, message string, cause error) *NetworkError {
	return &NetworkError{Details: Details{
		Provider:   provider,
		StatusCode: statusCode,
		Message:    message,
		Cause:      cause,
	}}
}

func (e *NetworkError) Error() string {
	return e.Details.format("network error")
}

type ContextTooLongError struct {
	Details
}

func NewContextTooLong(provider string, statusCode int, message string, cause error) *ContextTooLongError {
	return &ContextTooLongError{Details: Details{
		Provider:   provider,
		StatusCode: statusCode,
		Message:    message,
		Cause:      cause,
	}}
}

func (e *ContextTooLongError) Error() string {
	return e.Details.format("context too long error")
}
