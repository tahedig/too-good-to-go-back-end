package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/pkg/logger"
)

// RateLimit allows at most limit requests per client IP per window, counted
// under scope (e.g. "auth") so different route groups have separate budgets.
//
// Client IPs come from c.ClientIP(), so TRUSTED_PROXIES must be set correctly
// behind a reverse proxy, otherwise every request shares the proxy's IP.
//
// If the limiter itself fails (e.g. Redis is down) the request is let through
// and the failure logged: a broken limiter should not take login offline.
func RateLimit(limiter serviceinterface.RateLimiter, scope string, limit int, window time.Duration, log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ok, retryAfter, err := limiter.Allow(ctx, scope+":"+c.ClientIP(), limit, window)
		if err != nil {
			logger.FromContext(ctx, log).Error("rate limiter unavailable, allowing request",
				"scope", scope, "error", err.Error())
			c.Next()
			return
		}
		if !ok {
			_ = c.Error(exception.NewTooManyRequests("too many requests; slow down", retryAfter))
			c.Abort()
			return
		}
		c.Next()
	}
}
