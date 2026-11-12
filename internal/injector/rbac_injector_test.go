package injector

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
)

// The generated graph must build and protect the RBAC routes.
func TestInitializeAppProtectsRoutes(t *testing.T) {
	cfg := &bootstrap.Config{
		AppEnv:             bootstrap.EnvDevelopment,
		HealthCheckTimeout: time.Second,
		JWTSecret:          strings.Repeat("s", 32),
		JWTIssuer:          "test",
		JWTAccessTTL:       time.Hour,
	}
	app, cleanup, err := InitializeApp(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	for path, want := range map[string]int{"/health": http.StatusOK, "/auth/me": http.StatusUnauthorized, "/test/admin": http.StatusUnauthorized} {
		w := httptest.NewRecorder()
		app.Router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != want {
			t.Errorf("%s: got %d, want %d", path, w.Code, want)
		}
	}
}

// End to end through the real graph: OTP request -> verify -> token -> RBAC route.
func TestInitializeAppOTPLoginIssuesToken(t *testing.T) {
	cfg := &bootstrap.Config{
		AppEnv:             bootstrap.EnvDevelopment,
		HealthCheckTimeout: time.Second,
		JWTSecret:          strings.Repeat("s", 32),
		JWTIssuer:          "test",
		JWTAccessTTL:       time.Hour,
		OTPLength:          6,
		OTPTTL:             time.Minute,
		OTPMaxAttempts:     3,
		OTPExposeCode:      true,
		VendorPhones:       []string{"+989121111111"},
	}
	app, cleanup, err := InitializeApp(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	call := func(method, path, body, bearer string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		w := httptest.NewRecorder()
		app.Router.ServeHTTP(w, req)
		return w
	}

	w := call(http.MethodPost, "/auth/otp/request", `{"phone":"09121111111"}`, "")
	var otp struct {
		DebugCode string `json:"debug_code"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &otp) != nil || otp.DebugCode == "" {
		t.Fatalf("request: %d %s", w.Code, w.Body.String())
	}

	w = call(http.MethodPost, "/auth/otp/verify", `{"phone":"09121111111","code":"`+otp.DebugCode+`"}`, "")
	var session struct {
		AccessToken string `json:"access_token"`
		User        struct {
			Role string `json:"role"`
		} `json:"user"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &session) != nil || session.AccessToken == "" {
		t.Fatalf("verify: %d %s", w.Code, w.Body.String())
	}
	if session.User.Role != "vendor" {
		t.Fatalf("VENDOR_PHONES not applied: role %q", session.User.Role)
	}

	if w := call(http.MethodGet, "/test/vendor", "", session.AccessToken); w.Code != http.StatusOK {
		t.Fatalf("/test/vendor with login token: %d %s", w.Code, w.Body.String())
	}
}
