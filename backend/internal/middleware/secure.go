package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// SecureHeaders — базовые защитные заголовки для ответов API.
func SecureHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if !strings.HasPrefix(c.Request.URL.Path, "/api/v1/docs") { // Swagger открывается как страница
			h.Set("X-Frame-Options", "DENY")
			h.Set("Cache-Control", "no-store")
		}
		c.Next()
	}
}

// LimitBody ограничивает размер JSON-запросов (чеки — multipart, у них свой предел 10 МБ).
func LimitBody(max int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil && !strings.HasPrefix(c.ContentType(), "multipart/") {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, max)
		}
		c.Next()
	}
}
