package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"invoicekeeper/internal/middleware"
)

// timeState — GET /time: системная дата, реальная дата и сдвиг машины времени.
func (h *handlers) timeState(c *gin.Context) {
	c.JSON(http.StatusOK, h.Payments.TimeState())
}

// setTime — PUT /time {"offset_days": N}: администратор сдвигает системную дату (0 — вернуть сегодня).
func (h *handlers) setTime(c *gin.Context) {
	var in struct {
		OffsetDays *int `json:"offset_days" binding:"required"`
	}
	if !bind(c, &in) {
		return
	}
	st, err := h.Payments.SetTimeOffset(c.Request.Context(), middleware.ActorOf(c), *in.OffsetDays)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, st)
}
