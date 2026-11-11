package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	middlewareinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware/interface"
	"github.com/tahedig/too-good-to-go/backend/pkg/logger"
)

type errorHandler struct {
	log *slog.Logger
}

func NewErrorHandler(log *slog.Logger) middlewareinterface.ErrorHandler {
	return &errorHandler{log: log}
}

func (h *errorHandler) Handle() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 {
			return
		}
		err := c.Errors.Last().Err
		if c.Writer.Written() {
			// A handler already wrote a body; we can only log.
			logger.FromContext(c.Request.Context(), h.log).Error("error after response was written", "error", err.Error())
			return
		}
		h.write(c, err)
	}
}

func (h *errorHandler) Recover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler { // intentional abort, let net/http handle it
				panic(rec)
			}
			logger.FromContext(c.Request.Context(), h.log).Error("panic recovered",
				"panic", fmt.Sprint(rec), "stack", string(debug.Stack()))

			if c.Writer.Written() {
				c.Abort()
				return
			}
			h.write(c, exception.NewInternalServer(fmt.Errorf("panic: %v", rec)))
		}()
		c.Next()
	}
}

func (h *errorHandler) NotFound() gin.HandlerFunc {
	return func(c *gin.Context) {
		_ = c.Error(exception.NewNotFound("route not found"))
	}
}

func (h *errorHandler) MethodNotAllowed() gin.HandlerFunc {
	return func(c *gin.Context) {
		_ = c.Error(exception.NewMethodNotAllowed("method not allowed for this route"))
	}
}

func (h *errorHandler) write(c *gin.Context, err error) {
	var appErr exception.AppError
	if !errors.As(err, &appErr) {
		appErr = exception.NewInternalServer(err)
	}

	if appErr.StatusCode() >= http.StatusInternalServerError {
		logger.FromContext(c.Request.Context(), h.log).Error("request failed",
			"status", appErr.StatusCode(), "code", appErr.Code(), "error", err.Error())
	}

	var retryable exception.Retryable
	if errors.As(appErr, &retryable) && retryable.RetryAfter() > 0 {
		c.Header("Retry-After", strconv.Itoa(exception.RetryAfterSeconds(retryable.RetryAfter())))
	}

	c.AbortWithStatusJSON(appErr.StatusCode(), dto.ErrorResponse{
		Error: dto.ErrorBody{
			Code:      appErr.Code(),
			Message:   appErr.Message(),
			Details:   appErr.Details(),
			RequestID: c.GetString(bootstrap.CtxKeyRequestID),
		},
	})
}
