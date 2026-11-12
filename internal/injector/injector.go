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
	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/repository/memory"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/repository/postgres"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/repository/postgres/driver"
	redisrepo "github.com/tahedig/too-good-to-go/backend/internal/infrastructure/repository/redis"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/sms"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/token"
	handlerinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/handler/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/router"
)

type App struct {
	Router *gin.Engine
	Health serviceinterface.HealthService
}

func provideTokenManager(cfg *bootstrap.Config) serviceinterface.TokenManager {
	return token.NewJWTManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAccessTTL)
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

func provideHealthService(cfg *bootstrap.Config, log *slog.Logger, version string, store *storage, db *database) serviceinterface.HealthService {
	checkers := append(append([]serviceinterface.HealthChecker{}, db.checkers...), store.checkers...)
	return service.NewHealthService(version, cfg.HealthCheckTimeout, log, checkers...)
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

// database is everything that moves from memory to PostgreSQL when
// DATABASE_URL is set.
type database struct {
	users    repository.UserRepository
	orders   repository.OrderRepository
	checkers []serviceinterface.HealthChecker
}

const dbConnectTimeout = 5 * time.Second

// provideDatabase connects to PostgreSQL and checks the migrations were
// applied, or falls back to memory when DATABASE_URL is empty (bootstrap only
// allows that in development). The cleanup closes the pool after the HTTP
// server stopped.
func provideDatabase(cfg *bootstrap.Config, log *slog.Logger) (*database, func(), error) {
	if cfg.DatabaseURL == "" {
		log.Warn("DATABASE_URL is not set: users and orders are kept in memory, are lost on restart and are not shared between instances")
		return &database{users: memory.NewUserRepository(), orders: memory.NewOrderRepository()}, func() {}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), dbConnectTimeout)
	defer cancel()
	pool, err := driver.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, err
	}
	if err := postgres.VerifySchema(ctx, pool); err != nil {
		pool.Close()
		return nil, nil, err
	}
	connCfg := pool.Config().ConnConfig
	log.Info("connected to postgres", "host", connCfg.Host, "port", connCfg.Port, "database", connCfg.Database)

	return &database{
		users:    postgres.NewUserRepository(pool),
		orders:   postgres.NewOrderRepository(pool),
		checkers: []serviceinterface.HealthChecker{driver.NewHealthChecker(pool)},
	}, pool.Close, nil
}

func provideUserRepository(db *database) repository.UserRepository { return db.users }

func provideOrderRepository(db *database) repository.OrderRepository { return db.orders }

// providePhoneRoles maps ADMIN_PHONES / VENDOR_PHONES to the role a new user
// with that phone receives. Admin wins if a phone is in both lists.
func providePhoneRoles(cfg *bootstrap.Config) map[string]enum.Role {
	roles := make(map[string]enum.Role, len(cfg.AdminPhones)+len(cfg.VendorPhones))
	for _, p := range cfg.VendorPhones {
		roles[p] = enum.RoleVendor
	}
	for _, p := range cfg.AdminPhones {
		roles[p] = enum.RoleAdmin
	}
	return roles
}

func provideSessionService(
	cfg *bootstrap.Config,
	log *slog.Logger,
	users repository.UserRepository,
	tokens serviceinterface.TokenManager,
) serviceinterface.SessionService {
	return service.NewSessionService(users, tokens, providePhoneRoles(cfg), log)
}

func provideHandlers(
	health handlerinterface.HealthHandler,
	auth handlerinterface.AuthHandler,
	protected handlerinterface.ProtectedHandler,
	order handlerinterface.OrderHandler,
) router.Handlers {
	return router.Handlers{Health: health, Auth: auth, Protected: protected, Order: order}
}
