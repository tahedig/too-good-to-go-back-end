package handlerinterface

import "github.com/gin-gonic/gin"

type PublicStoreHandler interface {
	GetPublicStore(c *gin.Context)
}
