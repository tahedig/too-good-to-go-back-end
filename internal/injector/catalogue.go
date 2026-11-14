package injector

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tahedig/too-good-to-go/backend/internal/application/service"
	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	postgresrepo "github.com/tahedig/too-good-to-go/backend/internal/infrastructure/repository/postgres"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/handler"
	handlerinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/handler/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware"
	middlewareinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware/interface"
)

const databaseConnectTimeout = 5 * time.Second

// provideDatabase connects to PostgreSQL. With DATABASE_URL empty it returns a nil pool and the
// catalogue endpoints are simply not registered (the OTP/health parts of the API keep working).
// The returned cleanup closes the pool; Wire runs it if a later provider fails and main runs it
// after the HTTP server stopped.
func provideDatabase(cfg *bootstrap.Config, log *slog.Logger) (*pgxpool.Pool, func(), error) {
	if cfg.DatabaseURL == "" {
		log.Warn("DATABASE_URL is not set: the catalogue endpoints (home, offers, favorites) are disabled")
		return nil, func() {}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), databaseConnectTimeout)
	defer cancel()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("connect postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("ping postgres: %w", err)
	}
	log.Info("connected to postgres")
	return pool, pool.Close, nil
}

func provideCatalogueService(pool *pgxpool.Pool) serviceinterface.CatalogueService {
	if pool == nil {
		return nil
	}
	return service.NewCatalogueService(postgresrepo.NewCatalogueRepository(pool))
}

func provideCatalogueHandler(svc serviceinterface.CatalogueService) handlerinterface.CatalogueHandler {
	if svc == nil {
		return nil
	}
	return handler.NewCatalogueHandler(svc)
}

// provideUserAuth wires the TEMPORARY X-Dev-User-Id authentication, enabled only when APP_ENV=development.
// Replace it with the real token middleware once /auth/otp/verify issues tokens.
func provideUserAuth(cfg *bootstrap.Config, svc serviceinterface.CatalogueService) middlewareinterface.UserAuth {
	if svc == nil {
		return nil
	}
	return middleware.NewDevUserAuth(func(ctx context.Context, id int64) (bool, error) {
		return svc.UserExists(ctx, id)
	}, cfg.AppEnv == bootstrap.EnvDevelopment)
}
