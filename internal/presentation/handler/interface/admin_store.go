package handlerinterface

import "github.com/gin-gonic/gin"

type AdminStoreHandler interface {
	GetStore(c *gin.Context)
	UpdateStoreStatus(c *gin.Context)
	GetSubscription(c *gin.Context)
	ListOffers(c *gin.Context)
	GetOffer(c *gin.Context)
	UpdateOfferStatus(c *gin.Context)
}
