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
	tokens serviceinterface.TokenManager
}

func NewAuthMiddleware(tokens serviceinterface.TokenManager) middlewareinterface.AuthMiddleware {
	return &authMiddleware{tokens: tokens}
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

		claims, err := m.tokens.Parse(raw)
		if err != nil {
			_ = c.Error(exception.NewUnauthorized("invalid or expired token"))
			c.Abort()
			return
		}

		c.Set(bootstrap.CtxKeyUserID, claims.UserID)
		c.Set(bootstrap.CtxKeyUserRole, string(claims.Role))
		c.Request = c.Request.WithContext(logger.WithUserID(c.Request.Context(), claims.UserID))
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
