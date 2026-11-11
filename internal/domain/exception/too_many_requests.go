package exception

import (
	"net/http"
	"time"
)

const CodeTooManyRequests = "TOO_MANY_REQUESTS"

// NewTooManyRequests is for rate limited clients. retryAfter becomes the
// Retry-After header and is also exposed in details.
func NewTooManyRequests(message string, retryAfter time.Duration) AppError {
	return &appError{
		status:     http.StatusTooManyRequests,
		code:       CodeTooManyRequests,
		message:    message,
		details:    retryDetails(retryAfter),
		retryAfter: retryAfter,
	}
}

// retryDetails rounds up so clients never retry a fraction of a second early.
func retryDetails(d time.Duration) map[string]int {
	return map[string]int{"retry_after_seconds": RetryAfterSeconds(d)}
}

// RetryAfterSeconds rounds d up to whole seconds (minimum 1), as used by the
// Retry-After header.
func RetryAfterSeconds(d time.Duration) int {
	s := int((d + time.Second - 1) / time.Second)
	return max(s, 1)
}
