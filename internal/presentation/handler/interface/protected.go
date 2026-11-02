package handlerinterface

import "github.com/gin-gonic/gin"

// ProtectedHandler serves the routes that sit behind authentication. The
// per-role routes exist to exercise RBAC until real feature routes arrive.
type ProtectedHandler interface {
	Me(c *gin.Context)
	CustomerArea(c *gin.Context)
	VendorArea(c *gin.Context)
	AdminArea(c *gin.Context)
}
