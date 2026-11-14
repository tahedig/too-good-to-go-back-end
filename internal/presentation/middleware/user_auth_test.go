package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func devAuthEngine(enabled bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	a := NewDevUserAuth(func(_ context.Context, id int64) (bool, error) { return id == 2, nil }, enabled)
	r := gin.New()
	r.GET("/opt", a.Optional(), func(c *gin.Context) {
		if id, ok := UserID(c); ok {
			c.String(http.StatusOK, "user")
			_ = id
			return
		}
		c.String(http.StatusOK, "anon")
	})
	r.GET("/req", a.Required(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	return r
}

func devAuthCall(r *gin.Engine, path, header string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if header != "" {
		req.Header.Set(DevUserHeader, header)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestDevUserAuthEnabled(t *testing.T) {
	r := devAuthEngine(true)
	if w := devAuthCall(r, "/req", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("required without header: %d", w.Code)
	}
	if w := devAuthCall(r, "/req", "abc"); w.Code != http.StatusUnauthorized {
		t.Fatalf("required with junk: %d", w.Code)
	}
	if w := devAuthCall(r, "/req", "99"); w.Code != http.StatusUnauthorized {
		t.Fatalf("required with unknown user: %d", w.Code)
	}
	if w := devAuthCall(r, "/req", "2"); w.Code != http.StatusNoContent {
		t.Fatalf("required with known user: %d", w.Code)
	}
	if w := devAuthCall(r, "/opt", ""); w.Body.String() != "anon" {
		t.Fatalf("optional anonymous: %q", w.Body.String())
	}
	if w := devAuthCall(r, "/opt", "2"); w.Body.String() != "user" {
		t.Fatalf("optional identified: %q", w.Body.String())
	}
	if w := devAuthCall(r, "/opt", "99"); w.Code != http.StatusUnauthorized {
		t.Fatalf("optional with bad header must not be silently anonymous: %d", w.Code)
	}
}

func TestDevUserAuthDisabledIgnoresHeader(t *testing.T) {
	r := devAuthEngine(false)
	if w := devAuthCall(r, "/req", "2"); w.Code != http.StatusUnauthorized {
		t.Fatalf("disabled: required must be 401 even with a valid header, got %d", w.Code)
	}
	if w := devAuthCall(r, "/opt", "2"); w.Body.String() != "anon" {
		t.Fatalf("disabled: header must be ignored, got %q", w.Body.String())
	}
}
