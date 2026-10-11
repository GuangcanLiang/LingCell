package model

import (
	"context"
	"errors"
	"net"
)

// ErrorKind identifies a failure without exposing remote or credential data.
type ErrorKind string

// These kinds distinguish configuration, service and protocol failures.
const (
	Configuration  ErrorKind = "configuration"
	Authentication ErrorKind = "authentication"
	RateLimit      ErrorKind = "rate_limit"
	Service        ErrorKind = "service"
	Timeout        ErrorKind = "timeout"
	Canceled       ErrorKind = "canceled"
	Protocol       ErrorKind = "protocol"
)

// Error carries a terminal-safe category and an optional transport cause.
// Use errors.As for Kind and errors.Is for cancellation/deadline identity.
type Error struct {
	Kind  ErrorKind
	cause error
}

// Error returns a fixed summary and next step, never remote response text.
func (e *Error) Error() string {
	switch e.Kind {
	case Configuration:
		return "model configuration missing; configure base_url and model, or use :sh"
	case Authentication:
		return "model authentication failed; check the configured credential environment variable"
	case RateLimit:
		return "model rate limited; wait before submitting a new task"
	case Timeout:
		return "model request timed out; check connectivity or timeout_seconds"
	case Canceled:
		return "model request canceled; continue with shell or submit a new task"
	case Protocol:
		return "model protocol incompatible; check Chat Completions and tool-call support"
	default:
		return "model service unavailable; check service status and connectivity"
	}
}

// Unwrap preserves transport identity without displaying transport details.
func (e *Error) Unwrap() error { return e.cause }

func transportError(err error) error {
	kind := Service
	var timeout net.Error
	switch {
	case errors.Is(err, context.Canceled):
		kind = Canceled
	case errors.Is(err, context.DeadlineExceeded):
		kind = Timeout
	case errors.As(err, &timeout) && timeout.Timeout():
		kind = Timeout
	}
	return &Error{Kind: kind, cause: err}
}

func statusError(status int) error {
	kind := Protocol
	switch {
	case status == 401 || status == 403:
		kind = Authentication
	case status == 429:
		kind = RateLimit
	case status == 408 || status == 504:
		kind = Timeout
	case status >= 500:
		kind = Service
	}
	return &Error{Kind: kind}
}
