// Package apperr turns low-level errors (pgx / database/sql / context) into
// safe, client-facing errors. The raw cause is kept for LOGS only.
package apperr

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type AppError struct {
	Status  int    // HTTP status
	Code    string // stable machine-readable code
	Message string // safe, generic, human-readable text
	Err     error  // internal cause: logged, NEVER sent to the client
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Err.Error()
	}
	return e.Code
}
func (e *AppError) Unwrap() error { return e.Err }

// Public message per constraint name. Names come from the migrations; add rows as the schema grows.
// The constraint name itself is never returned to the client.
var uniqueMessages = map[string]string{
	"users_phone_key": "This phone number is already registered",
}

func internal(err error) *AppError {
	return &AppError{http.StatusInternalServerError, "internal_error", "Something went wrong. Please try again later.", err}
}

// FromDB maps any error to an AppError. Unknown errors become a generic 500.
func FromDB(err error) *AppError {
	if err == nil {
		return nil
	}
	var ae *AppError
	if errors.As(err, &ae) {
		return ae
	}
	if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
		return &AppError{http.StatusNotFound, "not_found", "Resource not found", err}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &AppError{http.StatusServiceUnavailable, "timeout", "The service is busy. Please try again.", err}
	}
	if errors.Is(err, context.Canceled) {
		return &AppError{http.StatusRequestTimeout, "request_canceled", "Request was canceled", err}
	}

	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23505": // unique_violation
			msg, ok := uniqueMessages[pg.ConstraintName]
			if !ok {
				msg = "This record already exists"
			}
			return &AppError{http.StatusConflict, "already_exists", msg, err}
		case "23503": // foreign_key_violation (also raised by our triggers)
			return &AppError{http.StatusConflict, "invalid_reference", "A referenced resource does not exist or is in use", err}
		case "23502": // not_null_violation
			return &AppError{http.StatusBadRequest, "missing_field", "A required field is missing", err}
		case "23514": // check_violation (also raised by our business-rule triggers)
			return &AppError{http.StatusUnprocessableEntity, "invalid_value", "One of the provided values is not allowed", err}
		case "22P02", "22001", "22003", "22007", "22008": // bad text/number/date format or size
			return &AppError{http.StatusBadRequest, "invalid_input", "Invalid input", err}
		case "40001", "40P01": // serialization failure / deadlock -> client may retry
			return &AppError{http.StatusConflict, "conflict_retry", "Please retry the request", err}
		case "57014": // query_canceled (statement_timeout)
			return &AppError{http.StatusServiceUnavailable, "timeout", "The service is busy. Please try again.", err}
		}
		// connection / resource / operator-intervention classes
		if strings.HasPrefix(pg.Code, "08") || strings.HasPrefix(pg.Code, "53") || strings.HasPrefix(pg.Code, "57") {
			return &AppError{http.StatusServiceUnavailable, "service_unavailable", "Service temporarily unavailable", err}
		}
	}
	return internal(err)
}
