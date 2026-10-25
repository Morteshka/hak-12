package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"invoicekeeper/internal/models"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"
)

// Личные напоминания в Telegram отправляет API-сервер напрямую через Bot API (тот же токен, что у бота).
// Нажатие кнопки «✅ Оплачено» приходит процессу bot — он отмечает оплату.

type tgButton struct {
	Text         string    `json:"text"`
	CallbackData string    `json:"callback_data,omitempty"`
	URL          string    `json:"url,omitempty"`
	WebApp       *tgWebApp `json:"web_app,omitempty"` // открыть страницу как Telegram Mini App (только HTTPS, личный чат)
}

type tgWebApp struct {
	URL string `json:"url"`
}

type tgKeyboard struct {
	InlineKeyboard [][]tgButton `json:"inline_keyboard"`
}

type tgMessage struct {
	ChatID      int64       `json:"chat_id"`
	Text        string      `json:"text"`
	ParseMode   string      `json:"parse_mode"`
	ReplyMarkup *tgKeyboard `json:"reply_markup,omitempty"`
}

// sendTelegram шлёт HTML-сообщение; paymentID > 0 — с кнопкой «Оплачено» для этого периода.
func (h *Hub) sendTelegram(ctx context.Context, chatID int64, text string, paymentID uint) error {
	var kb [][]tgButton
	if paymentID > 0 {
		kb = h.paymentButtons(models.OccurrenceView{PaymentOccurrence: models.PaymentOccurrence{ID: paymentID}}, true)
	}
	return h.sendTelegramKB(ctx, chatID, text, kb)
}

// sendTelegramKB — HTML-сообщение с произвольными кнопками.
func (h *Hub) sendTelegramKB(ctx context.Context, chatID int64, text string, kb [][]tgButton) error {
	if h.cfg.TelegramToken == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN is not set")
	}
	m := tgMessage{ChatID: chatID, Text: text, ParseMode: "HTML"}
	if len(kb) > 0 {
		m.ReplyMarkup = &tgKeyboard{InlineKeyboard: kb}
	}
	body, _ := json.Marshal(m)
	return h.tgCall(ctx, "sendMessage", "application/json", bytes.NewReader(body))
}

// sendTelegramFile — фото или документ с подписью и кнопками (multipart).
func (h *Hub) sendTelegramFile(ctx context.Context, chatID int64, asPhoto bool, name string, data []byte, caption string, kb [][]tgButton) error {
	if h.cfg.TelegramToken == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN is not set")
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("chat_id", strconv.FormatInt(chatID, 10))
	_ = w.WriteField("caption", caption)
	_ = w.WriteField("parse_mode", "HTML")
	if len(kb) > 0 {
		raw, _ := json.Marshal(tgKeyboard{InlineKeyboard: kb})
		_ = w.WriteField("reply_markup", string(raw))
	}
	field, method := "document", "sendDocument"
	if asPhoto {
		field, method = "photo", "sendPhoto"
	}
	fw, err := w.CreateFormFile(field, name)
	if err != nil {
		return err
	}
	_, _ = fw.Write(data)
	_ = w.Close()
	return h.tgCall(ctx, method, w.FormDataContentType(), &buf)
}

func (h *Hub) tgCall(ctx context.Context, method, contentType string, body io.Reader) error {
	url := "https://api.telegram.org/bot" + h.cfg.TelegramToken + "/" + method
	if h.cfg.TelegramAPI != "" {
		url = fmt.Sprintf(h.cfg.TelegramAPI, h.cfg.TelegramToken, method)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	client := h.client
	if method != "sendMessage" {
		client = &http.Client{Timeout: 60 * time.Second} // файл до 10 МБ
	}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: %s", redact(err.Error(), h.cfg.TelegramToken))
	}
	defer res.Body.Close()
	var out struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
	_ = json.Unmarshal(raw, &out)
	if !out.OK {
		if out.Description == "" {
			out.Description = res.Status
		}
		return fmt.Errorf("telegram: %s", out.Description)
	}
	return nil
}

// redact убирает токен бота из текста ошибки (в URL запроса).
func redact(s, token string) string {
	if token == "" {
		return s
	}
	return string(bytes.ReplaceAll([]byte(s), []byte(token), []byte("***")))
}
