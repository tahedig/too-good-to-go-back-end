package middleware

import "github.com/gin-gonic/gin"

// SecurityHeaders sets conservative defaults suitable for a JSON API. HSTS is
// left to the TLS-terminating proxy, which knows whether HTTPS is in use.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		c.Next()
	}
}
