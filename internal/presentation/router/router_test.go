package router_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	"github.com/tahedig/too-good-to-go/backend/internal/application/service"
	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/handler"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/router"
	"github.com/tahedig/too-good-to-go/backend/pkg/logger"
)

type checker struct{ err error }

func (checker) Name() string                    { return "postgres" }
func (c checker) Check(_ context.Context) error { return c.err }

func newTestRouter(t *testing.T, logs *bytes.Buffer, checkers ...serviceinterface.HealthChecker) (*gin.Engine, serviceinterface.HealthService) {
	t.Helper()
	log := logger.New("debug", logs)
	svc := service.NewHealthService("test", time.Second, log, checkers...)
	r, err := router.New(&bootstrap.Config{}, log, middleware.NewErrorHandler(log), nil,
		router.Handlers{
			Health: handler.NewHealthHandler(svc),
			Auth:   handler.NewAuthHandler(nil),
		})
	if err != nil {
		t.Fatal(err)
	}
	return r, svc
}

func do(r http.Handler, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w
}

func decodeError(t *testing.T, w *httptest.ResponseRecorder) dto.ErrorBody {
	t.Helper()
	var body dto.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not the standard error JSON: %v\n%s", err, w.Body.String())
	}
	if body.Error.RequestID == "" || body.Error.RequestID != w.Header().Get(bootstrap.HeaderRequestID) {
		t.Errorf("request_id missing or not matching header: %+v", body.Error)
	}
	return body.Error
}

func TestHealthStatusCodes(t *testing.T) {
	var logs bytes.Buffer

	r, svc := newTestRouter(t, &logs, checker{})
	if w := do(r, http.MethodGet, "/health"); w.Code != http.StatusOK {
		t.Fatalf("healthy: got %d, body %s", w.Code, w.Body.String())
	}
	if w := do(r, http.MethodHead, "/health"); w.Code != http.StatusOK || w.Body.Len() != 0 {
		t.Fatalf("HEAD: got %d with %d body bytes", w.Code, w.Body.Len())
	}
	svc.MarkShuttingDown()
	if w := do(r, http.MethodGet, "/health"); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("shutting down: got %d", w.Code)
	}

	r, _ = newTestRouter(t, &logs, checker{err: errors.New("db down")})
	w := do(r, http.MethodGet, "/health")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("dependency down: got %d", w.Code)
	}
	if bytes.Contains(w.Body.Bytes(), []byte("db down")) {
		t.Fatal("health response must not leak internal error text")
	}
}

func TestGlobalErrorFormat(t *testing.T) {
	var logs bytes.Buffer
	r, _ := newTestRouter(t, &logs)
	r.GET("/boom", func(*gin.Context) { panic("something broke") })
	r.GET("/conflict", func(c *gin.Context) { _ = c.Error(exception.NewConflict("offer already reserved")) })
	r.GET("/plain", func(c *gin.Context) { _ = c.Error(errors.New("sql: secret internals")) })

	cases := []struct {
		method, path string
		status       int
		code         string
	}{
		{http.MethodGet, "/nope", http.StatusNotFound, exception.CodeNotFound},
		{http.MethodPost, "/health", http.StatusMethodNotAllowed, exception.CodeMethodNotAllowed},
		{http.MethodGet, "/boom", http.StatusInternalServerError, exception.CodeInternalServer},
		{http.MethodGet, "/conflict", http.StatusConflict, exception.CodeConflict},
		{http.MethodGet, "/plain", http.StatusInternalServerError, exception.CodeInternalServer},
	}
	for _, tc := range cases {
		w := do(r, tc.method, tc.path)
		if w.Code != tc.status {
			t.Errorf("%s %s: status %d, want %d", tc.method, tc.path, w.Code, tc.status)
			continue
		}
		if got := decodeError(t, w); got.Code != tc.code {
			t.Errorf("%s %s: code %s, want %s", tc.method, tc.path, got.Code, tc.code)
		}
		if bytes.Contains(w.Body.Bytes(), []byte("secret internals")) {
			t.Errorf("%s: internal error details leaked to client", tc.path)
		}
	}

	// Every log line must be valid JSON.
	for _, line := range bytes.Split(bytes.TrimSpace(logs.Bytes()), []byte("\n")) {
		if !json.Valid(line) {
			t.Fatalf("non-JSON log line: %s", line)
		}
	}
}

func TestIncomingRequestIDIsReusedAndValidated(t *testing.T) {
	var logs bytes.Buffer
	r, _ := newTestRouter(t, &logs)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set(bootstrap.HeaderRequestID, "abc-123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if got := w.Header().Get(bootstrap.HeaderRequestID); got != "abc-123" {
		t.Fatalf("expected incoming id to be reused, got %q", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set(bootstrap.HeaderRequestID, "bad\nid")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if got := w.Header().Get(bootstrap.HeaderRequestID); got == "bad\nid" || got == "" {
		t.Fatalf("invalid incoming id should be replaced, got %q", got)
	}
}
