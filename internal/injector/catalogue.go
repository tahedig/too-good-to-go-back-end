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
