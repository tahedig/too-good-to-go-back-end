package handlerinterface

import "github.com/gin-gonic/gin"

type OrderHandler interface {
	ListMyOrders(c *gin.Context)
}
