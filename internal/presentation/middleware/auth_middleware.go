package middleware

import (
	"slices"
	"strings"

	"github.com/gin-gonic/gin"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	middlewareinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware/interface"
	"github.com/tahedig/too-good-to-go/backend/pkg/logger"
)

type authMiddleware struct {
	sessions serviceinterface.SessionService
}

// NewAuthMiddleware checks every request against the session service, which
// reloads the user: a suspended account or a revoked role is refused at once,
// not when the token expires.
func NewAuthMiddleware(sessions serviceinterface.SessionService) middlewareinterface.AuthMiddleware {
	return &authMiddleware{sessions: sessions}
}

func (m *authMiddleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		scheme, raw, ok := strings.Cut(c.GetHeader("Authorization"), " ")
		raw = strings.TrimSpace(raw)
		if !ok || !strings.EqualFold(scheme, "Bearer") || raw == "" {
			_ = c.Error(exception.NewUnauthorized("authentication required"))
			c.Abort()
			return
		}

		user, err := m.sessions.Authenticate(c.Request.Context(), raw)
		if err != nil {
			_ = c.Error(err)
			c.Abort()
			return
		}

		c.Set(bootstrap.CtxKeyUserID, user.ID)
		c.Set(bootstrap.CtxKeyUserRole, string(user.Role))
		c.Request = c.Request.WithContext(logger.WithUserID(c.Request.Context(), user.ID))
		c.Next()
	}
}

func (m *authMiddleware) RequireRoles(roles ...enum.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := enum.Role(c.GetString(bootstrap.CtxKeyUserRole))
		if role == "" {
			// Wired without Authenticate: treat as unauthenticated, never as allowed.
			_ = c.Error(exception.NewUnauthorized("authentication required"))
			c.Abort()
			return
		}
		if !slices.Contains(roles, role) {
			_ = c.Error(exception.NewForbidden("you do not have permission to access this resource"))
			c.Abort()
			return
		}
		c.Next()
	}
}
