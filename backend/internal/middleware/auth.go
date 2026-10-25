// Package middleware — HTTP-middleware: аутентификация, роли, CORS.
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"invoicekeeper/internal/models"
	"invoicekeeper/internal/service"
)

const (
	ctxUser    = "user"
	ctxSession = "sid"
)

// Auth проверяет Bearer-токен и живую сессию. Без него данные недоступны.
func Auth(auth *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		tok, ok := strings.CutPrefix(h, "Bearer ")
		if !ok || tok == "" {
			abort(c, http.StatusUnauthorized, "authorization required")
			return
		}
		u, sid, err := auth.Authenticate(c.Request.Context(), tok)
		if err != nil {
			abort(c, http.StatusUnauthorized, err.Error())
			return
		}
		c.Set(ctxUser, u)
		c.Set(ctxSession, sid)
		c.Next()
	}
}

// RequireRole — проверка прав на уровне API (не только скрытие кнопок во фронтенде).
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if CurrentUser(c).Role != role {
			abort(c, http.StatusForbidden, "forbidden: "+role+" role required")
			return
		}
		c.Next()
	}
}

func CurrentUser(c *gin.Context) *models.User { return c.MustGet(ctxUser).(*models.User) }
func SessionID(c *gin.Context) string         { return c.MustGet(ctxSession).(string) }

// ActorOf собирает service.Actor из контекста запроса.
func ActorOf(c *gin.Context) service.Actor {
	u := CurrentUser(c)
	return service.Actor{UserID: &u.ID, Login: u.Login, Role: u.Role, IP: c.ClientIP(), UserAgent: c.Request.UserAgent()}
}

// ClientOf — IP и User-Agent запроса (для журнала, в т.ч. до входа).
func ClientOf(c *gin.Context) service.Client {
	return service.Client{IP: c.ClientIP(), UserAgent: c.Request.UserAgent()}
}

func CORS(origins []string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, o := range origins {
		allowed[o] = true
	}
	return func(c *gin.Context) {
		if o := c.GetHeader("Origin"); o != "" && (allowed[o] || allowed["*"]) {
			c.Header("Access-Control-Allow-Origin", o)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func abort(c *gin.Context, code int, msg string) {
	c.AbortWithStatusJSON(code, gin.H{"error": msg})
}
