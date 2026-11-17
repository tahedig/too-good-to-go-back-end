package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	middlewareinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware/interface"
)

const userIDKey = "catalogue.user_id"

// DevUserHeader carries users.id while real authentication does not exist yet.
const DevUserHeader = "X-Dev-User-Id"

// SetUserID is what the REAL auth middleware must call once /auth/otp/verify issues tokens:
//
//	middleware.SetUserID(c, usersTableID)
func SetUserID(c *gin.Context, id int64) { c.Set(userIDKey, id) }

// UserID returns the authenticated user, if any.
func UserID(c *gin.Context) (int64, bool) {
	if v, ok := c.Get(userIDKey); ok {
		if id, ok := v.(int64); ok {
			return id, true
		}
	}
	return 0, false
}

// UserIDPtr is UserID as a nullable value (nil = anonymous).
func UserIDPtr(c *gin.Context) *int64 {
	if id, ok := UserID(c); ok {
		return &id
	}
	return nil
}

type devUserAuth struct {
	exists  func(ctx context.Context, id int64) (bool, error)
	enabled bool
}

// NewDevUserAuth is a TEMPORARY stand-in for token authentication: the caller states who it is in the
// X-Dev-User-Id header (a users.id). It only works when enabled is true (APP_ENV=development); otherwise
// the header is ignored and every Required route answers 401, so it can never be abused in production.
// Replace it with a JWT middleware that calls SetUserID.
func NewDevUserAuth(exists func(ctx context.Context, id int64) (bool, error), enabled bool) middlewareinterface.UserAuth {
	return &devUserAuth{exists: exists, enabled: enabled}
}

func (a *devUserAuth) Optional() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := strings.TrimSpace(c.GetHeader(DevUserHeader))
		if !a.enabled || raw == "" {
			c.Next()
			return
		}
		if a.identify(c, raw) {
			c.Next()
		}
	}
}

func (a *devUserAuth) Required() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := strings.TrimSpace(c.GetHeader(DevUserHeader))
		if !a.enabled || raw == "" {
			abortUnauthorized(c, "authentication required")
			return
		}
		if a.identify(c, raw) {
			c.Next()
		}
	}
}

func (a *devUserAuth) identify(c *gin.Context, raw string) bool {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 {
		abortUnauthorized(c, "invalid "+DevUserHeader)
		return false
	}
	ok, err := a.exists(c.Request.Context(), id)
	if err != nil {
		_ = c.Error(err) // global error middleware -> 500, details only in the log
		c.Abort()
		return false
	}
	if !ok {
		abortUnauthorized(c, "unknown user")
		return false
	}
	SetUserID(c, id)
	return true
}

// abortUnauthorized writes the standard error envelope. If the project has a typed Unauthorized
// exception handled by the error middleware, return that through c.Error instead.
func abortUnauthorized(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Error: dto.ErrorBody{
		Code:      "unauthorized",
		Message:   msg,
		RequestID: c.Writer.Header().Get("X-Request-ID"),
	}})
}
