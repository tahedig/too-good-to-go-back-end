package apperr

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestFromDB(t *testing.T) {
	secret := `duplicate key value violates unique constraint "users_phone_key" Key (phone)=(09121234567)`
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"unique", fmt.Errorf("insert: %w", &pgconn.PgError{Code: "23505", ConstraintName: "users_phone_key", Message: secret}), http.StatusConflict, "already_exists"},
		{"fk", &pgconn.PgError{Code: "23503", Message: secret}, http.StatusConflict, "invalid_reference"},
		{"check", &pgconn.PgError{Code: "23514", Message: secret}, http.StatusUnprocessableEntity, "invalid_value"},
		{"no rows", pgx.ErrNoRows, http.StatusNotFound, "not_found"},
		{"conn down", &pgconn.PgError{Code: "08006"}, http.StatusServiceUnavailable, "service_unavailable"},
		{"unknown", errors.New("boom: password authentication failed for user app"), http.StatusInternalServerError, "internal_error"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ae := FromDB(c.err)
			if ae.Status != c.status || ae.Code != c.code {
				t.Fatalf("got %d/%s want %d/%s", ae.Status, ae.Code, c.status, c.code)
			}
			// nothing internal may appear in the client-facing message
			for _, leak := range []string{"users_phone_key", "09121234567", "duplicate key", "password", "boom"} {
				if strings.Contains(strings.ToLower(ae.Message), strings.ToLower(leak)) {
					t.Fatalf("message leaks %q: %s", leak, ae.Message)
				}
			}
		})
	}
}
