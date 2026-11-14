
package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders sets conservative defaults suitable for a JSON API.
// Swagger UI receives a separate CSP policy so its local assets can load.
// HSTS is left to the TLS-terminating proxy.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")

		if strings.HasPrefix(c.Request.URL.Path, "/swagger/") {
			h.Set(
				"Content-Security-Policy",
				"default-src 'none'; "+
					"script-src 'self' 'unsafe-inline'; "+
					"style-src 'self' 'unsafe-inline'; "+
					"img-src 'self' data:; "+
					"connect-src 'self'; "+
					"frame-ancestors 'none'",
			)
		} else {
			h.Set(
				"Content-Security-Policy",
				"default-src 'none'; frame-ancestors 'none'",
			)
		}

		c.Next()
	}
}
