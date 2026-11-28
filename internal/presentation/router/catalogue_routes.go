package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	middlewareinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware/interface"
)

// registerCatalogue mounts the home / catalogue / map / favorite-offer routes. It is a no-op when the
// catalogue handler is nil (no DATABASE_URL), so the rest of the API keeps working without a database.
func registerCatalogue(r *gin.Engine, h Handlers, authMW middlewareinterface.AuthMiddleware) {
	if h.Catalogue == nil {
		return
	}
	optional := gin.HandlerFunc(func(c *gin.Context) { c.Next() })
	required := gin.HandlerFunc(func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) })
	if h.UserAuth != nil {
		optional = h.UserAuth.Optional()
	}
	if authMW != nil {
		required = authMW.Authenticate()
	}

	home := r.Group("/home", optional)
	home.GET("/categories", h.Catalogue.HomeCategories)
	home.GET("/today", h.Catalogue.HomeToday)
	home.GET("/popular-nearby", h.Catalogue.HomePopularNearby)
	home.GET("/last-chance", h.Catalogue.HomeLastChance)

	offers := r.Group("/offers", optional)
	offers.GET("", h.Catalogue.SearchOffers)
	offers.GET("/filters", h.Catalogue.Filters)
	offers.GET("/map", h.Catalogue.MapOffers)
	offers.GET("/:id", h.Catalogue.OfferDetail)

	// Same wildcard name (:id) as the existing GET /stores/:id route.
	r.GET("/stores/:id/offers", optional, h.Catalogue.StoreOffers)

	fav := r.Group("/favorites/offers", required)
	fav.GET("", h.Catalogue.ListFavorites)
	fav.PUT("/:id", h.Catalogue.AddFavorite)
	fav.DELETE("/:id", h.Catalogue.RemoveFavorite)
}
