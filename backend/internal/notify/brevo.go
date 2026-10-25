package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

// BrevoHost — «сервер» Brevo в настройках почты: письма уходят через HTTPS API, а не SMTP.
const BrevoHost = "api.brevo.com"

// brevoURL — адрес API (подменяется в тестах).
var brevoURL = "https://api.brevo.com/v3/smtp/email"

var brevoClient = &http.Client{Timeout: 15 * time.Second}

// sendBrevo — письмо через Brevo: логин — подтверждённый в Brevo адрес отправителя, пароль — API-ключ.
func (c SMTPConfig) sendBrevo(to, subject, htmlBody string) error {
	type addr struct {
		Name  string `json:"name,omitempty"`
		Email string `json:"email"`
	}
	sender := addr{Email: c.User}
	if a, err := mail.ParseAddress(c.From); err == nil {
		sender = addr{Name: a.Name, Email: a.Address}
	}
	if sender.Name == "" {
		sender.Name = "Invoice Keeper"
	}
	body, _ := json.Marshal(map[string]any{
		"sender": sender, "to": []addr{{Email: to}}, "subject": subject, "htmlContent": htmlBody,
	})
	req, err := http.NewRequest(http.MethodPost, brevoURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("api-key", c.Password)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := brevoClient.Do(req)
	if err != nil {
		return fmt.Errorf("brevo: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
	var e struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(b, &e)
	return fmt.Errorf("brevo HTTP %d %s: %s", resp.StatusCode, e.Code, e.Message)
}

// humanBrevoError — ответы Brevo простыми словами.
func humanBrevoError(msg string) string {
	low := strings.ToLower(msg)
	switch {
	case strings.Contains(msg, "HTTP 401") || strings.Contains(low, "key not found") || strings.Contains(low, "unauthorized"):
		return "Brevo не принял API-ключ — скопируйте ключ заново (раздел SMTP & API → API Keys)"
	case strings.Contains(low, "sender") || strings.Contains(low, "not valid") && strings.Contains(low, "from"):
		return "Brevo не знает такого отправителя — добавьте и подтвердите эту почту в Brevo (Senders), логин должен совпадать с ней"
	case strings.Contains(low, "permission") || strings.Contains(low, "not activated") || strings.Contains(low, "account"):
		return "аккаунт Brevo ещё не активирован для отправки — проверьте почту от Brevo и подтвердите аккаунт (" + msg + ")"
	case strings.Contains(low, "timeout") || strings.Contains(low, "no such host") || strings.Contains(low, "dial"):
		return "нет связи с api.brevo.com: " + msg
	}
	return "ошибка Brevo: " + msg
}
