package handlerinterface

import "github.com/gin-gonic/gin"

type VendorDashboardHandler interface {
	GetDashboard(c *gin.Context)
}
