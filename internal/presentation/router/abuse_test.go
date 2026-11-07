package router_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	"github.com/tahedig/too-good-to-go/backend/internal/application/service"
	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	redisrepo "github.com/tahedig/too-good-to-go/backend/internal/infrastructure/repository/redis"
)

func TestOTPResendCooldown(t *testing.T) {
	e := newAuthEnvWith(t, &bootstrap.Config{},
		service.OTPConfig{Length: 6, TTL: time.Minute, MaxAttempts: 3, ResendCooldown: time.Minute})

	code, w := e.request(t, testPhone)
	if w.Code != http.StatusOK {
		t.Fatalf("first request: %d %s", w.Code, w.Body.String())
	}

	w = e.post("/auth/otp/request", `{"phone":"09123456789"}`) // same number, other format
	if w.Code != http.StatusTooManyRequests || decodeError(t, w).Code != exception.CodeOTPResendTooSoon {
		t.Fatalf("resend within cooldown: %d %s", w.Code, w.Body.String())
	}
	if s, err := strconv.Atoi(w.Header().Get("Retry-After")); err != nil || s < 1 || s > 60 {
		t.Fatalf("Retry-After = %q, want 1..60", w.Header().Get("Retry-After"))
	}
	if n := len(e.sms.Sent()); n != 1 {
		t.Fatalf("blocked request must not send an sms, got %d messages", n)
	}

	// The first code is still valid; the blocked request did not replace it.
	if w := e.verify(testPhone, code); w.Code != http.StatusOK {
		t.Fatalf("original code should still verify: %d %s", w.Code, w.Body.String())
	}
}

func TestMalformedCodeDoesNotConsumeAttempts(t *testing.T) {
	e := newAuthEnv(t, time.Minute, 1)
	code, _ := e.request(t, testPhone)

	// The last entry is six Arabic-Indic digits, which must not pass as ASCII digits.
	for _, bad := range []string{"", "12", "1234567", "12a456", "١٢٣٤٥٦"} {
		w := e.verify(testPhone, bad)
		if w.Code != http.StatusBadRequest || decodeError(t, w).Code != exception.CodeBadRequest {
			t.Fatalf("code %q: %d %s", bad, w.Code, w.Body.String())
		}
	}

	// With MaxAttempts=1 this only works if none of the above counted.
	if w := e.verify(testPhone, " "+code+" "); w.Code != http.StatusOK {
		t.Fatalf("correct code after malformed ones: %d %s", w.Code, w.Body.String())
	}
}

func TestAuthRateLimitPerIP(t *testing.T) {
	e := newAuthEnvWith(t, &bootstrap.Config{AuthRateLimit: 2, AuthRateWindow: time.Minute},
		service.OTPConfig{Length: 6, TTL: time.Minute, MaxAttempts: 3})

	for i := 0; i < 2; i++ {
		if w := e.post("/auth/otp/request", `{"phone":"+98912000000`+strconv.Itoa(i)+`"}`); w.Code != http.StatusOK {
			t.Fatalf("request %d: %d %s", i+1, w.Code, w.Body.String())
		}
	}
	w := e.verify(testPhone, "123456") // verify shares the auth budget
	if w.Code != http.StatusTooManyRequests || decodeError(t, w).Code != exception.CodeTooManyRequests {
		t.Fatalf("over the limit: %d %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("rate limited response must carry Retry-After")
	}

	// Other routes are not limited.
	if w := e.get("/health"); w.Code != http.StatusOK {
		t.Fatalf("/health should not be rate limited: %d", w.Code)
	}
}

func TestRequestBodyLimit(t *testing.T) {
	e := newAuthEnvWith(t, &bootstrap.Config{HTTPMaxBodyBytes: 1024},
		service.OTPConfig{Length: 6, TTL: time.Minute, MaxAttempts: 3})

	big := `{"phone":"` + testPhone + `","pad":"` + strings.Repeat("x", 2048) + `"}`
	w := e.post("/auth/otp/request", big)
	if w.Code != http.StatusRequestEntityTooLarge || decodeError(t, w).Code != exception.CodePayloadTooLarge {
		t.Fatalf("oversized body: %d %s", w.Code, w.Body.String())
	}
	if len(e.sms.Sent()) != 0 {
		t.Fatal("no sms should be sent for a rejected body")
	}
}

func TestSecurityHeaders(t *testing.T) {
	e := newAuthEnv(t, time.Minute, 3)
	for _, path := range []string{"/health", "/nope"} {
		w := e.get(path)
		for header, want := range map[string]string{
			"X-Content-Type-Options": "nosniff",
			"X-Frame-Options":        "DENY",
			"Referrer-Policy":        "no-referrer",
		} {
			if got := w.Header().Get(header); got != want {
				t.Errorf("%s: %s = %q, want %q", path, header, got, want)
			}
		}
	}
}

// The whole login flow must behave the same with the Redis-backed stores.
func TestAuthFlowWithRedisStores(t *testing.T) {
	mr := miniredis.RunT(t)
	client, err := redisrepo.NewClient(context.Background(), "redis://"+mr.Addr())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })

	e := newAuthEnvWithStores(t,
		&bootstrap.Config{AuthRateLimit: 100, AuthRateWindow: time.Minute},
		service.OTPConfig{Length: 6, TTL: time.Minute, MaxAttempts: 2, ResendCooldown: time.Minute},
		redisrepo.NewOTPRepository(client, "t:"), redisrepo.NewRateLimiter(client, "t:"))

	code, w := e.request(t, testPhone)
	if w.Code != http.StatusOK {
		t.Fatalf("request: %d %s", w.Code, w.Body.String())
	}
	if w := e.post("/auth/otp/request", `{"phone":"`+testPhone+`"}`); w.Code != http.StatusTooManyRequests {
		t.Fatalf("cooldown not enforced through redis: %d", w.Code)
	}
	if w := e.verify(testPhone, wrongCode(code)); w.Code != http.StatusBadRequest ||
		decodeError(t, w).Code != exception.CodeOTPInvalid {
		t.Fatalf("wrong code: %d %s", w.Code, w.Body.String())
	}
	if resp := e.verify(testPhone, code); resp.Code != http.StatusOK {
		t.Fatalf("verify: %d %s", resp.Code, resp.Body.String())
	}
	if w := e.verify(testPhone, code); w.Code != http.StatusBadRequest ||
		decodeError(t, w).Code != exception.CodeOTPExpired {
		t.Fatalf("reuse should fail: %d %s", w.Code, w.Body.String())
	}
	if !mr.Exists("t:ratelimit:auth:192.0.2.1") {
		t.Fatalf("rate limit counter should live in redis, keys: %v", mr.Keys())
	}
}

func TestExposeCodeReturnsDebugCode(t *testing.T) {
	e := newAuthEnvWith(t, &bootstrap.Config{},
		service.OTPConfig{Length: 6, TTL: time.Minute, MaxAttempts: 3, ExposeCode: true})

	w := e.post("/auth/otp/request", `{"phone":"`+testPhone+`"}`)
	var resp dto.RequestOTPResponse
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &resp) != nil || resp.DebugCode == "" {
		t.Fatalf("debug_code missing: %d %s", w.Code, w.Body.String())
	}
	if w := e.verify(testPhone, resp.DebugCode); w.Code != http.StatusOK {
		t.Fatalf("debug_code should verify: %d %s", w.Code, w.Body.String())
	}

	// Default config never exposes it.
	plain := newAuthEnv(t, time.Minute, 3)
	if w := plain.post("/auth/otp/request", `{"phone":"`+testPhone+`"}`); strings.Contains(w.Body.String(), "debug_code") {
		t.Fatalf("debug_code must be absent by default: %s", w.Body.String())
	}
}
