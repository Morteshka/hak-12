// Package handler — HTTP-слой (Gin): разбор запросов, вызов сервисов, ответы.
package handler

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"invoicekeeper/internal/middleware"
	"invoicekeeper/internal/models"
	"invoicekeeper/internal/service"
)

type Deps struct {
	Auth     *service.AuthService
	Users    *service.UserService
	Payments *service.PaymentService
	Audit    *service.Audit
	Budgets  *service.BudgetService
	Settings *service.SettingsService
	Catalog  *service.CatalogService
	Receipts *service.ReceiptService
	Imports  *service.ImportService
	Notes    *service.NotificationService
	Notifier Notifier
	Loc      *time.Location
	Origins  []string
	// AllowRegistration — разрешена ли самостоятельная регистрация (всегда с ролью user).
	AllowRegistration bool
	// WebhookSecret — для проверки подписи во встроенном тестовом приёмнике.
	WebhookSecret string
}

// Notifier — отправка тестового уведомления (реализует notify.Hub).
type Notifier interface {
	Test(ctx context.Context, a service.Actor, u *models.User, channel string) error
	TestWebhook(ctx context.Context, a service.Actor, u *models.User, url string) (*models.WebhookDelivery, error)
	WebhookLog(ctx context.Context, a service.Actor, limit int) ([]models.WebhookDelivery, error)
	TestEmail(ctx context.Context, a service.Actor, u *models.User, to string) error
}

func NewRouter(d Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	// IP клиента берём из X-Forwarded-For только от своего nginx (сети Docker и localhost):
	// с внешнего адреса заголовок игнорируется, и подменить IP в журнале нельзя.
	_ = r.SetTrustedProxies([]string{"127.0.0.1", "::1", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"})
	r.Use(gin.Recovery(), gin.Logger(), middleware.SecureHeaders(), middleware.LimitBody(1<<20), middleware.CORS(d.Origins))

	v1 := r.Group("/api/v1")
	v1.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	h := &handlers{d}
	// Swagger и спецификация — только администратору (заголовок Authorization или кука ik_docs)
	v1.GET("/docs", h.docsPage)
	v1.GET("/docs/openapi.yml", h.docsSpec)

	// публичные
	v1.GET("/auth/config", h.authConfig)
	v1.GET("/time", h.timeState)            // «сегодня» в системе (с учётом машины времени)
	v1.POST("/webhook/echo", h.webhookEcho) // тестовый приёмник webhook: принимает событие и отвечает 200
	v1.POST("/auth/login", h.login)
	v1.POST("/auth/register", h.register)
	v1.POST("/auth/register/confirm", h.registerConfirm) // код из письма
	v1.POST("/auth/register/resend", h.registerResend)
	v1.POST("/auth/refresh", h.refresh)
	v1.POST("/auth/telegram", h.loginTelegram) // вход из Telegram Mini App (подпись initData)

	// только с авторизацией
	authed := v1.Group("", middleware.Auth(d.Auth))
	authed.GET("/auth/me", h.me)
	authed.PATCH("/auth/me", h.updateMe)
	authed.POST("/auth/logout", h.logout)
	authed.POST("/auth/password", h.changePassword)
	authed.POST("/auth/telegram/link", h.linkTelegramWebApp) // подключить Telegram, открытый как Mini App

	authed.GET("/recurring-payments", h.listRecurring)          // общие + свои подписки (админ — все; ?owner=shared|mine|<id>)
	authed.PATCH("/recurring-payments/:id", h.updateRecurring)  // администратор — всё; владелец подписки — автопродление
	authed.DELETE("/recurring-payments/:id", h.deleteRecurring) // общий — только админ; подписку — владелец или админ
	authed.GET("/payment-occurrences", h.listOccurrences)
	authed.GET("/payment-occurrences/:id", h.getOccurrence)
	authed.PATCH("/payment-occurrences/:id", h.patchOccurrence) // пользователь и админ
	// подтверждение оплаты чеком: загрузить, посмотреть свои чеки и файл
	authed.POST("/payment-occurrences/:id/receipts", h.uploadReceipt)
	authed.GET("/payment-occurrences/:id/receipts", h.occurrenceReceipts)
	authed.GET("/receipts", h.listReceipts) // админ — все, пользователь — свои
	authed.GET("/receipts/:id", h.getReceipt)
	authed.GET("/receipts/:id/file", h.receiptFile)
	authed.GET("/receipts/:id/history", h.receiptHistory)
	authed.GET("/payments/summary", h.summary)
	authed.GET("/payments/summary/months", h.monthlySummary) // итоги по месяцам и валютам
	authed.GET("/notifications", h.listNotifications)
	authed.POST("/notifications/read", h.readNotifications)
	authed.DELETE("/notifications", h.clearNotifications)
	authed.GET("/budgets", h.listBudgets)
	authed.GET("/budgets/status", h.budgetStatus)
	authed.POST("/budgets", h.upsertBudget) // бюджет личный — задаёт каждый себе
	authed.DELETE("/budgets/:id", h.deleteBudget)
	// каталог услуг: смотрит и подключает любой пользователь, управляет — администратор
	authed.GET("/services", h.listServices)
	authed.POST("/imports/statement", h.importStatement) // выписка банка → найденные регулярные списания
	authed.POST("/imports/confirm", h.importConfirm)     // добавить выбранные как платежи
	authed.POST("/services/:id/subscribe", h.subscribe)
	authed.GET("/settings", h.getSettings)
	authed.PATCH("/settings", h.patchSettings) // интеграции — только админ (проверка в сервисе)
	authed.POST("/settings/test-notification", h.testNotification)
	authed.POST("/settings/telegram/resend", h.resendTelegram) // повторить запрос «Это вы?» в Telegram

	// только администратор
	admin := authed.Group("", middleware.RequireRole(models.RoleAdmin))
	admin.POST("/recurring-payments", h.createRecurring)
	admin.PUT("/time", h.setTime)                                  // машина времени для демо
	admin.GET("/settings/webhook/deliveries", h.webhookDeliveries) // журнал отправок webhook
	admin.POST("/services", h.createService)
	admin.PATCH("/services/:id", h.updateService)
	admin.DELETE("/services/:id", h.deleteService)
	admin.POST("/receipts/:id/approve", h.approveReceipt)
	admin.POST("/receipts/:id/reject", h.rejectReceipt)
	admin.POST("/receipts/:id/recheck", h.recheckReceipt)
	admin.GET("/users", h.listUsers)
	admin.POST("/users", h.createUser)
	admin.PATCH("/users/:id", h.updateUser)
	admin.DELETE("/users/:id", h.deleteUser)
	admin.GET("/audit-logs", h.auditLogs)
	admin.GET("/logs", h.auditLogs) // синоним
	admin.GET("/audit-logs/actions", h.auditActions)
	return r
}

type handlers struct{ Deps }

// ---------- helpers ----------

func fail(c *gin.Context, err error) {
	if e, ok := service.AsError(err); ok {
		code := map[service.Kind]int{
			service.KindInvalid:      http.StatusUnprocessableEntity,
			service.KindUnauthorized: http.StatusUnauthorized,
			service.KindForbidden:    http.StatusForbidden,
			service.KindNotFound:     http.StatusNotFound,
			service.KindConflict:     http.StatusConflict,
		}[e.Kind]
		c.JSON(code, gin.H{"error": e.Msg})
		return
	}
	log.Printf("internal error: %v", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}

func bind(c *gin.Context, v any) bool {
	if err := c.ShouldBindJSON(v); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return false
	}
	return true
}

func idParam(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, false
	}
	return uint(id), true
}

func dateQuery(c *gin.Context, key string) (*models.Date, error) {
	s := c.Query(key)
	if s == "" {
		return nil, nil
	}
	d, err := models.ParseDate(s)
	if err != nil {
		return nil, service.Invalid(key + ": " + err.Error())
	}
	return &d, nil
}
