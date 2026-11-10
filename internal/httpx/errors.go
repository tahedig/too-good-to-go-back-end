package httpx

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tahedig/too-good-to-go/backend/internal/apperr"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := uuid.NewString()
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

func body(code, msg, reqID string) gin.H {
	return gin.H{"error": gin.H{"code": code, "message": msg, "request_id": reqID}}
}

// ErrorHandler: handlers just do `c.Error(err); return`. Full detail goes to the log,
// the client only gets code + generic message + request_id.
func ErrorHandler(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 || c.Writer.Written() {
			return
		}
		ae := apperr.FromDB(c.Errors.Last().Err)
		reqID := c.GetString("request_id")

		level := slog.LevelWarn
		if ae.Status >= 500 {
			level = slog.LevelError
		}
		log.Log(c.Request.Context(), level, "request failed",
			"request_id", reqID, "method", c.Request.Method, "route", c.FullPath(),
			"status", ae.Status, "code", ae.Code, "cause", ae.Err) // cause may contain SQL details: logs only

		c.AbortWithStatusJSON(ae.Status, body(ae.Code, ae.Message, reqID))
	}
}

// Recovery: a panic must not leak a stack trace to the client.
func Recovery(log *slog.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		reqID := c.GetString("request_id")
		log.Error("panic recovered", "request_id", reqID, "panic", recovered)
		c.AbortWithStatusJSON(http.StatusInternalServerError,
			body("internal_error", "Something went wrong. Please try again later.", reqID))
	})
}
