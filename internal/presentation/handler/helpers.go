package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
)

// abortWithError hands err to the global error middleware, which writes the
// standard error response. Handlers should return through this instead of
// calling c.JSON with an error body themselves.
//
// Example: if err != nil { abortWithError(c, err); return }
func abortWithError(c *gin.Context, err error) {
	_ = c.Error(err)
	c.Abort()
}

// bindJSON decodes the request body into dst and maps failures to typed
// errors: 413 when the body exceeds the BodyLimit middleware, 400 otherwise.
func bindJSON(c *gin.Context, dst any) error {
	err := c.ShouldBindJSON(dst)
	if err == nil {
		return nil
	}
	if maxErr := (*http.MaxBytesError)(nil); errors.As(err, &maxErr) {
		return exception.NewPayloadTooLarge("request body is too large")
	}
	return exception.NewBadRequest("request body must be valid JSON", nil)
}
