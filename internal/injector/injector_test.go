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

	"github.com/alicebob/miniredis/v2"

	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
)

func testConfig() *bootstrap.Config {
	return &bootstrap.Config{
		AppEnv:             bootstrap.EnvDevelopment,
		HealthCheckTimeout: time.Second,
		OTPLength:          6,
		OTPTTL:             time.Minute,
		OTPMaxAttempts:     3,
		SMSProvider:        bootstrap.SMSProviderFake,
		RedisKeyPrefix:     "test:",
	}
}

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

func health(t *testing.T, app *App) map[string]any {
	t.Helper()
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	var body map[string]any
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &body) != nil {
		t.Fatalf("/health: %d %s", w.Code, w.Body.String())
	}
	return body
}

// The generated graph must build and serve requests with the in-memory stores.
func TestInitializeAppInMemory(t *testing.T) {
	app, cleanup, err := InitializeApp(testConfig(), discard, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	if body := health(t, app); body["checks"] != nil {
		t.Fatalf("no dependency checks expected without redis: %v", body)
	}

	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/auth/otp/request",
		strings.NewReader(`{"phone":"09123456789"}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("otp request through the wired app: %d %s", w.Code, w.Body.String())
	}
}

func TestInitializeAppWithRedis(t *testing.T) {
	mr := miniredis.RunT(t)
	cfg := testConfig()
	cfg.RedisURL = "redis://" + mr.Addr()

	app, cleanup, err := InitializeApp(cfg, discard, "test")
	if err != nil {
		t.Fatal(err)
	}
	if checks, _ := health(t, app)["checks"].(map[string]any); checks["redis"] == nil {
		t.Fatal("redis should be registered in /health")
	}

	cleanup()
	// miniredis notices closed connections asynchronously, so poll briefly.
	deadline := time.Now().Add(2 * time.Second)
	for mr.CurrentConnectionCount() != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := mr.CurrentConnectionCount(); n != 0 {
		t.Fatalf("cleanup should close redis connections, %d open", n)
	}
}

func TestInitializeAppFailsOnUnreachableRedis(t *testing.T) {
	cfg := testConfig()
	cfg.RedisURL = "redis://127.0.0.1:1"
	if _, _, err := InitializeApp(cfg, discard, "test"); err == nil {
		t.Fatal("startup must fail when redis is unreachable")
	}
}
