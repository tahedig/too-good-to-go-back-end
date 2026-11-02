//go:build wireinject

package injector

import (
	"log/slog"

	"github.com/google/wire"

	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/handler"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/router"
)

// InitializeApp builds the whole application. The returned cleanup releases
// external connections (Redis) and must run after the HTTP server stopped.
func InitializeApp(cfg *bootstrap.Config, log *slog.Logger, version string) (*App, func(), error) {
	wire.Build(
		// infrastructure
		provideStorage,
		provideRateLimiter,
		provideSMSSender,

		// application
		provideHealthService,
		provideAuthService,

		// presentation
		handler.NewHealthHandler,
		handler.NewAuthHandler,
		middleware.NewErrorHandler,
		provideHandlers,
		router.New,

		newApp,
	)
	return nil, nil, nil
}
