package handlerinterface

import "github.com/gin-gonic/gin"

type VendorApprovalHandler interface {
	ListVendors(c *gin.Context)
	GetVendor(c *gin.Context)
	Approve(c *gin.Context)
	Reject(c *gin.Context)
	Suspend(c *gin.Context)
	Reactivate(c *gin.Context)
}
