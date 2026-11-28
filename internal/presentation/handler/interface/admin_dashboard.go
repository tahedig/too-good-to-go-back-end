package handlerinterface

import "github.com/gin-gonic/gin"

type AdminDashboardHandler interface {
	GetDashboard(c *gin.Context)
}
