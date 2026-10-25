package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"invoicekeeper/internal/middleware"
	"invoicekeeper/internal/service"
)

// ---------- каталог услуг ----------

func (h *handlers) listServices(c *gin.Context) {
	list, err := h.Catalog.List(c.Request.Context(), middleware.ActorOf(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, nonNil(list))
}

func (h *handlers) createService(c *gin.Context) {
	var in service.ServiceInput
	if !bind(c, &in) {
		return
	}
	sv, err := h.Catalog.Create(c.Request.Context(), middleware.ActorOf(c), in)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, sv)
}

func (h *handlers) updateService(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var in service.ServiceInput
	if !bind(c, &in) {
		return
	}
	sv, err := h.Catalog.Update(c.Request.Context(), middleware.ActorOf(c), id, in)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, sv)
}

func (h *handlers) deleteService(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := h.Catalog.Delete(c.Request.Context(), middleware.ActorOf(c), id); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// subscribe — подключить услугу: создаётся личная подписка (регулярный платёж) текущего пользователя.
func (h *handlers) subscribe(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	rp, err := h.Catalog.Subscribe(c.Request.Context(), middleware.ActorOf(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, rp)
}
