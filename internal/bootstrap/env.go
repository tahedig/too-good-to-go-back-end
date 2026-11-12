package bootstrap

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/tahedig/too-good-to-go/backend/pkg/validation"
)

type Config struct {
	AppEnv string

	HTTPHost            string
	HTTPPort            int
	HTTPReadTimeout     time.Duration
	HTTPWriteTimeout    time.Duration
	HTTPIdleTimeout     time.Duration
	HTTPShutdownTimeout time.Duration
	// HTTPMaxBodyBytes caps request bodies; larger ones get 413.
	HTTPMaxBodyBytes int64

	HealthCheckTimeout time.Duration

	LogLevel string

	OTPLength      int
	OTPTTL         time.Duration
	OTPMaxAttempts int
	// OTPResendCooldown is the minimum gap between two codes for one phone.
	OTPResendCooldown time.Duration
	// OTPExposeCode returns the generated code in the /auth/otp/request
	// response so the flow can be tested end to end while SMS delivery is
	// faked. Refused when APP_ENV is production.
	OTPExposeCode bool
	// SMSProvider selects the SMS implementation; only "fake" exists for now.
	SMSProvider string

	// AuthRateLimit requests per AuthRateWindow are allowed per client IP on
	// the /auth/otp endpoints.
	AuthRateLimit  int
	AuthRateWindow time.Duration

	// RedisURL (redis://[:password@]host:port/db) stores OTPs and rate-limit
	// counters. Required outside development; when empty in development
	// they are kept in process memory.
	RedisURL string
	// RedisKeyPrefix namespaces every key, so environments can share a Redis.
	RedisKeyPrefix string

	// DatabaseURL (postgres://user:pass@host:port/db?sslmode=...) stores users
	// and their roles. Required outside development; when empty in
	// development users are kept in process memory.
	DatabaseURL string

	// JWTSecret signs access tokens (HS256). Required outside development;
	// in development an ephemeral random secret is generated when unset.
	JWTSecret    string
	JWTIssuer    string
	JWTAccessTTL time.Duration

	// AdminPhones and VendorPhones (E.164) get that role when they log in for
	// the first time; every other new user is a customer.
	AdminPhones  []string
	VendorPhones []string

	// TrustedProxies is the list of proxy IPs/CIDRs allowed to set
	// X-Forwarded-For. Empty means no proxy is trusted.
	TrustedProxies []string
}

// Addr returns the host:port the HTTP server listens on.
func (c *Config) Addr() string {
	return net.JoinHostPort(c.HTTPHost, strconv.Itoa(c.HTTPPort))
}

func (c *Config) IsProduction() bool { return c.AppEnv == EnvProduction }

// LoadConfig reads configuration from the environment. If a .env file exists in
// the working directory it is loaded first, but real environment variables
// always win over values in the file.
func LoadConfig() (*Config, error) {
	if err := godotenv.Load(".env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("load .env: %w", err)
	}

	p := &envParser{}
	cfg := &Config{
		AppEnv:              p.oneOf("APP_ENV", EnvDevelopment, EnvDevelopment, EnvStaging, EnvProduction),
		HTTPHost:            p.string("HTTP_HOST", ""),
		HTTPPort:            p.port("HTTP_PORT", 8080),
		HTTPReadTimeout:     p.duration("HTTP_READ_TIMEOUT", 10*time.Second),
		HTTPWriteTimeout:    p.duration("HTTP_WRITE_TIMEOUT", 15*time.Second),
		HTTPIdleTimeout:     p.duration("HTTP_IDLE_TIMEOUT", 60*time.Second),
		HTTPShutdownTimeout: p.duration("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second),
		HTTPMaxBodyBytes:    int64(p.intRange("HTTP_MAX_BODY_BYTES", 1<<20, 1<<10, 64<<20)),
		HealthCheckTimeout:  p.duration("HEALTH_CHECK_TIMEOUT", 2*time.Second),
		LogLevel:            p.oneOf("LOG_LEVEL", "info", "debug", "info", "warn", "error"),
		OTPLength:           p.intRange("OTP_LENGTH", 6, 4, 8),
		OTPTTL:              p.duration("OTP_TTL", 5*time.Minute),
		OTPMaxAttempts:      p.intRange("OTP_MAX_ATTEMPTS", 5, 1, 20),
		OTPResendCooldown:   p.duration("OTP_RESEND_COOLDOWN", time.Minute),
		OTPExposeCode:       p.bool("OTP_EXPOSE_CODE", false),
		SMSProvider:         p.oneOf("SMS_PROVIDER", SMSProviderFake, SMSProviderFake),
		AuthRateLimit:       p.intRange("AUTH_RATE_LIMIT", 20, 1, 10000),
		AuthRateWindow:      p.duration("AUTH_RATE_WINDOW", time.Minute),
		RedisURL:            p.string("REDIS_URL", ""),
		RedisKeyPrefix:      p.string("REDIS_KEY_PREFIX", "tgtg:"),
		DatabaseURL:         p.string("DATABASE_URL", ""),
		JWTSecret:           p.string("JWT_SECRET", ""),
		JWTIssuer:           p.string("JWT_ISSUER", AppName),
		JWTAccessTTL:        p.duration("JWT_ACCESS_TTL", time.Hour),
		AdminPhones:         p.phones("ADMIN_PHONES"),
		VendorPhones:        p.phones("VENDOR_PHONES"),
		TrustedProxies:      p.list("TRUSTED_PROXIES"),
	}
	p.jwtSecret(cfg)
	if cfg.RedisURL == "" && cfg.AppEnv != EnvDevelopment {
		p.errs = append(p.errs, errors.New("REDIS_URL is required when APP_ENV is not development"))
	}
	if cfg.DatabaseURL == "" && cfg.AppEnv != EnvDevelopment {
		p.errs = append(p.errs, errors.New("DATABASE_URL is required when APP_ENV is not development"))
	}
	if cfg.OTPExposeCode && cfg.IsProduction() {
		p.errs = append(p.errs, errors.New("OTP_EXPOSE_CODE must not be enabled in production"))
	}

	if err := errors.Join(p.errs...); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	return cfg, nil
}

// envParser collects every problem so the developer sees all invalid
// variables at once instead of fixing them one by one.
type envParser struct {
	errs []error
}

// string returns def when the variable is unset or blank, so the empty
// placeholders copied from .env.example mean "use the default".
func (p *envParser) string(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func (p *envParser) oneOf(key, def string, allowed ...string) string {
	v := strings.ToLower(p.string(key, def))
	if v == "" {
		return def
	}
	for _, a := range allowed {
		if v == a {
			return v
		}
	}
	p.errs = append(p.errs, fmt.Errorf("%s=%q must be one of %v", key, v, allowed))
	return def
}

func (p *envParser) bool(key string, def bool) bool {
	raw := p.string(key, "")
	if raw == "" {
		return def
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		p.errs = append(p.errs, fmt.Errorf("%s=%q must be true or false", key, raw))
		return def
	}
	return b
}

func (p *envParser) port(key string, def int) int {
	raw := p.string(key, "")
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 65535 {
		p.errs = append(p.errs, fmt.Errorf("%s=%q must be a port number between 1 and 65535", key, raw))
		return def
	}
	return n
}

// phones parses a comma separated list of phone numbers into E.164.
func (p *envParser) phones(key string) []string {
	var out []string
	for _, raw := range p.list(key) {
		n, err := validation.NormalizePhone(raw)
		if err != nil {
			p.errs = append(p.errs, fmt.Errorf("%s contains an invalid phone number %q", key, raw))
			continue
		}
		out = append(out, n)
	}
	return out
}

const minJWTSecretLen = 32

func (p *envParser) jwtSecret(cfg *Config) {
	if cfg.JWTSecret == "" {
		if cfg.AppEnv != EnvDevelopment {
			p.errs = append(p.errs, errors.New("JWT_SECRET is required when APP_ENV is not development"))
			return
		}
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			p.errs = append(p.errs, fmt.Errorf("generate development JWT secret: %w", err))
			return
		}
		cfg.JWTSecret = hex.EncodeToString(b) // tokens stop working on restart
		return
	}
	if len(cfg.JWTSecret) < minJWTSecretLen {
		p.errs = append(p.errs, fmt.Errorf("JWT_SECRET must be at least %d characters", minJWTSecretLen))
	}
}

func (p *envParser) intRange(key string, def, lo, hi int) int {
	raw := p.string(key, "")
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < lo || n > hi {
		p.errs = append(p.errs, fmt.Errorf("%s=%q must be a number between %d and %d", key, raw, lo, hi))
		return def
	}
	return n
}

func (p *envParser) duration(key string, def time.Duration) time.Duration {
	raw := p.string(key, "")
	if raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		p.errs = append(p.errs, fmt.Errorf("%s=%q must be a positive duration such as 5s or 1m", key, raw))
		return def
	}
	return d
}

func (p *envParser) list(key string) []string {
	raw := p.string(key, "")
	if raw == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if s := strings.TrimSpace(part); s != "" {
			out = append(out, s)
		}
	}
	return out
}
