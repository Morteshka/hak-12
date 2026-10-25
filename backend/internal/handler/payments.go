package handler

import (
	"invoicekeeper/internal/models"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"invoicekeeper/internal/middleware"
	"invoicekeeper/internal/service"
)

func (h *handlers) listRecurring(c *gin.Context) {
	list, err := h.Payments.ListRecurring(c.Request.Context(), middleware.ActorOf(c), c.Query("owner"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, nonNil(list))
}

func (h *handlers) createRecurring(c *gin.Context) {
	var in service.CreateRecurringInput
	if !bind(c, &in) {
		return
	}
	rp, err := h.Payments.CreateRecurring(c.Request.Context(), middleware.ActorOf(c), in)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, rp)
}

func (h *handlers) deleteRecurring(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := h.Payments.DeleteRecurring(c.Request.Context(), middleware.ActorOf(c), id); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *handlers) listOccurrences(c *gin.Context) {
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
	q := service.OccurrenceQuery{
		Owner: c.Query("owner"), Status: c.Query("status"), Currency: c.Query("currency"),
		From: from, To: to, Desc: c.Query("order") == "desc",
	}
	if v := c.Query("recurring_payment_id"); v != "" {
		id, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			fail(c, service.Invalid("recurring_payment_id must be a number"))
			return
		}
		q.RecurringID = uint(id)
	}
	if v := c.Query("limit"); v != "" {
		q.Limit, _ = strconv.Atoi(v)
	}
	rows, err := h.Payments.ListOccurrences(c.Request.Context(), middleware.ActorOf(c), q)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, nonNil(rows))
}

func (h *handlers) getOccurrence(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	v, err := h.Payments.GetOccurrence(c.Request.Context(), middleware.ActorOf(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *handlers) patchOccurrence(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var in service.UpdateOccurrenceInput
	if !bind(c, &in) {
		return
	}
	v, err := h.Payments.UpdateOccurrence(c.Request.Context(), middleware.ActorOf(c), id, in)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *handlers) summary(c *gin.Context) {
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
	rows, err := h.Payments.Summary(c.Request.Context(), middleware.ActorOf(c), c.Query("owner"), from, to)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"currencies": nonNil(rows)})
}

func (h *handlers) updateRecurring(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var in service.UpdateRecurringInput
	if !bind(c, &in) {
		return
	}
	rp, err := h.Payments.UpdateRecurring(c.Request.Context(), middleware.ActorOf(c), id, in)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, rp)
}

func (h *handlers) monthlySummary(c *gin.Context) {
	months, _ := strconv.Atoi(c.DefaultQuery("months", "12"))
	ahead, _ := strconv.Atoi(c.DefaultQuery("ahead", "1"))
	rows, err := h.Payments.MonthlySummary(c.Request.Context(), middleware.ActorOf(c), c.Query("owner"), months, ahead)
	if err != nil {
		fail(c, err)
		return
	}
	if rows == nil {
		rows = []models.MonthSummary{}
	}
	c.JSON(http.StatusOK, gin.H{"months": nonNil(rows), "today": h.Payments.Today().String()})
}

func (h *handlers) listNotifications(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "30"))
	out, err := h.Notes.List(c.Request.Context(), middleware.ActorOf(c), c.Query("unread") == "1", limit)
	if err != nil {
		fail(c, err)
		return
	}
	out.Items = nonNil(out.Items)
	c.JSON(http.StatusOK, out)
}

func (h *handlers) readNotifications(c *gin.Context) {
	var in struct {
		IDs []uint `json:"ids"`
	}
	_ = c.ShouldBindJSON(&in) // пустое тело — прочитать все
	if err := h.Notes.MarkRead(c.Request.Context(), middleware.ActorOf(c), in.IDs); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *handlers) clearNotifications(c *gin.Context) {
	if err := h.Notes.Clear(c.Request.Context(), middleware.ActorOf(c)); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
