package handlerinterface

import "github.com/gin-gonic/gin"

type StoreHandler interface {
	ListStores(c *gin.Context)
	GetStore(c *gin.Context)
	UpdateStore(c *gin.Context)
}
