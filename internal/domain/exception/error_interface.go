// Package exception defines the typed errors every layer returns. The global
// error middleware turns them into the standard JSON error response, so
// handlers never build error bodies themselves.
package exception

import "time"

// AppError is an error that knows how it should be presented to a client.
type AppError interface {
	error
	StatusCode() int
	// Code is a stable, machine readable identifier (e.g. "NOT_FOUND").
	// Frontends should switch on this, never on Message.
	Code() string
	// Message is safe to show to the client.
	Message() string
	// Details is optional extra data, e.g. field validation errors.
	Details() any
}

type appError struct {
	status     int
	code       string
	message    string
	details    any
	cause      error
	retryAfter time.Duration
}

func (e *appError) Error() string {
	if e.cause != nil {
		return e.message + ": " + e.cause.Error()
	}
	return e.message
}

func (e *appError) StatusCode() int { return e.status }
func (e *appError) Code() string    { return e.code }
func (e *appError) Message() string { return e.message }
func (e *appError) Details() any    { return e.details }
func (e *appError) Unwrap() error   { return e.cause }

// Retryable is implemented by errors that tell the client when it may try
// again (typically 429). The error middleware turns it into a Retry-After
// header.
type Retryable interface {
	RetryAfter() time.Duration
}

func (e *appError) RetryAfter() time.Duration { return e.retryAfter }

// FieldError describes one invalid input field inside Details.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}
