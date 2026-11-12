package handlerinterface

import "github.com/gin-gonic/gin"

// ProfileHandler serves the authenticated customer's profile.
type ProfileHandler interface {
	GetProfile(c *gin.Context)
	UpdateProfile(c *gin.Context)
}
