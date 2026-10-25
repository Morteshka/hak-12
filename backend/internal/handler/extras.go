package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"invoicekeeper/internal/middleware"
	"invoicekeeper/internal/service"
)

// ---------- бюджет ----------

func (h *handlers) listBudgets(c *gin.Context) {
	list, err := h.Budgets.List(c.Request.Context(), middleware.ActorOf(c), c.Query("period"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, nonNil(list))
}

func (h *handlers) upsertBudget(c *gin.Context) {
	var in service.BudgetInput
	if !bind(c, &in) {
		return
	}
	b, err := h.Budgets.Upsert(c.Request.Context(), middleware.ActorOf(c), in)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, b)
}

func (h *handlers) deleteBudget(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := h.Budgets.Delete(c.Request.Context(), middleware.ActorOf(c), id); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// budgetStatus — план/факт/прогноз. ?period=month|year&date=YYYY-MM-DD (любая дата внутри периода).
func (h *handlers) budgetStatus(c *gin.Context) {
	at, err := dateQuery(c, "date")
	if err != nil {
		fail(c, err)
		return
	}
	st, err := h.Budgets.Status(c.Request.Context(), middleware.ActorOf(c), c.Query("period"), at)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, st)
}

// ---------- настройки ----------

func (h *handlers) getSettings(c *gin.Context) {
	s, err := h.Settings.Get(c.Request.Context(), middleware.CurrentUser(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h *handlers) patchSettings(c *gin.Context) {
	var in service.UpdateSettingsInput
	if !bind(c, &in) {
		return
	}
	s, err := h.Settings.Update(c.Request.Context(), middleware.ActorOf(c), middleware.CurrentUser(c), in)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h *handlers) resendTelegram(c *gin.Context) {
	s, err := h.Settings.ResendTelegramRequest(c.Request.Context(), middleware.CurrentUser(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h *handlers) testNotification(c *gin.Context) {
	var in struct {
		Channel string `json:"channel" binding:"required"`
		URL     string `json:"url"` // webhook: проверить адрес из поля, не сохраняя его
		To      string `json:"to"`  // email: куда отправить тестовое письмо (другой адрес — только админ)
	}
	if !bind(c, &in) {
		return
	}
	if in.Channel == "email" && strings.TrimSpace(in.To) != "" {
		if err := h.Notifier.TestEmail(c.Request.Context(), middleware.ActorOf(c), middleware.CurrentUser(c), in.To); err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "sent"})
		return
	}
	if in.Channel == "webhook" {
		d, err := h.Notifier.TestWebhook(c.Request.Context(), middleware.ActorOf(c), middleware.CurrentUser(c), in.URL)
		if err != nil {
			fail(c, err)
			return
		}
		status := "sent"
		if !d.OK {
			status = "failed"
		}
		c.JSON(http.StatusOK, gin.H{"status": status, "delivery": d})
		return
	}
	if err := h.Notifier.Test(c.Request.Context(), middleware.ActorOf(c), middleware.CurrentUser(c), in.Channel); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "sent"})
}

// ---------- webhook ----------

// webhookDeliveries — журнал отправок на webhook (последние 200), только администратор.
func (h *handlers) webhookDeliveries(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	list, err := h.Notifier.WebhookLog(c.Request.Context(), middleware.ActorOf(c), limit)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

// webhookEcho — встроенный тестовый приёмник: принимает событие и отвечает 200.
// Если задан WEBHOOK_SECRET, проверяет подпись X-InvoiceKeeper-Signature — как это должен делать настоящий приёмник.
func (h *handlers) webhookEcho(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 64<<10))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "cannot read body"})
		return
	}
	var ev struct {
		Event string `json:"event"`
	}
	if json.Unmarshal(body, &ev) != nil || ev.Event == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "expected Invoice Keeper event JSON"})
		return
	}
	sig := "not_configured"
	if h.WebhookSecret != "" {
		m := hmac.New(sha256.New, []byte(h.WebhookSecret))
		m.Write(body)
		want := "sha256=" + hex.EncodeToString(m.Sum(nil))
		if !hmac.Equal([]byte(want), []byte(c.GetHeader("X-InvoiceKeeper-Signature"))) {
			c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "error": "invalid signature"})
			return
		}
		sig = "valid"
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "received": ev.Event, "signature": sig, "message": "Событие получено тестовым приёмником Invoice Keeper"})
}
