package handlerinterface

import "github.com/gin-gonic/gin"

type AdminStoreManagementHandler interface {
	ListStores(c *gin.Context)
	GetManagedStore(c *gin.Context)
	UpdateManagedStore(c *gin.Context)
}
