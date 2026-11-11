package router

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	handlerinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/handler/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware"
	middlewareinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware/interface"
)

// Handlers groups every HTTP handler. Add new feature handlers here.
type Handlers struct {
	Health handlerinterface.HealthHandler
	Auth   handlerinterface.AuthHandler
}

func New(
	cfg *bootstrap.Config,
	log *slog.Logger,
	errHandler middlewareinterface.ErrorHandler,
	limiter serviceinterface.RateLimiter,
	h Handlers,
) (*gin.Engine, error) {
	// Release mode always: gin's debug output is plain text and would break
	// the JSON-only log stream.
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.HandleMethodNotAllowed = true
	if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		return nil, fmt.Errorf("invalid TRUSTED_PROXIES: %w", err)
	}

	// Order matters: the logger sits outside Recover so panics are logged
	// with their final 500 status.
	r.Use(
		middleware.RequestID(),
		middleware.RequestLogger(log),
		errHandler.Recover(),
		errHandler.Handle(),
		middleware.SecurityHeaders(),
	)
	// Zero values (e.g. a bare Config in tests) disable the limits; LoadConfig
	// always fills safe defaults.
	if cfg.HTTPMaxBodyBytes > 0 {
		r.Use(middleware.BodyLimit(cfg.HTTPMaxBodyBytes))
	}
	r.NoRoute(errHandler.NotFound())
	r.NoMethod(errHandler.MethodNotAllowed())

	r.GET(bootstrap.HealthPath, h.Health.Health)
	r.HEAD(bootstrap.HealthPath, h.Health.Health)

	auth := r.Group("/auth/otp")
	if limiter != nil && cfg.AuthRateLimit > 0 && cfg.AuthRateWindow > 0 {
		// These endpoints send SMS and accept guesses, so they are the
		// first target for abuse.
		auth.Use(middleware.RateLimit(limiter, "auth", cfg.AuthRateLimit, cfg.AuthRateWindow, log))
	}
	auth.POST("/request", h.Auth.RequestOTP)
	auth.POST("/verify", h.Auth.VerifyOTP)

	return r, nil
}
