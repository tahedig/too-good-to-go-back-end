package router_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	"github.com/tahedig/too-good-to-go/backend/internal/application/service"
	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/repository/memory"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/sms"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/token"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/handler"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/router"
	"github.com/tahedig/too-good-to-go/backend/pkg/logger"
)

const (
	testPhone       = "+989123456789"
	testVendorPhone = "+989121111111"
)

var codeRe = regexp.MustCompile(`\b\d{6}\b`)

type authEnv struct {
	r    *gin.Engine
	sms  *sms.FakeSender
	logs *bytes.Buffer
}

func newAuthEnv(t *testing.T, ttl time.Duration, maxAttempts int) *authEnv {
	t.Helper()
	return newAuthEnvWith(t, &bootstrap.Config{}, service.OTPConfig{Length: 6, TTL: ttl, MaxAttempts: maxAttempts})
}

// newAuthEnvWith lets a test enable router limits (cfg) or the resend cooldown (otpCfg).
func newAuthEnvWith(t *testing.T, cfg *bootstrap.Config, otpCfg service.OTPConfig) *authEnv {
	t.Helper()
	return newAuthEnvWithStores(t, cfg, otpCfg, memory.NewOTPRepository(), memory.NewRateLimiter())
}

// newAuthEnvWithStores also swaps the OTP store and rate limiter (e.g. for Redis).
func newAuthEnvWithStores(t *testing.T, cfg *bootstrap.Config, otpCfg service.OTPConfig,
	otps repository.OTPRepository, limiter serviceinterface.RateLimiter) *authEnv {
	t.Helper()
	logs := &bytes.Buffer{}
	log := logger.New("debug", logs)
	sender := sms.NewFakeSender(log, false) // production-style: body never logged
	auth := service.NewAuthService(otps, sender, otpCfg, log)
	tokens := token.NewJWTManager(rbacSecret, rbacIssuer, time.Hour)
	sessions := service.NewSessionService(memory.NewUserRepository(), tokens,
		map[string]enum.Role{testVendorPhone: enum.RoleVendor}, log)
	health := service.NewHealthService("test", time.Second, log)

	r, err := router.New(cfg, log, middleware.NewErrorHandler(log), middleware.NewAuthMiddleware(tokens), limiter, router.Handlers{
		Health:    handler.NewHealthHandler(health),
		Auth:      handler.NewAuthHandler(auth, sessions),
		Protected: handler.NewProtectedHandler(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return &authEnv{r: r, sms: sender, logs: logs}
}

func (e *authEnv) post(path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	return w
}

func (e *authEnv) request(t *testing.T, phone string) (code string, w *httptest.ResponseRecorder) {
	t.Helper()
	w = e.post("/auth/otp/request", `{"phone":"`+phone+`"}`)
	if w.Code != http.StatusOK {
		return "", w
	}
	msgs := e.sms.Sent()
	code = codeRe.FindString(msgs[len(msgs)-1].Body)
	if code == "" {
		t.Fatalf("no code in sms: %q", msgs[len(msgs)-1].Body)
	}
	return code, w
}

func (e *authEnv) verify(phone, code string) *httptest.ResponseRecorder {
	return e.post("/auth/otp/verify", `{"phone":"`+phone+`","code":"`+code+`"}`)
}

func wrongCode(code string) string {
	if code == "000000" {
		return "000001"
	}
	return "000000"
}

func TestOTPRequestAndVerifySuccess(t *testing.T) {
	e := newAuthEnv(t, time.Minute, 3)

	code, w := e.request(t, "09123456789") // local format is normalized
	if w.Code != http.StatusOK {
		t.Fatalf("request: %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), code) {
		t.Fatal("OTP must not appear in the API response")
	}
	if got := e.sms.Sent()[0].Phone; got != testPhone {
		t.Fatalf("sms sent to %q, want %q", got, testPhone)
	}

	w = e.verify(testPhone, code)
	if w.Code != http.StatusOK {
		t.Fatalf("verify: %d %s", w.Code, w.Body.String())
	}
	var resp dto.VerifyOTPResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || !resp.Verified || resp.Phone != testPhone {
		t.Fatalf("unexpected verify response: %s", w.Body.String())
	}
	if resp.AccessToken == "" || resp.TokenType != "Bearer" || resp.ExpiresIn != 3600 {
		t.Fatalf("verify must return a session: %s", w.Body.String())
	}
	if resp.User.ID == "" || resp.User.Phone != testPhone || resp.User.Role != string(enum.RoleCustomer) {
		t.Fatalf("unexpected user in verify response: %+v", resp.User)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("token response must not be cached, got Cache-Control %q", cc)
	}

	if bytes.Contains(e.logs.Bytes(), []byte(code)) {
		t.Fatal("OTP leaked into logs")
	}
}

func TestOTPIsSingleUse(t *testing.T) {
	e := newAuthEnv(t, time.Minute, 3)
	code, _ := e.request(t, testPhone)

	if w := e.verify(testPhone, code); w.Code != http.StatusOK {
		t.Fatalf("first verify: %d", w.Code)
	}
	w := e.verify(testPhone, code)
	if w.Code != http.StatusBadRequest || decodeError(t, w).Code != exception.CodeOTPExpired {
		t.Fatalf("reuse should fail with %s, got %d %s", exception.CodeOTPExpired, w.Code, w.Body.String())
	}
}

func TestOTPExpires(t *testing.T) {
	e := newAuthEnv(t, 50*time.Millisecond, 3)
	code, _ := e.request(t, testPhone)
	time.Sleep(80 * time.Millisecond)

	w := e.verify(testPhone, code)
	if w.Code != http.StatusBadRequest || decodeError(t, w).Code != exception.CodeOTPExpired {
		t.Fatalf("expired code: %d %s", w.Code, w.Body.String())
	}
}

func TestOTPWrongCodeAndAttemptLimit(t *testing.T) {
	e := newAuthEnv(t, time.Minute, 2)
	code, _ := e.request(t, testPhone)

	for i := 0; i < 2; i++ {
		w := e.verify(testPhone, wrongCode(code))
		if w.Code != http.StatusBadRequest || decodeError(t, w).Code != exception.CodeOTPInvalid {
			t.Fatalf("attempt %d: %d %s", i+1, w.Code, w.Body.String())
		}
	}

	// Limit reached: even the correct code is rejected.
	w := e.verify(testPhone, code)
	if w.Code != http.StatusTooManyRequests || decodeError(t, w).Code != exception.CodeOTPAttemptsLimit {
		t.Fatalf("after limit: %d %s", w.Code, w.Body.String())
	}

	// A fresh OTP resets the counter.
	code2, _ := e.request(t, testPhone)
	if w := e.verify(testPhone, code2); w.Code != http.StatusOK {
		t.Fatalf("new otp should verify: %d %s", w.Code, w.Body.String())
	}
}

func TestOTPInvalidInput(t *testing.T) {
	e := newAuthEnv(t, time.Minute, 3)

	for _, phone := range []string{"", "abc", "12345", "+0123456789", "0912"} {
		w := e.post("/auth/otp/request", `{"phone":"`+phone+`"}`)
		if w.Code != http.StatusBadRequest || decodeError(t, w).Code != exception.CodeInvalidPhone {
			t.Errorf("phone %q: %d %s", phone, w.Code, w.Body.String())
		}
	}
	if len(e.sms.Sent()) != 0 {
		t.Fatal("no sms should be sent for invalid phones")
	}

	if w := e.post("/auth/otp/request", `not json`); w.Code != http.StatusBadRequest ||
		decodeError(t, w).Code != exception.CodeBadRequest {
		t.Errorf("malformed body: %d %s", w.Code, w.Body.String())
	}

	// Verify with no OTP requested, and with a bad phone.
	if w := e.verify(testPhone, "123456"); w.Code != http.StatusBadRequest ||
		decodeError(t, w).Code != exception.CodeOTPExpired {
		t.Errorf("no otp: %d %s", w.Code, w.Body.String())
	}
	if w := e.verify("nope", "123456"); w.Code != http.StatusBadRequest ||
		decodeError(t, w).Code != exception.CodeInvalidPhone {
		t.Errorf("bad phone on verify: %d %s", w.Code, w.Body.String())
	}
}

func (e *authEnv) get(path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

func (e *authEnv) getAuth(path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	return w
}

// login runs the full OTP flow and returns the verify response.
func (e *authEnv) login(t *testing.T, phone string) dto.VerifyOTPResponse {
	t.Helper()
	code, w := e.request(t, phone)
	if w.Code != http.StatusOK {
		t.Fatalf("request: %d %s", w.Code, w.Body.String())
	}
	w = e.verify(phone, code)
	var resp dto.VerifyOTPResponse
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &resp) != nil {
		t.Fatalf("verify: %d %s", w.Code, w.Body.String())
	}
	return resp
}

// The token from an OTP login must open the protected routes for its role.
func TestOTPLoginTokenAuthenticates(t *testing.T) {
	e := newAuthEnv(t, time.Minute, 3)
	session := e.login(t, testPhone)

	w := e.getAuth("/auth/me", session.AccessToken)
	var me dto.MeResponse
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &me) != nil {
		t.Fatalf("/auth/me: %d %s", w.Code, w.Body.String())
	}
	if me.UserID != session.User.ID || me.Role != string(enum.RoleCustomer) {
		t.Fatalf("token identity %+v does not match login user %+v", me, session.User)
	}
	if w := e.getAuth("/test/customer", session.AccessToken); w.Code != http.StatusOK {
		t.Fatalf("customer route: %d", w.Code)
	}
	if w := e.getAuth("/test/vendor", session.AccessToken); w.Code != http.StatusForbidden {
		t.Fatalf("customer on vendor route: got %d, want 403", w.Code)
	}
}

// Logging in twice with the same phone is the same user; phones listed as
// vendor get the vendor role.
func TestOTPLoginReusesUserAndAssignsRole(t *testing.T) {
	e := newAuthEnv(t, time.Minute, 3)

	first := e.login(t, testPhone)
	second := e.login(t, "09123456789") // same phone, local format
	if first.User.ID != second.User.ID {
		t.Fatalf("second login created a new user: %s != %s", first.User.ID, second.User.ID)
	}

	vendor := e.login(t, testVendorPhone)
	if vendor.User.Role != string(enum.RoleVendor) || vendor.User.ID == first.User.ID {
		t.Fatalf("unexpected vendor user: %+v", vendor.User)
	}
	if w := e.getAuth("/test/vendor", vendor.AccessToken); w.Code != http.StatusOK {
		t.Fatalf("vendor route with vendor token: %d %s", w.Code, w.Body.String())
	}
}

// A failed verify must never return a token.
func TestFailedVerifyIssuesNoToken(t *testing.T) {
	e := newAuthEnv(t, time.Minute, 3)
	code, _ := e.request(t, testPhone)

	w := e.verify(testPhone, wrongCode(code))
	if w.Code == http.StatusOK || strings.Contains(w.Body.String(), "access_token") {
		t.Fatalf("wrong code returned a session: %d %s", w.Code, w.Body.String())
	}
}
