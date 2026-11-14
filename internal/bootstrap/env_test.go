package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigDefaults(t *testing.T) {
	t.Chdir(t.TempDir()) // no .env file here
	for _, k := range []string{"APP_ENV", "HTTP_PORT", "LOG_LEVEL", "HTTP_READ_TIMEOUT", "TRUSTED_PROXIES",
		"OTP_RESEND_COOLDOWN", "HTTP_MAX_BODY_BYTES", "AUTH_RATE_LIMIT", "AUTH_RATE_WINDOW", "REDIS_KEY_PREFIX"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AppEnv != EnvDevelopment || cfg.HTTPPort != 8080 || cfg.LogLevel != "info" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.HTTPReadTimeout != 10*time.Second {
		t.Fatalf("unexpected read timeout: %v", cfg.HTTPReadTimeout)
	}
	if cfg.OTPResendCooldown != time.Minute || cfg.HTTPMaxBodyBytes != 1<<20 ||
		cfg.AuthRateLimit != 20 || cfg.AuthRateWindow != time.Minute || cfg.RedisKeyPrefix != "tgtg:" {
		t.Fatalf("unexpected OTP/abuse-protection defaults: %+v", cfg)
	}
}

func TestOTPAndRedisRules(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, k := range []string{"APP_ENV", "REDIS_URL", "OTP_EXPOSE_CODE"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32)) // production also requires it
	t.Setenv("DATABASE_URL", "postgres://app:app@localhost:5432/tgtg")

	if _, err := LoadConfig(); err != nil { // development: memory fallback is fine
		t.Fatalf("development without REDIS_URL should load: %v", err)
	}

	t.Setenv("APP_ENV", "production")
	if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "REDIS_URL") {
		t.Fatalf("production without REDIS_URL must fail, got %v", err)
	}
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	if _, err := LoadConfig(); err != nil {
		t.Fatalf("production with REDIS_URL should load: %v", err)
	}

	t.Setenv("OTP_EXPOSE_CODE", "true")
	if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "OTP_EXPOSE_CODE") {
		t.Fatalf("OTP_EXPOSE_CODE in production must fail, got %v", err)
	}
	t.Setenv("APP_ENV", "staging")
	if cfg, err := LoadConfig(); err != nil || !cfg.OTPExposeCode {
		t.Fatalf("OTP_EXPOSE_CODE should be allowed in staging, err=%v", err)
	}
	t.Setenv("OTP_EXPOSE_CODE", "maybe")
	if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "OTP_EXPOSE_CODE") {
		t.Fatalf("non-boolean OTP_EXPOSE_CODE must fail, got %v", err)
	}
}

func TestJWTSecretRules(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, k := range []string{"APP_ENV", "JWT_SECRET", "JWT_ISSUER", "ADMIN_PHONES"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	t.Setenv("REDIS_URL", "redis://localhost:6379/0") // production also requires it
	t.Setenv("DATABASE_URL", "postgres://app:app@localhost:5432/tgtg")

	cfg, err := LoadConfig() // development: an ephemeral secret is generated
	if err != nil || len(cfg.JWTSecret) < minJWTSecretLen {
		t.Fatalf("development should get a generated secret, err=%v", err)
	}

	t.Setenv("APP_ENV", "production")
	if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("production without JWT_SECRET must fail, got %v", err)
	}

	t.Setenv("JWT_SECRET", "too-short")
	if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("short JWT_SECRET must fail, got %v", err)
	}

	t.Setenv("JWT_SECRET", strings.Repeat("k", 40))
	t.Setenv("ADMIN_PHONES", "09121234567, +989129999999")
	cfg, err = LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.AdminPhones) != 2 || cfg.AdminPhones[0] != "+989121234567" {
		t.Fatalf("admin phones not normalized: %v", cfg.AdminPhones)
	}

	t.Setenv("ADMIN_PHONES", "nope")
	if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "ADMIN_PHONES") {
		t.Fatalf("invalid ADMIN_PHONES must fail, got %v", err)
	}
	t.Setenv("ADMIN_PHONES", "")

	t.Setenv("JWT_ISSUER", "  ") // blank placeholder from .env.example
	if cfg, err := LoadConfig(); err != nil || cfg.JWTIssuer != AppName {
		t.Fatalf("blank JWT_ISSUER should use the default, got %q err=%v", cfg.JWTIssuer, err)
	}
}

func TestLoadConfigReportsAllInvalidValues(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("APP_ENV", "prod")
	t.Setenv("HTTP_PORT", "99999")
	t.Setenv("HTTP_READ_TIMEOUT", "ten seconds")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, key := range []string{"APP_ENV", "HTTP_PORT", "HTTP_READ_TIMEOUT"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error should mention %s, got: %v", key, err)
		}
	}
}

func TestLoadConfigUsesDefaultsForEmptyDotEnvValues(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	for _, key := range []string{"APP_ENV", "LOG_LEVEL"} {
		value, exists := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if exists {
				_ = os.Setenv(key, value)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("APP_ENV=\nLOG_LEVEL=\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AppEnv != EnvDevelopment || cfg.LogLevel != "info" {
		t.Fatalf("empty values should use defaults, got env=%q log level=%q", cfg.AppEnv, cfg.LogLevel)
	}
}

func TestDotEnvDoesNotOverrideRealEnv(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	content := "HTTP_PORT=9000\nLOG_LEVEL=\"debug\" \n# comment\nexport APP_ENV=staging # inline\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HTTP_PORT", "7000") // real env wins
	// .env sets APP_ENV=staging, which requires all of these:
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("DATABASE_URL", "postgres://app:app@localhost:5432/tgtg")
	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("APP_ENV")
	t.Cleanup(func() { os.Unsetenv("LOG_LEVEL"); os.Unsetenv("APP_ENV") })

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPPort != 7000 {
		t.Errorf("real env should win, got port %d", cfg.HTTPPort)
	}
	if cfg.LogLevel != "debug" || cfg.AppEnv != EnvStaging {
		t.Errorf(".env values not applied: %+v", cfg)
	}
}

func TestLoadConfigReportsMalformedDotEnv(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("not-an-assignment\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig()
	if err == nil || !strings.Contains(err.Error(), ".env") {
		t.Fatalf("expected a .env parsing error, got %v", err)
	}
}

func TestDatabaseURLRules(t *testing.T) {
	t.Chdir(t.TempDir()) // no .env
	for _, k := range []string{"APP_ENV", "DATABASE_URL"} {
		t.Setenv(k, "")
	}
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")

	if _, err := LoadConfig(); err != nil {
		t.Fatalf("development without DATABASE_URL should load: %v", err)
	}
	t.Setenv("APP_ENV", "production")
	if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("production without DATABASE_URL must fail, got %v", err)
	}
	t.Setenv("DATABASE_URL", "postgres://app:app@localhost:5432/tgtg")
	cfg, err := LoadConfig()
	if err != nil || cfg.DatabaseURL != "postgres://app:app@localhost:5432/tgtg" {
		t.Fatalf("production with DATABASE_URL should load: %v", err)
	}
}

func TestCORSOriginRules(t *testing.T) {
	t.Chdir(t.TempDir()) // no .env
	for _, k := range []string{"APP_ENV", "CORS_ALLOWED_ORIGINS"} {
		t.Setenv(k, "")
	}
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("DATABASE_URL", "postgres://app:app@localhost:5432/tgtg")

	cfg, err := LoadConfig()
	if err != nil || len(cfg.CORSAllowedOrigins) != 1 || cfg.CORSAllowedOrigins[0] != DevFrontendOrigin {
		t.Fatalf("development defaults to the Vite origin: %v %v", cfg.CORSAllowedOrigins, err)
	}
	t.Setenv("APP_ENV", "production")
	if cfg, err := LoadConfig(); err != nil || len(cfg.CORSAllowedOrigins) != 0 {
		t.Fatalf("production allows no origin by default: %v %v", cfg.CORSAllowedOrigins, err)
	}

	t.Setenv("CORS_ALLOWED_ORIGINS", "https://admin.example.com/, http://localhost:5173")
	cfg, err = LoadConfig()
	if err != nil || strings.Join(cfg.CORSAllowedOrigins, " ") != "https://admin.example.com http://localhost:5173" {
		t.Fatalf("explicit origins: %v %v", cfg.CORSAllowedOrigins, err)
	}
	for _, bad := range []string{"*", "admin.example.com", "ftp://x.example", "https://x.example/path", "https://u:p@x.example"} {
		t.Setenv("CORS_ALLOWED_ORIGINS", bad)
		if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "CORS_ALLOWED_ORIGINS") {
			t.Errorf("%q must be refused, got %v", bad, err)
		}
	}
}
