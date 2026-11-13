package router

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	_ "github.com/tahedig/too-good-to-go/backend/docs"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	handlerinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/handler/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware"
	middlewareinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware/interface"
)

// Handlers groups every HTTP handler. Add new feature handlers here.
type Handlers struct {
	Health       handlerinterface.HealthHandler
	Auth         handlerinterface.AuthHandler
	Protected    handlerinterface.ProtectedHandler
	Store        handlerinterface.StoreHandler
	StoreRequest handlerinterface.StoreRequestHandler
	PublicStore  handlerinterface.PublicStoreHandler
	Profile      handlerinterface.ProfileHandler
}

func New(
	cfg *bootstrap.Config,
	log *slog.Logger,
	errHandler middlewareinterface.ErrorHandler,
	authMW middlewareinterface.AuthMiddleware,
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
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	auth := r.Group("/auth/otp")
	if limiter != nil && cfg.AuthRateLimit > 0 && cfg.AuthRateWindow > 0 {
		// These endpoints send SMS and accept guesses, so they are the
		// first target for abuse.
		auth.Use(middleware.RateLimit(limiter, "auth", cfg.AuthRateLimit, cfg.AuthRateWindow, log))
	}
	auth.POST("/request", h.Auth.RequestOTP)
	auth.POST("/verify", h.Auth.VerifyOTP)

	if authMW == nil && (h.Protected != nil || h.Store != nil || h.StoreRequest != nil || h.Profile != nil) {
		// Refuse to start rather than serve private routes unauthenticated.
		return nil, errors.New("router: auth middleware is required for protected routes")
	}

	if h.Protected != nil {
		r.GET("/auth/me", authMW.Authenticate(), h.Protected.Me)

		// Test routes, one per role, to exercise RBAC.
		test := r.Group("/test", authMW.Authenticate())
		test.GET("/customer", authMW.RequireRoles(enum.RoleCustomer), h.Protected.CustomerArea)
		test.GET("/vendor", authMW.RequireRoles(enum.RoleVendor), h.Protected.VendorArea)
		test.GET("/admin", authMW.RequireRoles(enum.RoleAdmin), h.Protected.AdminArea)
	}

	// Everything under /vendor needs a valid token with the vendor role.
	vendor := r.Group("/vendor")
	if authMW != nil {
		vendor.Use(authMW.Authenticate(), authMW.RequireRoles(enum.RoleVendor))
	}
	if h.Store != nil {
		vendor.GET("/stores", h.Store.ListStores)
		vendor.GET("/stores/:id", h.Store.GetStore)
		vendor.PATCH("/stores/:id", h.Store.UpdateStore)
		vendor.PUT("/stores/:id", h.Store.UpdateStore)
	}
	if h.StoreRequest != nil {
		vendor.GET("/store-requests", h.StoreRequest.ListStoreRequests)
		vendor.GET("/store-requests/:id", h.StoreRequest.GetStoreRequest)
		vendor.POST("/store-requests", h.StoreRequest.CreateStoreRequest)
	}

	if h.Profile != nil {
		customer := r.Group("/customer")
		customer.Use(authMW.Authenticate(), authMW.RequireRoles(enum.RoleCustomer))
		customer.GET("/profile", h.Profile.GetProfile)
		customer.PATCH("/profile", h.Profile.UpdateProfile)
	}

	if h.PublicStore != nil {
		r.GET("/stores/:id", h.PublicStore.GetPublicStore)
		r.HEAD("/stores/:id", h.PublicStore.GetPublicStore)
	}

	return r, nil
}
