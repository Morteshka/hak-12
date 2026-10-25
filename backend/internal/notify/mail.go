package notify

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"invoicekeeper/internal/models"
	"invoicekeeper/internal/service"
)

// smtp — текущий почтовый сервер: из .env или из «Настройки → Почта» (читается при каждой рассылке,
// поэтому сохранённые в панели настройки начинают работать сразу, без перезапуска).
func (h *Hub) smtp(ctx context.Context) SMTPConfig {
	mc, err := service.EffectiveMail(ctx, h.st, h.cfg.Env)
	if err != nil {
		log.Printf("notify: smtp settings: %v", err)
		return SMTPConfig{}
	}
	return SMTPConfig{Host: mc.Host, Port: mc.Port, User: mc.User, Password: mc.Password, From: mc.From}
}

// MailConfigured — почта подключена (реализует service.Mailer: код подтверждения при регистрации).
func (h *Hub) MailConfigured(ctx context.Context) bool { return h.smtp(ctx).Configured() }

// SendMail — письмо в фирменном оформлении. Ошибка — понятным текстом на русском.
func (h *Hub) SendMail(ctx context.Context, to, subject, htmlBody string) error {
	c := h.smtp(ctx)
	if !c.Configured() {
		return errors.New("почта не подключена")
	}
	if err := c.Send(to, subject, wrapHTML(htmlBody, h.cfg.PublicURL)); err != nil {
		return errors.New(humanMailError(err, c))
	}
	return nil
}

// TestEmail — тестовое письмо (кнопка «Отправить тестовое письмо»). Администратор может указать любой адрес.
func (h *Hub) TestEmail(ctx context.Context, a service.Actor, u *models.User, to string) error {
	c := h.smtp(ctx)
	if !c.Configured() {
		return service.Invalid("SMTP is not configured (SMTP_HOST, SMTP_FROM in .env)")
	}
	to = strings.TrimSpace(to)
	if to == "" {
		if u.Email == nil {
			return service.Invalid("set email to receive email reminders")
		}
		to = *u.Email
	}
	if (u.Email == nil || !strings.EqualFold(to, *u.Email)) && a.Role != models.RoleAdmin {
		return service.Forbidden("admin role required")
	}
	start := time.Now()
	body := `<p style="font-size:16px;margin:0 0 12px">✅ <b>Почта подключена</b></p>
<p style="margin:0 0 10px">Это тестовое письмо Invoice Keeper. Напоминания о платежах будут приходить так же: за неделю, за 3 дня и за сутки до срока, а также если оплата просрочена.</p>
<p style="margin:0;color:#6b6b6b">Сервер: ` + c.Host + fmt.Sprintf(":%d", c.Port) + `</p>`
	if err := c.Send(to, "Invoice Keeper: почта подключена", wrapHTML(body, h.cfg.PublicURL)); err != nil {
		log.Printf("notify: test email to %s via %s:%d failed after %s: %v", to, c.Host, c.Port, time.Since(start).Round(time.Millisecond), err)
		return service.Invalid("email failed: " + humanMailError(err, c))
	}
	return nil
}

// humanMailError — частые ошибки SMTP простыми словами.
func humanMailError(err error, c SMTPConfig) string {
	msg := err.Error()
	low := strings.ToLower(msg)
	var ne net.Error
	var dnsErr *net.DNSError
	switch {
	case strings.HasPrefix(msg, "brevo"):
		return humanBrevoError(msg)
	case strings.Contains(msg, "535") || strings.Contains(low, "authentication") || strings.Contains(low, "auth") && strings.Contains(low, "fail"):
		return "неверный логин или пароль. Для Яндекса, Mail.ru и Gmail нужен пароль приложения, а не обычный пароль от почты"
	case errors.As(err, &dnsErr):
		return fmt.Sprintf("почтовый сервер %s не найден — проверьте адрес", c.Host)
	case errors.As(err, &ne) && ne.Timeout(), strings.Contains(low, "timeout"):
		return fmt.Sprintf("нет ответа от %s:%d — похоже, хостинг закрыл исходящие почтовые порты. Выберите «Brevo (HTTPS)»: он отправляет письма через обычный HTTPS и работает на любом хостинге", c.Host, c.Port)
	case strings.Contains(low, "connection refused"):
		return fmt.Sprintf("%s:%d не принимает подключения — проверьте порт (обычно 465)", c.Host, c.Port)
	case strings.Contains(low, "certificate") || strings.Contains(low, "tls") || strings.Contains(low, "first record does not look like"):
		return "ошибка шифрования: для порта 465 нужен SSL, для 587 — STARTTLS. Попробуйте порт 465"
	case strings.HasPrefix(msg, "550") || strings.HasPrefix(msg, "553") || strings.HasPrefix(msg, "554") || strings.Contains(msg, " 55"):
		return "сервер отказался отправить письмо. Обычно адрес «От кого» должен совпадать с логином почты (" + msg + ")"
	}
	return "ошибка почтового сервера: " + msg
}
