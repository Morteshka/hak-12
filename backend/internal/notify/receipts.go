package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"strconv"
	"strings"
	"time"

	"invoicekeeper/internal/models"
	"invoicekeeper/internal/service"
)

// ReceiptForReview — администраторам в Telegram: файл чека, что распознано, замечания и кнопки решения.
// Если ни один администратор не подтвердил Telegram — в общий чат.
func (h *Hub) ReceiptForReview(r *models.Receipt, file []byte) {
	go func() { // колокольчик администраторов
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ids, err := h.st.ActiveUserIDs(ctx, true)
		if err != nil {
			return
		}
		rid := r.ID
		h.inApp(ctx, ids, models.Notification{Kind: "receipt", Level: "warn", ReceiptID: &rid, OccurrenceID: r.OccurrenceID,
			Title: "Чек на проверке: «" + r.PaymentTitle + "»",
			Body:  money(r.PaymentAmount.StringFixed(2), r.PaymentCurrency) + " · от " + r.SubmittedBy})
	}()
	if h.cfg.TelegramToken == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		chats, err := h.st.AdminTelegramChats(ctx)
		if err != nil {
			log.Printf("notify: admin chats: %v", err)
		}
		if len(chats) == 0 {
			if eff, _, err := service.EffectiveChannels(ctx, h.st, h.cfg.Env); err == nil && eff.TelegramChatID != 0 {
				chats = []int64{eff.TelegramChatID}
			}
		}
		id := strconv.FormatUint(uint64(r.ID), 10)
		kb := [][]tgButton{{{Text: "✅ Подтвердить", CallbackData: "rcpt:ok:" + id}, {Text: "❌ Отклонить", CallbackData: "rcpt:no:" + id}}}
		if u := h.publicLink("/reviews"); u != "" {
			kb = append(kb, []tgButton{{Text: "Открыть на сайте", URL: u}})
		}
		caption := ReceiptCaption(r)
		photo := r.Mime == "image/jpeg" || r.Mime == "image/png"
		for _, chat := range chats {
			err := h.sendTelegramFile(ctx, chat, photo, r.FileName, file, caption, kb)
			if err != nil && photo { // Telegram не принял фото (размер/пропорции) — шлём документом
				err = h.sendTelegramFile(ctx, chat, false, r.FileName, file, caption, kb)
			}
			if err != nil {
				log.Printf("notify: receipt #%d to %d: %s", r.ID, chat, redact(err.Error(), h.cfg.TelegramToken))
				h.audit.Log(ctx, system, service.ActNotifyFailed, "receipt", r.ID, service.D{"channel": "telegram", "error": err.Error()})
			}
		}
	}()
}

// ReceiptDecided — пользователю: администратор подтвердил или отклонил его чек.
func (h *Hub) ReceiptDecided(r *models.Receipt) {
	if r.UserID == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		u, err := h.st.UserByID(ctx, *r.UserID)
		if err != nil {
			return
		}
		rid := r.ID
		n := models.Notification{Kind: "receipt", ReceiptID: &rid, OccurrenceID: r.OccurrenceID, Body: money(r.PaymentAmount.StringFixed(2), r.PaymentCurrency)}
		if r.Status == models.ReceiptApproved {
			n.Level, n.Title = "success", "Чек принят: «"+r.PaymentTitle+"»"
		} else {
			n.Level, n.Title = "danger", "Чек отклонён: «"+r.PaymentTitle+"»"
			n.Body += " · " + r.DecisionComment
		}
		h.inApp(ctx, []uint{u.ID}, n)
		title := html.EscapeString(r.PaymentTitle)
		var text string
		var kb [][]tgButton
		if r.Status == models.ReceiptApproved {
			text = fmt.Sprintf("✅ <b>Оплата подтверждена</b>\n«%s» — %s\nЧек проверил администратор.", title, money(r.PaymentAmount.StringFixed(2), r.PaymentCurrency))
		} else {
			text = fmt.Sprintf("❌ <b>Чек отклонён</b>\n«%s» — %s\nПричина: %s\n\nПришлите другой чек — кнопкой ниже или на сайте.",
				title, money(r.PaymentAmount.StringFixed(2), r.PaymentCurrency), html.EscapeString(r.DecisionComment))
			if r.OccurrenceID != nil {
				kb = [][]tgButton{{{Text: "📎 Прислать другой чек", CallbackData: "paid:" + strconv.FormatUint(uint64(*r.OccurrenceID), 10)}}}
			}
		}
		if u.TelegramID != nil && u.TelegramVerifiedAt != nil && h.cfg.TelegramToken != "" {
			if err := h.sendTelegramKB(ctx, *u.TelegramID, text, kb); err != nil {
				log.Printf("notify: receipt decision to %s: %s", u.Login, redact(err.Error(), h.cfg.TelegramToken))
			}
		}
		if mailCfg := h.smtp(ctx); u.EmailNotifications && u.Email != nil && mailCfg.Configured() {
			subj := "Оплата подтверждена: " + r.PaymentTitle
			if r.Status != models.ReceiptApproved {
				subj = "Чек отклонён: " + r.PaymentTitle
			}
			body := wrapHTML("<p>"+strings.ReplaceAll(text, "\n", "<br>")+"</p>", h.cfg.PublicURL)
			if err := mailCfg.Send(*u.Email, subj, body); err != nil {
				log.Printf("notify: receipt decision email to %s: %v", u.Login, err)
			}
		}
	}()
}

// ReceiptCaption — подпись к чеку для администратора (до 1024 символов — лимит Telegram).
func ReceiptCaption(r *models.Receipt) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "🧾 <b>Чек на проверку</b> #%d\n", r.ID)
	due := ""
	if r.PaymentDueDate != nil {
		due = ", срок " + r.PaymentDueDate.Format("02.01.2006")
	}
	fmt.Fprintf(&sb, "«%s» — %s%s\n", html.EscapeString(r.PaymentTitle), money(r.PaymentAmount.StringFixed(2), r.PaymentCurrency), due)
	src := map[string]string{"web": "сайт", "telegram": "Telegram"}[r.Source]
	fmt.Fprintf(&sb, "От: <b>%s</b> (%s)\n", html.EscapeString(r.SubmittedBy), src)
	var got []string
	if r.ExtAmount != nil {
		cur := r.PaymentCurrency
		if r.ExtCurrency != nil {
			cur = *r.ExtCurrency
		}
		got = append(got, money(r.ExtAmount.StringFixed(2), cur))
	}
	if r.ExtDate != nil {
		got = append(got, r.ExtDate.Format("02.01.2006"))
	}
	if len(got) > 0 {
		fmt.Fprintf(&sb, "В чеке: %s\n", strings.Join(got, " · "))
	}
	var checks []models.ReceiptCheck
	_ = json.Unmarshal(r.Checks, &checks)
	var issues []string
	for _, c := range checks {
		switch c.Status {
		case models.CheckFail:
			issues = append(issues, "❌ "+html.EscapeString(c.Title+": "+c.Detail))
		case models.CheckWarn:
			issues = append(issues, "⚠️ "+html.EscapeString(c.Title+": "+c.Detail))
		}
	}
	if len(issues) == 0 {
		sb.WriteString("\nВсе автопроверки пройдены — включён режим ручного подтверждения.")
	} else {
		sb.WriteString("\n" + strings.Join(issues, "\n"))
	}
	if r.Note != "" {
		fmt.Fprintf(&sb, "\n\n💬 %s", html.EscapeString(r.Note))
	}
	out := sb.String()
	if len([]rune(out)) > 1000 {
		out = string([]rune(out)[:1000]) + "…"
	}
	return out
}

// money — «85 000,00 ₽».
func money(amount, cur string) string { return service.MoneyText(amount, cur) }

// publicLink — ссылка на сайт для кнопки (Telegram не принимает localhost-адреса).
func (h *Hub) publicLink(path string) string {
	u := strings.TrimRight(h.cfg.PublicURL, "/")
	if u == "" || strings.Contains(u, "localhost") || strings.Contains(u, "127.0.0.1") {
		return ""
	}
	return u + path
}
