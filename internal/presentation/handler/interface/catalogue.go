package handlerinterface

import "github.com/gin-gonic/gin"

type CatalogueHandler interface {
	HomeCategories(c *gin.Context)
	HomeToday(c *gin.Context)
	HomePopularNearby(c *gin.Context)
	HomeLastChance(c *gin.Context)

	SearchOffers(c *gin.Context)
	StoreOffers(c *gin.Context)
	Filters(c *gin.Context)
	MapOffers(c *gin.Context)

	ListFavorites(c *gin.Context)
	AddFavorite(c *gin.Context)
	RemoveFavorite(c *gin.Context)
}
