package handlerinterface

import "github.com/gin-gonic/gin"

type StoreRequestHandler interface {
	ListStoreRequests(c *gin.Context)
	GetStoreRequest(c *gin.Context)
	CreateStoreRequest(c *gin.Context)
}
