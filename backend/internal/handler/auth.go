package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"invoicekeeper/internal/middleware"
	"invoicekeeper/internal/service"
)

func (h *handlers) login(c *gin.Context) {
	var in struct {
		Login    string `json:"login" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if !bind(c, &in) {
		return
	}
	ip := c.ClientIP()
	if tooMany(c, loginByIP, ip) || tooMany(c, loginByName, in.Login) {
		return
	}
	pair, err := h.Auth.Login(c.Request.Context(), in.Login, in.Password, middleware.ClientOf(c))
	if err != nil {
		if service.IsUnauthorized(err) { // неверный пароль — считаем попытку
			loginByIP.Hit(ip)
			loginByName.Hit(in.Login)
		}
		fail(c, err)
		return
	}
	loginByName.Reset(in.Login)
	c.JSON(http.StatusOK, pair)
}

// Защита от подбора: не больше 10 неудачных входов в один логин и 30 с одного IP за 15 минут,
// не больше 30 регистраций с одного IP в час (с запасом для общего Wi-Fi на демо).
var (
	loginByName = middleware.NewLimiter(10, 15*time.Minute)
	loginByIP   = middleware.NewLimiter(30, 15*time.Minute)
	registerIP  = middleware.NewLimiter(30, time.Hour)
	confirmIP   = middleware.NewLimiter(30, 15*time.Minute) // неверные коды подтверждения почты с одного IP
)

func tooMany(c *gin.Context, l *middleware.Limiter, key string) bool {
	blocked, wait := l.Blocked(key)
	if !blocked {
		return false
	}
	mins := int(wait.Minutes()) + 1
	c.Header("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many attempts, try again in " + strconv.Itoa(mins) + " min"})
	return true
}

func (h *handlers) refresh(c *gin.Context) {
	var in struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if !bind(c, &in) {
		return
	}
	pair, err := h.Auth.Refresh(c.Request.Context(), in.RefreshToken)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, pair)
}

func (h *handlers) me(c *gin.Context) { c.JSON(http.StatusOK, middleware.CurrentUser(c)) }

func (h *handlers) logout(c *gin.Context) {
	if err := h.Auth.Logout(c.Request.Context(), middleware.ActorOf(c), middleware.SessionID(c)); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// loginTelegram — POST /auth/telegram {init_data}: вход из Telegram Mini App по подписи Telegram.
func (h *handlers) loginTelegram(c *gin.Context) {
	var in struct {
		InitData string `json:"init_data" binding:"required"`
	}
	if !bind(c, &in) {
		return
	}
	ip := c.ClientIP()
	if tooMany(c, loginByIP, ip) {
		return
	}
	pair, err := h.Auth.LoginTelegram(c.Request.Context(), in.InitData, middleware.ClientOf(c))
	if err != nil {
		if service.IsUnauthorized(err) {
			loginByIP.Hit(ip)
		}
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, pair)
}

// linkTelegramWebApp — POST /auth/telegram/link {init_data}: вошли паролем внутри Mini App — подключаем Telegram.
func (h *handlers) linkTelegramWebApp(c *gin.Context) {
	var in struct {
		InitData string `json:"init_data" binding:"required"`
	}
	if !bind(c, &in) {
		return
	}
	if err := h.Auth.LinkTelegramWebApp(c.Request.Context(), middleware.ActorOf(c), in.InitData); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
