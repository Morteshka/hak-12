package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"invoicekeeper/internal/middleware"
)

func (h *handlers) listUsers(c *gin.Context) {
	us, err := h.Users.List(c.Request.Context(), middleware.ActorOf(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, nonNil(us))
}

func (h *handlers) createUser(c *gin.Context) {
	var in struct {
		Login    string `json:"login" binding:"required"`
		Email    string `json:"email"` // обязательна — проверяется в сервисе
		Password string `json:"password" binding:"required"`
	}
	if !bind(c, &in) {
		return
	}
	u, err := h.Users.Create(c.Request.Context(), middleware.ActorOf(c), in.Login, in.Email, in.Password)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, u)
}

func (h *handlers) updateUser(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var in struct {
		Login    *string `json:"login"`
		Email    *string `json:"email"`
		Password *string `json:"password"`
	}
	if !bind(c, &in) {
		return
	}
	u, err := h.Users.Update(c.Request.Context(), middleware.ActorOf(c), id, in.Login, in.Email, in.Password)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, u)
}

func (h *handlers) deleteUser(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := h.Users.Delete(c.Request.Context(), middleware.ActorOf(c), id); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
