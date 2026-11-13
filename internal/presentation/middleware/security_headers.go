package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	// apiCSP: JSON responses never load anything.
	apiCSP = "default-src 'none'; frame-ancestors 'none'"
	// docsCSP lets the Swagger UI page load its own scripts, styles and
	// images (all served from /swagger/, no CDN). The UI sets inline styles
	// and data: images, so those are allowed, on these paths only.
	docsCSP = "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data:; frame-ancestors 'none'"
	docsPathPrefix = "/swagger/"
)

// SecurityHeaders sets conservative defaults suitable for a JSON API. HSTS is
// left to the TLS-terminating proxy, which knows whether HTTPS is in use.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		if strings.HasPrefix(c.Request.URL.Path, docsPathPrefix) {
			h.Set("Content-Security-Policy", docsCSP)
		} else {
			h.Set("Content-Security-Policy", apiCSP)
		}
		c.Next()
	}
}
