// Package injector assembles the application dependency graph. The providers
// live here; wire.go lists them and `go generate ./internal/injector`
// regenerates wire_gen.go.
package injector

import (
	"context"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

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
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/handler"
	handlerinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/handler/interface"
	middlewareinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware/interface"
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
	users         repository.UserRepository
	orders        repository.OrderRepository
	reservations  repository.ReservationRepository
	vendors       repository.VendorApprovalRepository
	stores        repository.StoreRepository
	offers        repository.OfferRepository
	storeRequests repository.StoreRequestRepository
	pool          *pgxpool.Pool
	checkers      []serviceinterface.HealthChecker
}

const dbConnectTimeout = 5 * time.Second

// provideDatabase connects to PostgreSQL and checks the migrations were
// applied, or falls back to memory when DATABASE_URL is empty (bootstrap only
// allows that in development). The cleanup closes the pool after the HTTP
// server stopped.
func provideDatabase(cfg *bootstrap.Config, log *slog.Logger) (*database, func(), error) {
	if cfg.DatabaseURL == "" {
		log.Warn("DATABASE_URL is not set: users and orders are kept in memory, are lost on restart and are not shared between instances")
		memOrders := memory.NewOrderRepository()
		return &database{
			users:        memory.NewUserRepository(),
			orders:       memOrders,
			reservations: memory.NewReservationRepository(memOrders),
			vendors:      memory.NewVendorApprovalRepository(),
		}, func() {}, nil
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
		users:         postgres.NewUserRepository(pool),
		orders:        postgres.NewOrderRepository(pool),
		reservations:  postgres.NewReservationRepository(pool),
		vendors:       postgres.NewVendorApprovalRepository(pool),
		stores:        postgres.NewStoreRepository(pool),
		offers:        postgres.NewOfferRepository(pool),
		storeRequests: postgres.NewStoreRequestRepository(pool),
		pool:          pool,
		checkers:      []serviceinterface.HealthChecker{driver.NewHealthChecker(pool)},
	}, pool.Close, nil
}

func provideUserRepository(db *database) repository.UserRepository { return db.users }

func provideOrderRepository(db *database) repository.OrderRepository { return db.orders }

func provideReservationRepository(db *database) repository.ReservationRepository {
	return db.reservations
}

func provideVendorApprovalRepository(db *database) repository.VendorApprovalRepository {
	return db.vendors
}

func provideStoreRepository(db *database) repository.StoreRepository { return db.stores }

func provideOfferRepository(db *database) repository.OfferRepository { return db.offers }

func provideStoreRequestRepository(db *database) repository.StoreRequestRepository {
	return db.storeRequests
}

func provideStoreService(stores repository.StoreRepository) serviceinterface.StoreService {
	if stores == nil {
		return nil
	}
	return service.NewStoreService(stores)
}

func provideStoreRequestService(reqs repository.StoreRequestRepository) serviceinterface.StoreRequestService {
	if reqs == nil {
		return nil
	}
	return service.NewStoreRequestService(reqs)
}

func providePublicStoreService(stores repository.StoreRepository, offers repository.OfferRepository) serviceinterface.PublicStoreService {
	if stores == nil {
		return nil
	}
	return service.NewPublicStoreService(stores, offers)
}

func provideStoreHandler(svc serviceinterface.StoreService) handlerinterface.StoreHandler {
	if svc == nil {
		return nil
	}
	return handler.NewStoreHandler(svc)
}

func provideStoreRequestHandler(svc serviceinterface.StoreRequestService) handlerinterface.StoreRequestHandler {
	if svc == nil {
		return nil
	}
	return handler.NewStoreRequestHandler(svc)
}

func providePublicStoreHandler(svc serviceinterface.PublicStoreService) handlerinterface.PublicStoreHandler {
	if svc == nil {
		return nil
	}
	return handler.NewPublicStoreHandler(svc)
}

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

func provideProfileService(users repository.UserRepository) serviceinterface.ProfileService {
	return service.NewProfileService(users)
}

func provideHandlers(
	health handlerinterface.HealthHandler,
	auth handlerinterface.AuthHandler,
	catalogue handlerinterface.CatalogueHandler,
	userAuth middlewareinterface.UserAuth,
	protected handlerinterface.ProtectedHandler,
	profile handlerinterface.ProfileHandler,
	order handlerinterface.OrderHandler,
	reservation handlerinterface.ReservationHandler,
	vendorApproval handlerinterface.VendorApprovalHandler,
	store handlerinterface.StoreHandler,
	storeRequest handlerinterface.StoreRequestHandler,
	publicStore handlerinterface.PublicStoreHandler,
) router.Handlers {
	return router.Handlers{
		Health:         health,
		Auth:           auth,
		Catalogue:      catalogue,
		UserAuth:       userAuth,
		Protected:      protected,
		Profile:        profile,
		Order:          order,
		Reservation:    reservation,
		VendorApproval: vendorApproval,
		Store:          store,
		StoreRequest:   storeRequest,
		PublicStore:    publicStore,
	}
}
