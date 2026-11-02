package middlewareinterface

import (
	"github.com/gin-gonic/gin"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
)

type AuthMiddleware interface {
	// Authenticate requires a valid "Authorization: Bearer <jwt>" header (401
	// otherwise) and stores the user id and role on the request context.
	Authenticate() gin.HandlerFunc
	// RequireRoles must run after Authenticate. It answers 403 when the
	// authenticated user's role is not in roles.
	RequireRoles(roles ...enum.Role) gin.HandlerFunc
}
