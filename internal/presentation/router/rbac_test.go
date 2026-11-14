package router_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	"github.com/tahedig/too-good-to-go/backend/internal/application/service"
	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/repository/memory"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/token"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/handler"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/router"
	"github.com/tahedig/too-good-to-go/backend/pkg/logger"
)

const (
	rbacCustomerPhone = "+989123456789"
	rbacVendorPhone   = "+989121111111"
	rbacAdminPhone    = "+989122222222"
	rbacSecret        = "test-secret-test-secret-test-secret-123"
	rbacIssuer        = "test-issuer"
)

type rbacEnv struct {
	r        *gin.Engine
	sessions serviceinterface.SessionService
	users    *memory.UserRepository
}

func newRBACEnv(t *testing.T) *rbacEnv {
	t.Helper()
	log := logger.New("debug", &bytes.Buffer{})
	tokens := token.NewJWTManager(rbacSecret, rbacIssuer, time.Hour)
	users := memory.NewUserRepository()
	sessions := service.NewSessionService(users, tokens,
		map[string]enum.Role{rbacVendorPhone: enum.RoleVendor, rbacAdminPhone: enum.RoleAdmin}, log)
	health := service.NewHealthService("test", time.Second, log)

	r, err := router.New(&bootstrap.Config{}, log, middleware.NewErrorHandler(log), middleware.NewAuthMiddleware(sessions), nil, router.Handlers{
		Health:    handler.NewHealthHandler(health),
		Auth:      handler.NewAuthHandler(nil, nil),
		Protected: handler.NewProtectedHandler(),
		Profile:   handler.NewProfileHandler(service.NewProfileService(users)),
	})
	if err != nil {
		t.Fatal(err)
	}
	return &rbacEnv{r: r, sessions: sessions, users: users}
}

// login issues a real session for phone, as a verified login would.
func (e *rbacEnv) login(t *testing.T, phone string) *serviceinterface.AuthResult {
	t.Helper()
	res, err := e.sessions.Login(context.Background(), phone)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func (e *rbacEnv) get(path, authHeader string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	return w
}

func (e *rbacEnv) patch(path, authHeader, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	return w
}

func TestMeReturnsTokenIdentity(t *testing.T) {
	e := newRBACEnv(t)
	session := e.login(t, rbacCustomerPhone)

	w := e.get("/auth/me", "Bearer "+session.AccessToken)
	var me dto.MeResponse
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &me) != nil {
		t.Fatalf("/auth/me: %d %s", w.Code, w.Body.String())
	}
	if me.UserID != session.User.ID || me.Role != string(enum.RoleCustomer) {
		t.Fatalf("identity mismatch: %+v vs user %+v", me, session.User)
	}
}

func TestCustomerCanReadAndUpdateOwnProfile(t *testing.T) {
	e := newRBACEnv(t)
	session := e.login(t, rbacCustomerPhone)
	auth := "Bearer " + session.AccessToken

	w := e.get("/customer/profile", auth)
	var profile dto.ProfileResponse
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &profile) != nil {
		t.Fatalf("GET /customer/profile: %d %s", w.Code, w.Body.String())
	}
	if profile.UserID != session.User.ID || profile.Phone != rbacCustomerPhone || profile.DisplayName != "" {
		t.Fatalf("unexpected profile: %+v", profile)
	}

	w = e.patch("/customer/profile", auth,
		`{"display_name":"  Sam Example  ","phone":"+989111111111","role":"admin","status":"deleted"}`)
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &profile) != nil {
		t.Fatalf("PATCH /customer/profile: %d %s", w.Code, w.Body.String())
	}
	if profile.UserID != session.User.ID || profile.Phone != rbacCustomerPhone || profile.DisplayName != "Sam Example" {
		t.Fatalf("update changed protected fields or failed: %+v", profile)
	}

	w = e.get("/customer/profile", auth)
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &profile) != nil || profile.DisplayName != "Sam Example" {
		t.Fatalf("profile update was not persisted: %d %s", w.Code, w.Body.String())
	}
}

func TestCustomerProfileRequiresCustomerAuthenticationAndValidName(t *testing.T) {
	e := newRBACEnv(t)
	if w := e.get("/customer/profile", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated profile read: %d", w.Code)
	}
	vendor := e.login(t, rbacVendorPhone)
	if w := e.get("/customer/profile", "Bearer "+vendor.AccessToken); w.Code != http.StatusForbidden {
		t.Fatalf("vendor profile read: %d", w.Code)
	}

	customer := e.login(t, rbacCustomerPhone)
	for _, body := range []string{`{"display_name":" "}`, `{"display_name":"` + strings.Repeat("x", 101) + `"}`} {
		w := e.patch("/customer/profile", "Bearer "+customer.AccessToken, body)
		if w.Code != http.StatusBadRequest || decodeError(t, w).Code != exception.CodeBadRequest {
			t.Errorf("invalid profile update: %d %s", w.Code, w.Body.String())
		}
	}
}

func TestMissingOrInvalidAuthReturns401(t *testing.T) {
	e := newRBACEnv(t)

	expired := signToken(t, rbacSecret, rbacIssuer, enum.RoleAdmin, jwt.SigningMethodHS256, time.Now().Add(-time.Hour))
	wrongSecret := signToken(t, "another-secret-another-secret-12345", rbacIssuer, enum.RoleAdmin, jwt.SigningMethodHS256, time.Now().Add(time.Hour))
	wrongIssuer := signToken(t, rbacSecret, "someone-else", enum.RoleAdmin, jwt.SigningMethodHS256, time.Now().Add(time.Hour))
	hs512 := signToken(t, rbacSecret, rbacIssuer, enum.RoleAdmin, jwt.SigningMethodHS512, time.Now().Add(time.Hour))
	badRole := signToken(t, rbacSecret, rbacIssuer, enum.Role("root"), jwt.SigningMethodHS256, time.Now().Add(time.Hour))

	cases := map[string]string{
		"no header":     "",
		"wrong scheme":  "Basic abc",
		"empty bearer":  "Bearer ",
		"garbage":       "Bearer not.a.jwt",
		"expired":       "Bearer " + expired,
		"wrong secret":  "Bearer " + wrongSecret,
		"wrong issuer":  "Bearer " + wrongIssuer,
		"other alg":     "Bearer " + hs512,
		"unknown role":  "Bearer " + badRole,
		"token no type": expired,
	}
	for name, header := range cases {
		for _, path := range []string{"/auth/me", "/test/admin", "/test/customer"} {
			w := e.get(path, header)
			if w.Code != http.StatusUnauthorized || decodeError(t, w).Code != exception.CodeUnauthorized {
				t.Errorf("%s %s: got %d %s, want 401", name, path, w.Code, w.Body.String())
			}
		}
	}
}

func TestRoleAccessMatrix(t *testing.T) {
	e := newRBACEnv(t)
	tokens := map[enum.Role]string{
		enum.RoleCustomer: e.login(t, rbacCustomerPhone).AccessToken,
		enum.RoleVendor:   e.login(t, rbacVendorPhone).AccessToken,
		enum.RoleAdmin:    e.login(t, rbacAdminPhone).AccessToken,
	}
	routes := map[string]enum.Role{
		"/test/customer": enum.RoleCustomer,
		"/test/vendor":   enum.RoleVendor,
		"/test/admin":    enum.RoleAdmin,
	}

	for role, tok := range tokens {
		for path, allowed := range routes {
			w := e.get(path, "Bearer "+tok)
			if role == allowed {
				var body dto.MeResponse
				if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &body) != nil || body.Role != string(role) {
					t.Errorf("%s on %s: got %d %s, want 200", role, path, w.Code, w.Body.String())
				}
				continue
			}
			if w.Code != http.StatusForbidden || decodeError(t, w).Code != exception.CodeForbidden {
				t.Errorf("%s on %s: got %d %s, want 403", role, path, w.Code, w.Body.String())
			}
		}
		// every role may read its own identity
		if w := e.get("/auth/me", "Bearer "+tok); w.Code != http.StatusOK {
			t.Errorf("%s on /auth/me: got %d", role, w.Code)
		}
	}
}

func signToken(t *testing.T, secret, issuer string, role enum.Role, method jwt.SigningMethod, exp time.Time) string {
	t.Helper()
	s, err := jwt.NewWithClaims(method, jwt.MapClaims{
		"sub":  "user-1",
		"role": string(role),
		"iss":  issuer,
		"iat":  time.Now().Add(-2 * time.Hour).Unix(),
		"exp":  exp.Unix(),
	}).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// A token stays signed and unexpired after its account is suspended, deleted
// or loses its role; every request must still be refused from then on.
func TestExistingTokenRefusedAfterAccessChange(t *testing.T) {
	cases := []struct {
		name     string
		role     enum.Role
		status   model.UserStatus
		wantCode int
	}{
		{"suspended", enum.RoleCustomer, model.UserStatusSuspended, http.StatusForbidden},
		{"deleted", enum.RoleCustomer, model.UserStatusDeleted, http.StatusForbidden},
		{"role revoked", "", model.UserStatusActive, http.StatusForbidden},
	}
	for _, tc := range cases {
		e := newRBACEnv(t)
		session := e.login(t, rbacCustomerPhone)
		if w := e.get("/auth/me", "Bearer "+session.AccessToken); w.Code != http.StatusOK {
			t.Fatalf("%s: token must work before the change, got %d", tc.name, w.Code)
		}

		e.users.SetAccess(session.User.ID, tc.role, tc.status)
		for _, path := range []string{"/auth/me", "/test/customer"} {
			if w := e.get(path, "Bearer "+session.AccessToken); w.Code != tc.wantCode {
				t.Errorf("%s %s: got %d %s, want %d", tc.name, path, w.Code, w.Body.String(), tc.wantCode)
			}
		}
	}
}

// Role changes take effect on the next request: the stored role wins over
// the one in the token.
func TestStoredRoleOverridesTokenRole(t *testing.T) {
	e := newRBACEnv(t)
	session := e.login(t, rbacCustomerPhone)
	e.users.SetAccess(session.User.ID, enum.RoleAdmin, model.UserStatusActive)

	if w := e.get("/test/admin", "Bearer "+session.AccessToken); w.Code != http.StatusOK {
		t.Fatalf("promoted user on /test/admin: got %d", w.Code)
	}
	if w := e.get("/test/customer", "Bearer "+session.AccessToken); w.Code != http.StatusForbidden {
		t.Fatalf("promoted user keeps the old role: got %d", w.Code)
	}
}

// A validly signed token for a user that does not exist is not a session.
func TestTokenForUnknownUserReturns401(t *testing.T) {
	e := newRBACEnv(t)
	tok := signToken(t, rbacSecret, rbacIssuer, enum.RoleAdmin, jwt.SigningMethodHS256, time.Now().Add(time.Hour))
	if w := e.get("/test/admin", "Bearer "+tok); w.Code != http.StatusUnauthorized {
		t.Fatalf("got %d %s, want 401", w.Code, w.Body.String())
	}
}
