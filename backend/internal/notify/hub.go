// Package notify — дополнительные каналы уведомлений: email (SMTP) и webhook.
// Telegram живёт в отдельном пакете/процессе (internal/telegram).
package notify

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"invoicekeeper/internal/models"
	"invoicekeeper/internal/service"
	"invoicekeeper/internal/store"
)

type Config struct {
	WebhookSecret string // если задан — тело подписывается HMAC-SHA256 (заголовок X-InvoiceKeeper-Signature)
	PublicURL     string // адрес веб-панели для ссылок в письмах
	Env           service.ChannelEnv
	TelegramAPI   string // свой адрес Bot API вместо api.telegram.org (формат https://host/bot%s/%s); пусто — стандартный
	TelegramToken string // для личных напоминаний пользователям с подтверждённым Telegram
	// InternalURL — адрес API внутри сети Docker (http://backend:8080): туда уходят события
	// для встроенного тестового приёмника /api/v1/webhook/echo. Пусто — отправка по адресу как есть.
	InternalURL string
}

// Hub рассылает события и напоминания по каналам. Ошибки каналов не влияют на основной процесс:
// они пишутся в лог и в журнал действий (notification_failed).
type Hub struct {
	st     *store.Store
	audit  *service.Audit
	cfg    Config
	client *http.Client
	pay    *service.PaymentService
}

// UsePayments — для тестового напоминания (берётся ближайший неоплаченный платёж пользователя).
func (h *Hub) UsePayments(p *service.PaymentService) { h.pay = p }

func New(st *store.Store, audit *service.Audit, cfg Config) *Hub {
	return &Hub{st: st, audit: audit, cfg: cfg, client: &http.Client{Timeout: 5 * time.Second}}
}

var system = service.Actor{Login: "system"}

// Emit — событие платежа (создание, оплата, отмена, смена даты). Асинхронно: API не ждёт внешние сервисы.
func (h *Hub) Emit(ev models.PaymentEvent) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		eff, _, err := service.EffectiveChannels(ctx, h.st, h.cfg.Env)
		if err != nil {
			log.Printf("notify: settings: %v", err)
			return
		}
		if eff.WebhookURL != "" {
			h.report(ctx, "webhook", ev, h.postWebhook(ctx, eff.WebhookURL, ev))
		}
		h.onPaymentEvent(ctx, ev) // колокольчик и «Оплата проведена» в Telegram/на почту
	}()
}

func (h *Hub) report(ctx context.Context, channel string, ev models.PaymentEvent, err error) {
	if err == nil {
		log.Printf("notify: %s %s #%d delivered", channel, ev.Event, ev.PaymentID)
		return
	}
	log.Printf("notify: %s %s #%d failed: %v", channel, ev.Event, ev.PaymentID, err)
	h.audit.Log(ctx, system, service.ActNotifyFailed, "payment_occurrence", ev.PaymentID,
		service.D{"channel": channel, "event": ev.Event, "error": err.Error()})
}

// RunReminders — напоминания за 7, 3 и 1 день до срока и при просрочке: письмо, личный Telegram, колокольчик и webhook.
// Правила те же, что у общего чата: без дублей, оплаченные/удалённые не напоминают, поздно добавленные — сразу.
//   - email — общие платежи и свои подписки;
//   - личный Telegram — свои подписки (и общие платежи, если общий чат не настроен);
//   - webhook — все платежи.
func (h *Hub) RunReminders(ctx context.Context, pay *service.PaymentService) {
	eff, _, err := service.EffectiveChannels(ctx, h.st, h.cfg.Env)
	if err != nil {
		log.Printf("notify: settings: %v", err)
		return
	}
	if mailCfg := h.smtp(ctx); mailCfg.Configured() {
		users, err := h.st.EmailRecipients(ctx)
		if err != nil {
			log.Printf("notify: email recipients: %v", err)
		}
		for _, u := range users {
			u := u
			channel := "email:" + strconv.FormatUint(uint64(u.ID), 10)
			err := pay.ProcessChannelReminders(ctx, channel, store.Scope{UserID: u.ID, WithShared: u.Role == models.RoleAdmin}, func(v models.OccurrenceView) error {
				subj, body := reminderEmail(v, h.cfg.PublicURL, pay.Today())
				return mailCfg.Send(*u.Email, subj, body)
			})
			if err != nil {
				log.Printf("notify: email reminders for %s: %v", u.Login, err)
			}
		}
	}
	if h.cfg.TelegramToken != "" {
		users, err := h.st.TelegramRecipients(ctx)
		if err != nil {
			log.Printf("notify: telegram recipients: %v", err)
		}
		for _, u := range users {
			u := u
			sc := store.Scope{UserID: u.ID, Owner: "mine"}
			if eff.TelegramChatID == 0 && u.Role == models.RoleAdmin {
				sc = store.Scope{UserID: u.ID, WithShared: true} // общего чата нет — общие платежи приходят администратору лично
			}
			channel := "telegram:" + strconv.FormatUint(uint64(u.ID), 10)
			today := pay.Today()
			err := pay.ProcessChannelReminders(ctx, channel, sc, func(v models.OccurrenceView) error {
				return h.sendTelegramKB(ctx, *u.TelegramID, service.ReminderHTML(v, today, false)+h.linkLine(v), h.paymentButtons(v, true))
			})
			if err != nil {
				log.Printf("notify: telegram reminders for %s: %v", u.Login, err)
			}
		}
	}
	h.appReminders(ctx, pay) // колокольчик в веб-панели
	if eff.WebhookURL != "" {
		err := pay.ProcessChannelReminders(ctx, "webhook", store.Scope{All: true}, func(v models.OccurrenceView) error {
			return h.postWebhook(ctx, eff.WebhookURL, service.EventFor(models.EventPaymentReminder, &v, "system"))
		})
		if err != nil {
			log.Printf("notify: webhook reminders: %v", err)
		}
	}
}

// Test отправляет тестовое уведомление в канал (кнопка «Проверить» в настройках).
func (h *Hub) Test(ctx context.Context, a service.Actor, u *models.User, channel string) error {
	ev := models.PaymentEvent{
		Event: models.EventTest, Name: "Тестовое уведомление Invoice Keeper", Amount: "0.00", Currency: "RUB",
		DueDate: time.Now().Format(models.DateLayout), Status: models.StatusUnpaid, Actor: u.Login, Timestamp: time.Now().UTC(),
	}
	eff, _, err := service.EffectiveChannels(ctx, h.st, h.cfg.Env)
	if err != nil {
		return err
	}
	switch channel {
	case "telegram":
		return h.testTelegram(ctx, a, u)
	case "email":
		if u.Email == nil {
			return service.Invalid("set email to receive email reminders")
		}
		return h.TestEmail(ctx, a, u, *u.Email)
	case "webhook":
		if a.Role != models.RoleAdmin {
			return service.Forbidden("admin role required")
		}
		if eff.WebhookURL == "" {
			return service.Invalid("webhook url is not set")
		}
		if err := h.postWebhook(ctx, eff.WebhookURL, ev); err != nil {
			return service.Invalid("webhook failed: " + humanError(err, 0))
		}
	default:
		return service.Invalid("channel must be email, telegram or webhook")
	}
	return nil
}

// TestWebhook отправляет тестовое событие на адрес из поля (или сохранённый, если поле пустое)
// и возвращает результат отправки — его показывает «Настройки → Webhook». Только администратор.
func (h *Hub) TestWebhook(ctx context.Context, a service.Actor, u *models.User, target string) (*models.WebhookDelivery, error) {
	if a.Role != models.RoleAdmin {
		return nil, service.Forbidden("admin role required")
	}
	target = strings.TrimSpace(target)
	if target == "" {
		eff, _, err := service.EffectiveChannels(ctx, h.st, h.cfg.Env)
		if err != nil {
			return nil, err
		}
		target = eff.WebhookURL
	}
	if target == "" {
		return nil, service.Invalid("webhook url is not set")
	}
	if pu, err := url.Parse(target); err != nil || (pu.Scheme != "http" && pu.Scheme != "https") || pu.Host == "" {
		return nil, service.Invalid("webhook_url must be a valid http(s) link")
	}
	now := time.Now()
	ev := models.PaymentEvent{
		Event: models.EventTest, Name: "Тестовое событие Invoice Keeper", Amount: "1990.00", Currency: "RUB",
		DueDate: now.AddDate(0, 0, 3).Format(models.DateLayout), Status: models.StatusUnpaid, Actor: u.Login, Timestamp: now.UTC(),
	}
	d, _ := h.deliverWebhook(ctx, target, ev)
	if d == nil {
		return nil, service.Invalid("webhook failed")
	}
	return d, nil
}

// WebhookLog — последние отправки на webhook (журнал в настройках). Только администратор.
func (h *Hub) WebhookLog(ctx context.Context, a service.Actor, limit int) ([]models.WebhookDelivery, error) {
	if a.Role != models.RoleAdmin {
		return nil, service.Forbidden("admin role required")
	}
	list, err := h.st.WebhookDeliveries(ctx, limit)
	if list == nil {
		list = []models.WebhookDelivery{}
	}
	return list, err
}

// testTelegram присылает в личный Telegram пользователя напоминание по его ближайшему неоплаченному
// платежу — в точности как настоящее, с кнопкой «Оплачено». Отметка о доставке не ставится:
// плановое напоминание за 3 дня всё равно придёт.
func (h *Hub) testTelegram(ctx context.Context, a service.Actor, u *models.User) error {
	if h.cfg.TelegramToken == "" {
		return service.Invalid("telegram bot is not configured (TELEGRAM_BOT_TOKEN in .env)")
	}
	if u.TelegramID == nil || u.TelegramVerifiedAt == nil {
		return service.Invalid("confirm your telegram first")
	}
	var (
		text string
		id   uint
		v    *models.OccurrenceView
	)
	if h.pay != nil {
		for _, owner := range []string{"mine", "shared", ""} {
			list, err := h.pay.Upcoming(ctx, a, owner, 1)
			if err != nil {
				return err
			}
			if len(list) > 0 {
				v = &list[0]
				break
			}
		}
	}
	if v != nil {
		text, id = service.ReminderHTML(*v, h.pay.Today(), true)+h.linkLine(*v), v.ID
	} else {
		text = "🧪 <b>Тестовое напоминание</b>\n\nTelegram подключён: напоминания об оплате будут приходить сюда за неделю, за 3 дня и за сутки до срока. Сейчас неоплаченных платежей нет — подключите услугу в каталоге."
	}
	if err := h.sendTelegram(ctx, *u.TelegramID, text, id); err != nil {
		return service.Invalid("telegram failed: " + err.Error())
	}
	details := service.D{"channel": "telegram"}
	if v != nil {
		details["payment"] = v.Title
		details["due_date"] = v.DueDate.String()
	}
	h.audit.Log(ctx, a, service.ActTestReminder, "user", u.ID, details)
	return nil
}

func statusRU(s string) string {
	switch s {
	case models.StatusPaid:
		return "оплачено"
	case models.StatusOverdue:
		return "просрочено"
	}
	return "не оплачено"
}

func fmtDate(iso string) string {
	if d, err := models.ParseDate(iso); err == nil {
		return d.Format("02.01.2006")
	}
	return iso
}

func eventTitle(ev models.PaymentEvent) string {
	switch ev.Event {
	case models.EventPaymentCreated:
		return "Новый регулярный платёж"
	case models.EventPaymentReminder:
		return "Напоминание об оплате"
	case models.EventPaymentPaid:
		return "Платёж оплачен"
	case models.EventPaymentUnpaid:
		return "Оплата отменена"
	case models.EventPaymentDateFix:
		return "Исправлена дата оплаты"
	case models.EventTest:
		return "Тестовое уведомление"
	}
	return fmt.Sprintf("Событие %s", ev.Event)
}

// linkLine — строка со ссылкой на платёж для текста сообщения (если адрес сайта задан).
// Если сайт доступен из интернета, ссылка уже есть кнопкой «Открыть платёж» — в тексте её не дублируем.
func (h *Hub) linkLine(v models.OccurrenceView) string {
	if h.publicLink(PaymentPath(v.ID)) != "" {
		return ""
	}
	if u := h.siteLink(PaymentPath(v.ID)); u != "" {
		return "\n\n🔗 Оплатить и отметить: " + u
	}
	return ""
}
