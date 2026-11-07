// Package injector assembles the application dependency graph. The providers
// live here; wire.go lists them and `go generate ./internal/injector`
// regenerates wire_gen.go.
package injector

import (
	"context"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tahedig/too-good-to-go/backend/internal/application/service"
	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/repository/memory"
	redisrepo "github.com/tahedig/too-good-to-go/backend/internal/infrastructure/repository/redis"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/sms"
	handlerinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/handler/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/router"
)

type App struct {
	Router *gin.Engine
	Health serviceinterface.HealthService
}

func newApp(r *gin.Engine, health serviceinterface.HealthService) *App {
	return &App{Router: r, Health: health}
}

// storage is everything that moves from memory to Redis when REDIS_URL is set.
type storage struct {
	otps     repository.OTPRepository
	limiter  serviceinterface.RateLimiter
	checkers []serviceinterface.HealthChecker
}

const redisConnectTimeout = 5 * time.Second

// provideStorage connects to Redis, or falls back to memory when REDIS_URL is
// empty (bootstrap only allows that in development). The returned cleanup
// closes the connection; Wire calls it if a later provider fails, and main
// calls it after the HTTP server stopped.
func provideStorage(cfg *bootstrap.Config, log *slog.Logger) (*storage, func(), error) {
	if cfg.RedisURL == "" {
		log.Warn("REDIS_URL is not set: OTPs and rate limits are kept in memory, are lost on restart and are not shared between instances")
		return &storage{
			otps:    memory.NewOTPRepository(),
			limiter: memory.NewRateLimiter(),
		}, func() {}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisConnectTimeout)
	defer cancel()
	client, err := redisrepo.NewClient(ctx, cfg.RedisURL)
	if err != nil {
		return nil, nil, err
	}
	log.Info("connected to redis", "addr", client.Options().Addr, "db", client.Options().DB)

	cleanup := func() {
		if err := client.Close(); err != nil {
			log.Warn("closing redis failed", "error", err.Error())
		}
	}
	return &storage{
		otps:     redisrepo.NewOTPRepository(client, cfg.RedisKeyPrefix),
		limiter:  redisrepo.NewRateLimiter(client, cfg.RedisKeyPrefix),
		checkers: []serviceinterface.HealthChecker{redisrepo.NewHealthChecker(client)},
	}, cleanup, nil
}

func provideRateLimiter(store *storage) serviceinterface.RateLimiter { return store.limiter }

func provideHealthService(cfg *bootstrap.Config, log *slog.Logger, version string, store *storage) serviceinterface.HealthService {
	return service.NewHealthService(version, cfg.HealthCheckTimeout, log, store.checkers...)
}

func provideSMSSender(cfg *bootstrap.Config, log *slog.Logger) serviceinterface.SMSSender {
	switch cfg.SMSProvider {
	// Add real providers here (e.g. case bootstrap.SMSProviderFerzz). The
	// value is validated in bootstrap, so the default is the fake sender.
	default:
		return sms.NewFakeSender(log, !cfg.IsProduction())
	}
}

func provideAuthService(
	cfg *bootstrap.Config,
	log *slog.Logger,
	store *storage,
	smsSender serviceinterface.SMSSender,
) serviceinterface.AuthService {
	return service.NewAuthService(store.otps, smsSender, service.OTPConfig{
		Length:         cfg.OTPLength,
		TTL:            cfg.OTPTTL,
		MaxAttempts:    cfg.OTPMaxAttempts,
		ResendCooldown: cfg.OTPResendCooldown,
		ExposeCode:     cfg.OTPExposeCode,
	}, log)
}

func provideHandlers(
	health handlerinterface.HealthHandler,
	auth handlerinterface.AuthHandler,
) router.Handlers {
	return router.Handlers{Health: health, Auth: auth}
}
