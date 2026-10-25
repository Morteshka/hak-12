package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"invoicekeeper/internal/middleware"
	"invoicekeeper/internal/service"
)

// authConfig — публичные настройки для экрана входа.
func (h *handlers) authConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"registration_enabled": h.AllowRegistration, "email_verification": h.AllowRegistration && h.Auth.EmailVerificationOn(c.Request.Context())})
}

// register — самостоятельная регистрация. Роль всегда user: права админа так не получить.
func (h *handlers) register(c *gin.Context) {
	if !h.AllowRegistration {
		fail(c, service.Forbidden("registration is disabled, ask the administrator for an account"))
		return
	}
	var in struct {
		Login    string `json:"login"`
		Email    string `json:"email"`
		Password string `json:"password" binding:"required"`
	}
	if !bind(c, &in) {
		return
	}
	if tooMany(c, registerIP, c.ClientIP()) {
		return
	}
	registerIP.Hit(c.ClientIP())
	pair, pend, err := h.Auth.Register(c.Request.Context(), in.Login, in.Email, in.Password, middleware.ClientOf(c))
	if err != nil {
		fail(c, err)
		return
	}
	if pend != nil { // аккаунт создастся после ввода кода из письма
		c.JSON(http.StatusAccepted, pend)
		return
	}
	c.JSON(http.StatusCreated, pair)
}

// registerConfirm — код из письма: верный код создаёт аккаунт и выполняет вход.
func (h *handlers) registerConfirm(c *gin.Context) {
	var in struct {
		Email string `json:"email" binding:"required"`
		Code  string `json:"code" binding:"required"`
	}
	if !bind(c, &in) {
		return
	}
	if tooMany(c, confirmIP, c.ClientIP()) {
		return
	}
	pair, err := h.Auth.ConfirmRegistration(c.Request.Context(), in.Email, in.Code, middleware.ClientOf(c))
	if err != nil {
		confirmIP.Hit(c.ClientIP())
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, pair)
}

// registerResend — отправить код ещё раз (не чаще раза в минуту).
func (h *handlers) registerResend(c *gin.Context) {
	var in struct {
		Email string `json:"email" binding:"required"`
	}
	if !bind(c, &in) {
		return
	}
	if tooMany(c, registerIP, c.ClientIP()) {
		return
	}
	registerIP.Hit(c.ClientIP())
	pend, err := h.Auth.ResendCode(c.Request.Context(), in.Email)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, pend)
}

func (h *handlers) changePassword(c *gin.Context) {
	var in struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if !bind(c, &in) {
		return
	}
	err := h.Auth.ChangePassword(c.Request.Context(), middleware.ActorOf(c), middleware.CurrentUser(c), middleware.SessionID(c), in.OldPassword, in.NewPassword)
	if err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// auditLogs — журнал с фильтрами: from, to (YYYY-MM-DD), user_id, action, limit.
func (h *handlers) auditLogs(c *gin.Context) {
	from, err := dateQuery(c, "from")
	if err != nil {
		fail(c, err)
		return
	}
	to, err := dateQuery(c, "to")
	if err != nil {
		fail(c, err)
		return
	}
	q := service.AuditQuery{From: from, To: to, Action: c.Query("action")}
	q.Limit, _ = strconv.Atoi(c.DefaultQuery("limit", "300"))
	if v := c.Query("user_id"); v != "" {
		id, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			fail(c, service.Invalid("user_id must be a number"))
			return
		}
		uid := uint(id)
		q.UserID = &uid
	}
	logs, err := h.Audit.List(c.Request.Context(), middleware.ActorOf(c), q, h.Loc)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, nonNil(logs))
}

func (h *handlers) auditActions(c *gin.Context) {
	list, err := h.Audit.Actions(c.Request.Context(), middleware.ActorOf(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, nonNil(list))
}

func (h *handlers) updateMe(c *gin.Context) {
	var in struct {
		Login string `json:"login" binding:"required"`
	}
	if !bind(c, &in) {
		return
	}
	u, err := h.Auth.ChangeLogin(c.Request.Context(), middleware.ActorOf(c), middleware.CurrentUser(c), in.Login)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, u)
}
