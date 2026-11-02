package handlerinterface

import "github.com/gin-gonic/gin"

type AuthHandler interface {
	RequestOTP(c *gin.Context)
	VerifyOTP(c *gin.Context)
}
