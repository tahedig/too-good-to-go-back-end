package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	corsAllowMethods  = "GET, HEAD, POST, PUT, PATCH, DELETE"
	corsAllowHeaders  = "Authorization, Content-Type, X-Request-ID"
	corsExposeHeaders = "X-Request-ID, Retry-After"
	corsMaxAge        = "600" // seconds a browser may cache a preflight
)

// CORS lets browsers on the listed origins (exact scheme://host[:port]) call
// the API with a bearer token. Other origins get no CORS headers, so the
// browser blocks them; the request itself is still served and authenticated
// as usual. Preflights from allowed origins are answered with 204.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = true
	}
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Add("Vary", "Origin")
		origin := c.GetHeader("Origin")
		if !allowed[origin] {
			c.Next()
			return
		}
		h.Set("Access-Control-Allow-Origin", origin)
		h.Set("Access-Control-Expose-Headers", corsExposeHeaders)
		if c.Request.Method == http.MethodOptions && c.GetHeader("Access-Control-Request-Method") != "" {
			h.Set("Access-Control-Allow-Methods", corsAllowMethods)
			h.Set("Access-Control-Allow-Headers", corsAllowHeaders)
			h.Set("Access-Control-Max-Age", corsMaxAge)
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
