package notify

import (
	"context"
	"fmt"
	"html"
	"log"
	"strconv"
	"strings"

	"invoicekeeper/internal/models"
	"invoicekeeper/internal/service"
	"invoicekeeper/internal/store"
)

// ---------- ссылки на сайт ----------

// PaymentPath — страница платежа на сайте: открывает окно этого периода (там же чек).
func PaymentPath(occurrenceID uint) string {
	return "/payments?open=" + strconv.FormatUint(uint64(occurrenceID), 10)
}

// siteLink — полная ссылка для текста письма/сообщения (пусто, если адрес сайта не задан).
func (h *Hub) siteLink(path string) string {
	u := strings.TrimRight(h.cfg.PublicURL, "/")
	if u == "" {
		return ""
	}
	return u + path
}

// paymentButtons — кнопки под напоминанием: «Оплачено» и ссылка на платёж (если сайт доступен из интернета).
func (h *Hub) paymentButtons(v models.OccurrenceView, paidButton bool) [][]tgButton {
	var row []tgButton
	if paidButton {
		row = append(row, tgButton{Text: "✅ Оплачено", CallbackData: "paid:" + strconv.FormatUint(uint64(v.ID), 10)})
	}
	if u := h.publicLink(PaymentPath(v.ID)); u != "" {
		if strings.HasPrefix(u, "https://") { // сайт по HTTPS — платёж открывается прямо в Telegram, уже с входом
			row = append(row, tgButton{Text: "📱 Открыть платёж", WebApp: &tgWebApp{URL: u}})
		} else {
			row = append(row, tgButton{Text: "🔗 Открыть платёж", URL: u})
		}
	}
	if len(row) == 0 {
		return nil
	}
	return [][]tgButton{row}
}

// ---------- колокольчик ----------

// audience — кому показывать событие платежа: личный — владельцу, общий — администраторам.
func (h *Hub) audience(ctx context.Context, owner *uint) []uint {
	if owner != nil {
		return []uint{*owner}
	}
	// общий платёж (без владельца) ведёт администратор — уведомления о нём только ему
	ids, err := h.st.ActiveUserIDs(ctx, true)
	if err != nil {
		log.Printf("notify: users: %v", err)
	}
	return ids
}

func (h *Hub) inApp(ctx context.Context, users []uint, n models.Notification) {
	for _, id := range users {
		m := n
		m.ID, m.UserID = 0, id
		if err := h.st.CreateNotification(ctx, &m); err != nil {
			log.Printf("notify: in-app for user %d: %v", id, err)
		}
	}
}

func amountText(v models.OccurrenceView) string { return money(v.Amount.StringFixed(2), v.Currency) }

// reminderNotification — текст напоминания для колокольчика.
func reminderNotification(v models.OccurrenceView, today models.Date) models.Notification {
	level := "warn"
	if v.Status == models.StatusOverdue {
		level = "danger"
	}
	id := v.ID
	return models.Notification{
		Kind: "reminder", Level: level, OccurrenceID: &id,
		Title: "Оплатить «" + v.Title + "»",
		Body:  fmt.Sprintf("%s · срок %s · %s", amountText(v), v.DueDate.Format("02.01.2006"), service.DaysLeftRU(v, today)),
	}
}

// onPaymentEvent — колокольчик и сообщения по событию платежа (вызывается из Emit).
func (h *Hub) onPaymentEvent(ctx context.Context, ev models.PaymentEvent) {
	switch ev.Event {
	case models.EventPaymentPaid, models.EventPaymentUnpaid, models.EventPaymentDateFix:
	case models.EventPaymentCreated:
		rp, err := h.st.RecurringByID(ctx, ev.RecurringID)
		if err != nil {
			return
		}
		body := money(ev.Amount, ev.Currency) + " · первый срок " + fmtDate(ev.DueDate)
		if rp.Period == models.PeriodOnce {
			body += " · разовый платёж"
		}
		h.inApp(ctx, h.audience(ctx, rp.UserID), models.Notification{Kind: "created", Level: "info", Title: "Новый платёж «" + ev.Name + "»", Body: body})
		return
	default:
		return
	}
	v, err := h.st.OccurrenceByID(ctx, ev.PaymentID)
	if err != nil {
		return
	}
	id := v.ID
	n := models.Notification{OccurrenceID: &id}
	switch ev.Event {
	case models.EventPaymentPaid:
		n.Kind, n.Level, n.Title = "paid", "success", "Оплачено: «"+v.Title+"»"
		n.Body = amountText(*v) + " · " + paidText(v)
	case models.EventPaymentUnpaid:
		n.Kind, n.Level, n.Title = "unpaid", "warn", "Оплата отменена: «"+v.Title+"»"
		n.Body = amountText(*v) + " · срок " + v.DueDate.Format("02.01.2006") + " · отменил " + ev.Actor
	case models.EventPaymentDateFix:
		n.Kind, n.Level, n.Title = "paid", "info", "Исправлена дата оплаты «"+v.Title+"»"
		n.Body = amountText(*v) + " · " + paidText(v)
	}
	h.inApp(ctx, h.audience(ctx, v.UserID), n)
	if ev.Event == models.EventPaymentPaid {
		h.paidMessages(ctx, *v)
	}
}

func paidText(v *models.OccurrenceView) string {
	s := "оплачено"
	if v.PaidAt != nil {
		s += " " + v.PaidAt.Format("02.01.2006")
	}
	if v.PaidBy != nil && *v.PaidBy != "" {
		s += " (" + *v.PaidBy + ")"
	}
	return s
}

// paidMessages — оповещение о проведённой оплате в Telegram и на почту, со ссылкой на чек.
// Получатели те же, что у напоминаний: личный платёж — владельцу, общий — общему чату (или каждому лично) и на почту.
func (h *Hub) paidMessages(ctx context.Context, v models.OccurrenceView) {
	receipt := ""
	switch {
	case v.ReceiptURL != nil:
		receipt = *v.ReceiptURL
	case v.ReceiptStatus != nil && *v.ReceiptStatus == models.ReceiptApproved:
		receipt = h.siteLink(PaymentPath(v.ID)) // файл чека хранится на сайте, в окне платежа
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "✅ <b>Оплата проведена</b>\n\nПлатёж: <b>%s</b>\nСумма: <b>%s</b>\nСрок: %s",
		html.EscapeString(v.Title), amountText(v), v.DueDate.Format("02.01.2006"))
	if v.PaidAt != nil {
		fmt.Fprintf(&sb, "\nДата оплаты: <b>%s</b>", v.PaidAt.Format("02.01.2006"))
	}
	if v.PaidBy != nil && *v.PaidBy != "" {
		fmt.Fprintf(&sb, "\nОтметил: %s", html.EscapeString(*v.PaidBy))
	}
	if v.OverdueDays > 0 {
		fmt.Fprintf(&sb, "\n⚠️ С опозданием на %d дн.", v.OverdueDays)
	}
	if receipt != "" {
		fmt.Fprintf(&sb, "\n🧾 Чек: %s", html.EscapeString(receipt))
	} else {
		sb.WriteString("\n🧾 Чек не приложен")
	}
	text := sb.String()
	kb := h.paymentButtons(v, false)

	if h.cfg.TelegramToken != "" {
		var chats []int64
		eff, _, _ := service.EffectiveChannels(ctx, h.st, h.cfg.Env)
		if v.UserID == nil && eff.TelegramChatID != 0 {
			chats = append(chats, eff.TelegramChatID)
		} else if users, err := h.st.TelegramRecipients(ctx); err == nil {
			for _, u := range users {
				// своё — владельцу, общее (без владельца) — администраторам
				if (v.UserID == nil && u.Role == models.RoleAdmin) || (v.UserID != nil && u.ID == *v.UserID) {
					chats = append(chats, *u.TelegramID)
				}
			}
		}
		for _, c := range chats {
			if err := h.sendTelegramKB(ctx, c, text, kb); err != nil {
				log.Printf("notify: paid #%d to telegram: %s", v.ID, redact(err.Error(), h.cfg.TelegramToken))
			}
		}
	}
	if mailCfg := h.smtp(ctx); mailCfg.Configured() {
		users, err := h.st.EmailRecipients(ctx)
		if err != nil {
			return
		}
		body := paidEmail(v, receipt, h.cfg.PublicURL)
		for _, u := range users {
			if (v.UserID != nil && u.ID != *v.UserID) || (v.UserID == nil && u.Role != models.RoleAdmin) {
				continue
			}
			if err := mailCfg.Send(*u.Email, "Оплачено: "+v.Title, body); err != nil {
				log.Printf("notify: paid email to %s: %v", u.Login, err)
			}
		}
	}
}

// appReminders — напоминания в колокольчик: те же ступени, что у Telegram и почты (канал app:<id>).
func (h *Hub) appReminders(ctx context.Context, pay *service.PaymentService) {
	ids, err := h.st.ActiveUserIDs(ctx, false)
	if err != nil {
		log.Printf("notify: users: %v", err)
		return
	}
	admins := map[uint]bool{}
	if as, err := h.st.ActiveUserIDs(ctx, true); err == nil {
		for _, a := range as {
			admins[a] = true
		}
	}
	today := pay.Today()
	for _, id := range ids {
		uid := id
		channel := "app:" + strconv.FormatUint(uint64(uid), 10)
		err := pay.ProcessChannelReminders(ctx, channel, store.Scope{UserID: uid, WithShared: admins[uid]}, func(v models.OccurrenceView) error {
			n := reminderNotification(v, today)
			n.UserID = uid
			return h.st.CreateNotification(ctx, &n)
		})
		if err != nil {
			log.Printf("notify: in-app reminders for user %d: %v", uid, err)
		}
	}
}
