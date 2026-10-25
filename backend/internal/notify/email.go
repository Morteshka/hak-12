package notify

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"html"
	"mime"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"invoicekeeper/internal/models"
	"invoicekeeper/internal/service"
)

type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
}

func (c SMTPConfig) Configured() bool { return c.Host != "" && c.From != "" }

// Send — письмо в HTML. Порт 465 — сразу TLS, иначе (587/25) — STARTTLS, если сервер его поддерживает.
func (c SMTPConfig) Send(to, subject, htmlBody string) error {
	addr := net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	tlsCfg := &tls.Config{ServerName: c.Host, MinVersion: tls.VersionTLS12}

	var conn net.Conn
	var err error
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	if c.Port == 465 {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsCfg)
	} else {
		conn, err = dialer.Dial("tcp", addr)
	}
	if err != nil {
		return err
	}
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	cl, err := smtp.NewClient(conn, c.Host)
	if err != nil {
		conn.Close()
		return err
	}
	defer cl.Close()

	if c.Port != 465 {
		if ok, _ := cl.Extension("STARTTLS"); ok {
			if err := cl.StartTLS(tlsCfg); err != nil {
				return err
			}
		}
	}
	if c.User != "" {
		if err := cl.Auth(smtp.PlainAuth("", c.User, c.Password, c.Host)); err != nil {
			return err
		}
	}
	fromAddr := c.From
	if i := strings.LastIndex(fromAddr, "<"); i >= 0 {
		fromAddr = strings.Trim(fromAddr[i:], "<> ")
	}
	if err := cl.Mail(fromAddr); err != nil {
		return err
	}
	if err := cl.Rcpt(to); err != nil {
		return err
	}
	w, err := cl.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(buildMessage(c.From, to, subject, htmlBody)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return cl.Quit()
}

func buildMessage(from, to, subject, body string) []byte {
	var sb strings.Builder
	sb.WriteString("From: " + from + "\r\n")
	sb.WriteString("To: " + to + "\r\n")
	sb.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n")
	sb.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	sb.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	enc := base64.StdEncoding.EncodeToString([]byte(body))
	for len(enc) > 76 {
		sb.WriteString(enc[:76] + "\r\n")
		enc = enc[76:]
	}
	sb.WriteString(enc + "\r\n")
	return []byte(sb.String())
}

// reminderEmail — тема и HTML письма-напоминания: название, сумма, валюта, срок, ссылка на чек.
func reminderEmail(v models.OccurrenceView, publicURL string, today models.Date) (string, string) {
	subject := fmt.Sprintf("Оплатить «%s» до %s", v.Title, v.DueDate.Format("02.01.2006"))
	if v.Status == models.StatusOverdue {
		subject = fmt.Sprintf("Просрочено: «%s» (срок %s)", v.Title, v.DueDate.Format("02.01.2006"))
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, `<p style="font-size:16px;margin:0 0 16px">🔔 <b>Напоминание об оплате</b><br><span style="color:#6b6b6b">%s</span></p>`, html.EscapeString(service.DaysLeftRU(v, today)))
	sb.WriteString(`<table cellpadding="6" style="border-collapse:collapse;font-size:15px">`)
	row := func(k, val string) {
		fmt.Fprintf(&sb, `<tr><td style="color:#6b6b6b">%s</td><td><b>%s</b></td></tr>`, k, val)
	}
	row("Платёж", html.EscapeString(v.Title))
	row("Сумма", v.Amount.StringFixed(2)+" "+v.Currency)
	row("Срок оплаты", v.DueDate.Format("02.01.2006"))
	if v.ReceiptURL != nil {
		row("Чек", fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(*v.ReceiptURL), html.EscapeString(*v.ReceiptURL)))
	}
	sb.WriteString(`</table>`)
	if v.Status == models.StatusOverdue {
		fmt.Fprintf(&sb, `<p style="color:#b42318">⚠️ Просрочено на %d дн.</p>`, v.OverdueDays)
	}
	sb.WriteString(paymentButtonHTML(publicURL, v.ID, "Оплатить и отметить на сайте"))
	return subject, wrapHTML(sb.String(), publicURL)
}

func paymentButtonHTML(publicURL string, id uint, label string) string {
	if publicURL == "" {
		return ""
	}
	return fmt.Sprintf(`<p style="margin-top:20px"><a href="%s" style="background:#f97316;color:#fff;padding:10px 18px;border-radius:999px;text-decoration:none">%s</a></p>`,
		html.EscapeString(strings.TrimRight(publicURL, "/")+PaymentPath(id)), html.EscapeString(label))
}

// paidEmail — письмо «Оплата проведена» со ссылкой на чек.
func paidEmail(v models.OccurrenceView, receipt, publicURL string) string {
	var sb strings.Builder
	sb.WriteString(`<p style="font-size:16px;margin:0 0 16px">✅ <b>Оплата проведена</b></p>`)
	sb.WriteString(`<table cellpadding="6" style="border-collapse:collapse;font-size:15px">`)
	row := func(k, val string) {
		fmt.Fprintf(&sb, `<tr><td style="color:#6b6b6b">%s</td><td><b>%s</b></td></tr>`, k, val)
	}
	row("Платёж", html.EscapeString(v.Title))
	row("Сумма", v.Amount.StringFixed(2)+" "+v.Currency)
	row("Срок оплаты", v.DueDate.Format("02.01.2006"))
	if v.PaidAt != nil {
		row("Оплачено", v.PaidAt.Format("02.01.2006"))
	}
	if receipt != "" {
		row("Чек", fmt.Sprintf(`<a href="%s">открыть чек</a>`, html.EscapeString(receipt)))
	} else {
		row("Чек", "не приложен")
	}
	sb.WriteString(`</table>`)
	sb.WriteString(paymentButtonHTML(publicURL, v.ID, "Открыть платёж"))
	return wrapHTML(sb.String(), publicURL)
}

func wrapHTML(content, publicURL string) string {
	link := ""
	if publicURL != "" {
		link = fmt.Sprintf(`<p style="margin-top:24px"><a href="%s/payments" style="background:#0d0d0d;color:#fff;padding:10px 18px;border-radius:999px;text-decoration:none">Открыть Invoice Keeper</a></p>`,
			html.EscapeString(strings.TrimRight(publicURL, "/")))
	}
	return `<!doctype html><html><body style="font-family:Arial,sans-serif;color:#0d0d0d;background:#f4f4f2;padding:24px">` +
		`<div style="max-width:520px;margin:auto;background:#fff;border-radius:20px;padding:28px">` + content + link +
		`<p style="color:#9a9a9a;font-size:12px;margin-top:24px">Invoice Keeper — напоминания о регулярных платежах. Отключить письма можно в настройках профиля.</p>` +
		`</div></body></html>`
}
