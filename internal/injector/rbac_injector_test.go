package injector

import (
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
