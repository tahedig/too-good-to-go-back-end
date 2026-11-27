package injector

import (
	"context"

	"github.com/tahedig/too-good-to-go/backend/internal/application/service"
	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	postgresrepo "github.com/tahedig/too-good-to-go/backend/internal/infrastructure/repository/postgres"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/handler"
	handlerinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/handler/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware"
	middlewareinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware/interface"
)

func provideCatalogueService(db *database) serviceinterface.CatalogueService {
	if db == nil || db.pool == nil {
		return nil
	}
	return service.NewCatalogueService(postgresrepo.NewCatalogueRepository(db.pool))
}

func provideCatalogueHandler(svc serviceinterface.CatalogueService) handlerinterface.CatalogueHandler {
	if svc == nil {
		return nil
	}
	return handler.NewCatalogueHandler(svc)
}

// provideUserAuth wires Bearer JWT authentication for catalogue and favorite
// routes, with X-Dev-User-Id fallback in development when no Authorization header is sent.
func provideUserAuth(
	cfg *bootstrap.Config,
	db *database,
	svc serviceinterface.CatalogueService,
	sessions serviceinterface.SessionService,
) middlewareinterface.UserAuth {
	if svc == nil || db == nil || db.pool == nil {
		return nil
	}
	dev := middleware.NewDevUserAuth(func(ctx context.Context, id int64) (bool, error) {
		return svc.UserExists(ctx, id)
	}, cfg.AppEnv == bootstrap.EnvDevelopment)
	if sessions == nil {
		return dev
	}
	resolveDBID := func(ctx context.Context, publicID string) (int64, error) {
		var id int64
		err := db.pool.QueryRow(ctx, `SELECT id FROM users WHERE public_id = $1::uuid AND deleted_at IS NULL`, publicID).Scan(&id)
		return id, err
	}
	return middleware.NewHybridUserAuth(sessions, resolveDBID, dev)
}
