package middlewareinterface

import (
	"github.com/gin-gonic/gin"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
)

type AuthMiddleware interface {
	// Authenticate requires a valid "Authorization: Bearer <jwt>" header (401
	// otherwise) for a user that still exists, is active and has a role (403
	// otherwise). It stores the user id and current role on the context.
	Authenticate() gin.HandlerFunc
	// RequireRoles must run after Authenticate. It answers 403 when the
	// authenticated user's role is not in roles.
	RequireRoles(roles ...enum.Role) gin.HandlerFunc
}
