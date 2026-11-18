package handlerinterface

import "github.com/gin-gonic/gin"

type AdminStoreHandler interface {
	GetStore(c *gin.Context)
	UpdateStoreStatus(c *gin.Context)
}
