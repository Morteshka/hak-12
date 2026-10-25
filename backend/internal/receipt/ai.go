package receipt

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AIConfig — необязательная проверка чека ИИ-моделью с распознаванием изображений.
// provider: anthropic (Claude) или openai — любой OpenAI-совместимый API (OpenAI, OpenRouter, локальная Ollama и т. п.).
type AIConfig struct {
	Provider string
	APIKey   string
	Model    string
	BaseURL  string
}

func (c AIConfig) Enabled() bool {
	return c.APIKey != "" || (c.Provider == "openai" && c.BaseURL != "")
}

// Label — что показать в настройках: «anthropic · claude-haiku-5-5».
func (c AIConfig) Label() string {
	if !c.Enabled() {
		return ""
	}
	return c.Provider + " · " + c.Model
}

// AIResult — ответ модели (строгий JSON по инструкции).
type AIResult struct {
	IsPaymentDocument bool     `json:"is_payment_document"`
	Success           *bool    `json:"success"`
	Amount            *string  `json:"amount"`
	Currency          *string  `json:"currency"`
	Date              *string  `json:"date"`
	Payee             *string  `json:"payee"`
	OperationID       *string  `json:"operation_id"`
	Suspicious        []string `json:"suspicious"`
	Comment           string   `json:"comment"`
}

const aiPrompt = `Ты проверяешь документ об оплате для системы учёта регулярных платежей.
Посмотри на изображение (чек, квитанция банка, скриншот перевода) и верни ТОЛЬКО JSON без пояснений:
{
 "is_payment_document": true|false,      // это подтверждение совершённой оплаты/перевода?
 "success": true|false|null,             // операция успешна (не отклонена, не возврат, не «в обработке»)
 "amount": "399.00"|null,                // итоговая списанная/оплаченная сумма, точка как разделитель
 "currency": "RUB"|"USD"|"EUR"|null,
 "date": "YYYY-MM-DD"|null,              // дата операции
 "payee": "строка"|null,                 // получатель / магазин
 "operation_id": "строка"|null,          // номер операции, документа или квитанции
 "suspicious": ["…"],                    // признаки подделки или проблемы: следы редактирования, обрезанный кадр, несовпадающие шрифты, это не чек и т. п. Пусто, если всё в порядке
 "comment": "одно короткое предложение на русском"
}
Не придумывай значения: если поле не видно — null.`

type aiClient struct {
	cfg  AIConfig
	http *http.Client
}

func newAI(cfg AIConfig) *aiClient {
	if !cfg.Enabled() {
		return nil
	}
	if cfg.Provider == "" {
		cfg.Provider = "anthropic"
	}
	if cfg.Model == "" {
		if cfg.Provider == "anthropic" {
			cfg.Model = "claude-haiku-5-5"
		} else {
			cfg.Model = "gpt-4o-mini"
		}
	}
	if cfg.BaseURL == "" {
		if cfg.Provider == "anthropic" {
			cfg.BaseURL = "https://api.anthropic.com"
		} else {
			cfg.BaseURL = "https://api.openai.com/v1"
		}
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &aiClient{cfg: cfg, http: &http.Client{Timeout: 45 * time.Second}}
}

// check отправляет документ модели. Для anthropic PDF уходит как документ, для openai — картинкой первой страницы.
func (c *aiClient) check(ctx context.Context, data []byte, mime string, pagePNG []byte) (*AIResult, error) {
	var (
		req *http.Request
		err error
	)
	if c.cfg.Provider == "anthropic" {
		block := map[string]any{"type": "image", "source": map[string]any{"type": "base64", "media_type": mime, "data": base64.StdEncoding.EncodeToString(data)}}
		if mime == "application/pdf" {
			block["type"] = "document"
		}
		body, _ := json.Marshal(map[string]any{
			"model": c.cfg.Model, "max_tokens": 700, "temperature": 0,
			"messages": []any{map[string]any{"role": "user", "content": []any{block, map[string]any{"type": "text", "text": aiPrompt}}}},
		})
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/v1/messages", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("x-api-key", c.cfg.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		imgMime, img := mime, data
		if mime == "application/pdf" {
			if pagePNG == nil {
				return nil, fmt.Errorf("pdf page is not rendered")
			}
			imgMime, img = "image/png", pagePNG
		}
		body, _ := json.Marshal(map[string]any{
			"model": c.cfg.Model, "temperature": 0,
			"messages": []any{map[string]any{"role": "user", "content": []any{
				map[string]any{"type": "text", "text": aiPrompt},
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:" + imgMime + ";base64," + base64.StdEncoding.EncodeToString(img)}},
			}}},
		})
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		if c.cfg.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
		}
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ai: %s", redactKey(err.Error(), c.cfg.APIKey))
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode/100 != 2 {
		return nil, fmt.Errorf("ai: HTTP %d: %s", res.StatusCode, truncate(string(raw), 300))
	}
	text, err := c.extractText(raw)
	if err != nil {
		return nil, err
	}
	return parseAIJSON(text)
}

func (c *aiClient) extractText(raw []byte) (string, error) {
	if c.cfg.Provider == "anthropic" {
		var out struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			return "", fmt.Errorf("ai: bad response: %w", err)
		}
		var sb strings.Builder
		for _, b := range out.Content {
			if b.Type == "text" {
				sb.WriteString(b.Text)
			}
		}
		return sb.String(), nil
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Choices) == 0 {
		return "", fmt.Errorf("ai: bad response")
	}
	return out.Choices[0].Message.Content, nil
}

// parseAIJSON берёт первый JSON-объект из ответа (модель иногда оборачивает его в ```json).
func parseAIJSON(s string) (*AIResult, error) {
	i, j := strings.Index(s, "{"), strings.LastIndex(s, "}")
	if i < 0 || j <= i {
		return nil, fmt.Errorf("ai: no JSON in answer")
	}
	var r AIResult
	if err := json.Unmarshal([]byte(s[i:j+1]), &r); err != nil {
		return nil, fmt.Errorf("ai: bad JSON: %w", err)
	}
	return &r, nil
}

func redactKey(s, key string) string {
	if key == "" {
		return s
	}
	return strings.ReplaceAll(s, key, "***")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
