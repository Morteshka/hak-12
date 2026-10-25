package service

import (
	"fmt"
	"html"
	"strings"

	"invoicekeeper/internal/models"
)

// plural — 1 день, 2 дня, 5 дней.
func plural(n int, one, few, many string) string {
	m10, m100 := n%10, n%100
	switch {
	case m10 == 1 && m100 != 11:
		return one
	case m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14):
		return few
	}
	return many
}

// DaysLeftRU — «Осталось 3 дня», «Срок сегодня», «Просрочено на 2 дня».
func DaysLeftRU(v models.OccurrenceView, today models.Date) string {
	n := today.DaysUntil(v.DueDate)
	switch {
	case n == 0:
		return "Срок оплаты — сегодня"
	case n == 1:
		return "Остался 1 день"
	case n > 1:
		return fmt.Sprintf("%s %d %s", plural(n, "Остался", "Осталось", "Осталось"), n, plural(n, "день", "дня", "дней"))
	}
	n = -n
	return fmt.Sprintf("Просрочено на %d %s", n, plural(n, "день", "дня", "дней"))
}

// ReminderHTML — текст напоминания для Telegram (HTML): название, сумма, валюта, дата и сколько осталось.
// test=true — пометка «тестовое напоминание» (кнопка «Проверить» в профиле).
func ReminderHTML(v models.OccurrenceView, today models.Date, test bool) string {
	var sb strings.Builder
	if test {
		sb.WriteString("🧪 <i>Тестовое напоминание — так выглядит настоящее</i>\n\n")
	}
	left := DaysLeftRU(v, today)
	head := "🔔 <b>Напоминание об оплате</b>"
	switch n := today.DaysUntil(v.DueDate); {
	case n < 0:
		head = "⚠️ <b>Оплата просрочена</b>"
	case n == 0:
		head = "⏰ <b>Сегодня срок оплаты</b>"
	}
	if today.DaysUntil(v.DueDate) == 0 { // «сегодня» уже в заголовке
		fmt.Fprintf(&sb, "%s\n\n", head)
	} else {
		fmt.Fprintf(&sb, "%s\n%s\n\n", head, left)
	}
	fmt.Fprintf(&sb, "Платёж: <b>%s</b>\n", html.EscapeString(v.Title))
	fmt.Fprintf(&sb, "Сумма: <b>%s</b>\n", MoneyText(v.Amount.StringFixed(2), v.Currency))
	fmt.Fprintf(&sb, "Срок оплаты: <b>%s</b>", v.DueDate.Format("02.01.2006"))
	switch {
	case v.UserID != nil && v.TrialUntil != nil && v.TrialUntil.Time.Equal(v.DueDate.Time):
		sb.WriteString("\nПробный период заканчивается — это первый платёж по подписке.")
	case v.UserID != nil && v.ServiceID != nil:
		sb.WriteString("\nВаша подписка")
	case v.UserID != nil:
		sb.WriteString("\nВаш личный платёж")
	}
	if v.ReceiptURL != nil {
		fmt.Fprintf(&sb, "\nЧек: %s", html.EscapeString(*v.ReceiptURL))
	}
	return sb.String()
}

// MoneyText — сумма для сообщений: «85 000,00 ₽», «15,00 $». Валюты не конвертируются.
func MoneyText(amount, cur string) string {
	whole, frac, _ := strings.Cut(amount, ".")
	neg := strings.HasPrefix(whole, "-")
	whole = strings.TrimPrefix(whole, "-")
	var b strings.Builder
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteString("\u00a0")
		}
		b.WriteRune(r)
	}
	out := b.String()
	if neg {
		out = "-" + out
	}
	if frac != "" {
		out += "," + frac
	}
	sign := map[string]string{"RUB": "₽", "USD": "$", "EUR": "€"}[cur]
	if sign == "" {
		sign = cur
	}
	return out + "\u00a0" + sign
}
