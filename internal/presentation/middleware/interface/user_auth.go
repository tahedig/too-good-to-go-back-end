package middlewareinterface

import "github.com/gin-gonic/gin"

// UserAuth identifies the calling customer.
//   - Optional never rejects anonymous calls; it only fills in the user when credentials are sent.
//   - Required answers 401 when there is no authenticated user.
//
// Handlers read the result with middleware.UserID(c).
type UserAuth interface {
	Optional() gin.HandlerFunc
	Required() gin.HandlerFunc
}
