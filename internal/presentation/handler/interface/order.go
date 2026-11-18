package handlerinterface

import "github.com/gin-gonic/gin"

type OrderHandler interface {
	ListMyOrders(c *gin.Context)
	GetVendorStore(c *gin.Context)
	ListVendorOrders(c *gin.Context)
	GetVendorOrder(c *gin.Context)
	GetVendorOrderSummary(c *gin.Context)
	VerifyPickup(c *gin.Context)
	UpdateVendorOrderStatus(c *gin.Context)
}
