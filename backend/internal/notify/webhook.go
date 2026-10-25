package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"invoicekeeper/internal/models"
)

// EchoPath — встроенный тестовый приёмник: принимает события и отвечает 200.
// Нужен, чтобы попробовать webhook без внешних сервисов.
const EchoPath = "/api/v1/webhook/echo"

// postWebhook отправляет событие JSON-ом и пишет результат в журнал отправок.
func (h *Hub) postWebhook(ctx context.Context, target string, ev models.PaymentEvent) error {
	_, err := h.deliverWebhook(ctx, target, ev)
	return err
}

// deliverWebhook — одна отправка: POST с JSON, подпись HMAC (если задан WEBHOOK_SECRET), запись в журнал.
func (h *Hub) deliverWebhook(ctx context.Context, target string, ev models.PaymentEvent) (*models.WebhookDelivery, error) {
	body, err := json.Marshal(ev)
	if err != nil {
		return nil, err
	}
	headers := map[string]string{"X-InvoiceKeeper-Event": ev.Event}
	if h.cfg.WebhookSecret != "" {
		headers["X-InvoiceKeeper-Signature"] = Sign(h.cfg.WebhookSecret, body)
	}
	start := time.Now()
	code, resp, err := h.post(ctx, h.route(target), body, headers)
	d := &models.WebhookDelivery{
		Event: ev.Event, Name: clip(ev.Name, 200), URL: clip(target, 500), OK: err == nil,
		StatusCode: code, DurationMs: int(time.Since(start).Milliseconds()),
		Payload: string(body), Response: clip(resp, 500),
	}
	if ev.PaymentID != 0 {
		id := ev.PaymentID
		d.PaymentID = &id
	}
	if err != nil {
		d.Error = clip(humanError(err, code), 500)
	}
	if h.st != nil {
		if lerr := h.st.AddWebhookDelivery(context.WithoutCancel(ctx), d); lerr != nil {
			log.Printf("notify: webhook log: %v", lerr)
		}
	}
	return d, err
}

// Sign — подпись тела: "sha256=" + HMAC-SHA256(secret, body) в hex.
func Sign(secret string, body []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(body)
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

// route — встроенный тестовый приёмник вызывается напрямую через внутреннюю сеть (INTERNAL_API_URL),
// а не через внешний адрес сайта: из контейнера внешний адрес своего же сервера доступен не всегда.
func (h *Hub) route(target string) string {
	if h.cfg.InternalURL == "" {
		return target
	}
	u, err := url.Parse(target)
	if err != nil || u.Path != EchoPath {
		return target
	}
	return strings.TrimRight(h.cfg.InternalURL, "/") + EchoPath
}

var errHTTPStatus = errors.New("bad status")

func (h *Hub) post(ctx context.Context, target string, body []byte, headers map[string]string) (int, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return 0, "", fmt.Errorf("bad url: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "InvoiceKeeper-Webhook/1.0")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := h.client.Do(req) // таймаут 5 секунд
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
	text := strings.TrimSpace(string(b))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return resp.StatusCode, text, fmt.Errorf("%w: HTTP %d: %s", errHTTPStatus, resp.StatusCode, clip(text, 200))
	}
	return resp.StatusCode, text, nil
}

// humanError — причина неудачи простыми словами (показывается в журнале отправок).
func humanError(err error, code int) string {
	var ne net.Error
	var dnsErr *net.DNSError
	msg := err.Error()
	switch {
	case errors.Is(err, errHTTPStatus):
		switch {
		case code == 404:
			return "Сервер ответил 404: по этому адресу нет приёмника — проверьте ссылку"
		case code == 401 || code == 403:
			return fmt.Sprintf("Сервер ответил %d: приёмник отклонил запрос (нет доступа или неверная подпись)", code)
		case code >= 500:
			return fmt.Sprintf("Сервер ответил %d: ошибка на стороне приёмника", code)
		default:
			return fmt.Sprintf("Сервер ответил %d вместо 2xx", code)
		}
	case strings.HasPrefix(msg, "bad url"):
		return "Неверный адрес"
	case errors.As(err, &dnsErr):
		return "Адрес не найден: проверьте домен в ссылке"
	case errors.As(err, &ne) && ne.Timeout():
		return "Сервер не ответил за 5 секунд"
	case strings.Contains(msg, "connection refused"):
		return "Сервер отклонил подключение: приёмник не запущен или закрыт порт"
	case strings.Contains(msg, "certificate"):
		return "Ошибка HTTPS-сертификата у приёмника"
	}
	return "Не удалось отправить: " + clip(msg, 300)
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for !utf8.ValidString(s) && len(s) > 0 {
		s = s[:len(s)-1]
	}
	return s
}
