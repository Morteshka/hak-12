// Package config загружает настройки приложения из переменных окружения (.env).
package config

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"invoicekeeper/internal/receipt"
	"invoicekeeper/internal/service"
)

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string

	AdminLogin    string
	AdminPassword string

	// Единый часовой пояс системы (IANA), например Europe/Moscow.
	Timezone string
	Loc      *time.Location

	AccessTTL  time.Duration
	RefreshTTL time.Duration

	// Telegram
	TelegramToken  string
	TelegramAPI    string // TELEGRAM_API_ENDPOINT: свой адрес Bot API (локальный сервер или тестовая заглушка)
	TelegramChatID int64
	ReminderDays   []int         // за сколько дней напоминать: 7, 3 и 1 (REMINDER_STAGES)
	CheckInterval  time.Duration // как часто бот проверяет сроки

	// Дополнительные каналы
	AdminEmail          string
	SMTPHost            string
	SMTPPort            int
	SMTPUser            string
	SMTPPassword        string
	SMTPFrom            string
	WebhookURL          string // можно задать и в панели (Настройки → Интеграции); .env важнее
	WebhookSecret       string
	PublicURL           string
	InternalURL         string // адрес API внутри Docker (http://backend:8080) — для тестового приёмника webhook
	TelegramRequireLink bool   // true — «Оплачено» в Telegram могут нажимать только пользователи с привязанным Telegram ID

	// Проверка чеков ИИ-моделью (необязательно): anthropic или openai-совместимый API
	AIProvider string
	AIAPIKey   string
	AIModel    string
	AIBaseURL  string
	OCRLangs   string

	AllowedOrigins []string
	SeedDemo       bool
	// TimeTravel — машина времени для демо: администратор сдвигает системную дату (DEMO_TIME_TRAVEL, по умолчанию вкл.)
	TimeTravel        bool
	DemoReceipts      bool // показывать тестерам образцы чеков
	AllowRegistration bool
	EmailVerify       string // EMAIL_VERIFY: auto (код, если почта подключена) | true | false
}

func Load() (*Config, error) {
	c := &Config{
		Port:              get("PORT", "8080"),
		DatabaseURL:       get("DATABASE_URL", ""),
		JWTSecret:         get("JWT_SECRET", ""),
		AdminLogin:        get("ADMIN_LOGIN", "admin"),
		AdminPassword:     get("ADMIN_PASSWORD", ""),
		Timezone:          get("TZ", "Europe/Moscow"),
		TelegramToken:     get("TELEGRAM_BOT_TOKEN", ""),
		TelegramAPI:       get("TELEGRAM_API_ENDPOINT", ""),
		ReminderDays:      getDays("REMINDER_STAGES", []int{7, 3, 1}),
		SeedDemo:          get("SEED_DEMO", "false") == "true",
		DemoReceipts:      get("DEMO_MODE", "false") == "true",
		AllowRegistration: get("ALLOW_REGISTRATION", "true") == "true",
		TimeTravel:        get("DEMO_TIME_TRAVEL", "true") == "true",

		AdminEmail:          get("ADMIN_EMAIL", ""),
		SMTPHost:            get("SMTP_HOST", ""),
		SMTPPort:            getInt("SMTP_PORT", 587),
		SMTPUser:            get("SMTP_USER", ""),
		SMTPPassword:        get("SMTP_PASSWORD", ""),
		SMTPFrom:            get("SMTP_FROM", ""),
		WebhookURL:          get("WEBHOOK_URL", ""),
		WebhookSecret:       get("WEBHOOK_SECRET", ""),
		PublicURL:           get("PUBLIC_URL", "http://localhost:3000"),
		InternalURL:         get("INTERNAL_API_URL", ""),
		EmailVerify:         get("EMAIL_VERIFY", "auto"),
		TelegramRequireLink: get("TELEGRAM_REQUIRE_LINK", "true") != "false",

		AIProvider: strings.ToLower(get("AI_PROVIDER", "anthropic")),
		AIAPIKey:   get("AI_API_KEY", ""),
		AIModel:    get("AI_MODEL", ""),
		AIBaseURL:  get("AI_BASE_URL", ""),
		OCRLangs:   get("OCR_LANGS", "rus+eng"),
	}
	if c.SMTPFrom == "" {
		c.SMTPFrom = c.SMTPUser
	}
	c.AccessTTL = getDur("ACCESS_TTL", 15*time.Minute)
	c.RefreshTTL = getDur("REFRESH_TTL", 7*24*time.Hour)
	c.CheckInterval = getDur("CHECK_INTERVAL", time.Minute)

	if origins := get("ALLOWED_ORIGINS", ""); origins != "" {
		for _, o := range strings.Split(origins, ",") {
			c.AllowedOrigins = append(c.AllowedOrigins, strings.TrimSpace(o))
		}
	}
	if id := get("TELEGRAM_CHAT_ID", ""); id != "" {
		v, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("TELEGRAM_CHAT_ID: %w", err)
		}
		c.TelegramChatID = v
	}

	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return nil, fmt.Errorf("TZ %q: %w", c.Timezone, err)
	}
	c.Loc = loc

	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	return c, nil
}

// ValidateServer — доп. проверки только для API-сервера.
func (c *Config) ValidateServer() error {
	if len(c.JWTSecret) < 16 {
		return fmt.Errorf("JWT_SECRET must be at least 16 characters")
	}
	if c.AdminLogin == "" || len(c.AdminPassword) < 6 {
		return fmt.Errorf("ADMIN_LOGIN / ADMIN_PASSWORD (min 6 chars) are required")
	}
	return nil
}

// SecurityWarnings — небезопасные значения из примера .env. Сервер запускается, но пишет предупреждение в лог.
func (c *Config) SecurityWarnings() []string {
	var w []string
	if strings.Contains(c.JWTSecret, "change-me") {
		w = append(w, "JWT_SECRET взят из .env.example — замените на случайную строку: openssl rand -hex 32")
	}
	if c.AdminPassword == "change-me-admin" || c.AdminPassword == "admin123" || c.AdminPassword == "admin12345" {
		w = append(w, "ADMIN_PASSWORD простой или из примера — задайте надёжный пароль администратора")
	}
	return w
}

// Analyzer — распознавание чеков с настройками из .env (общий для API и бота).
func (c *Config) Analyzer() *receipt.Analyzer {
	return receipt.New(receipt.Config{
		AI:       receipt.AIConfig{Provider: c.AIProvider, APIKey: c.AIAPIKey, Model: c.AIModel, BaseURL: c.AIBaseURL},
		OCRLangs: c.OCRLangs, Loc: c.Loc,
	})
}

// ChannelEnv — каналы из .env для сервисов.
func (c *Config) ChannelEnv() service.ChannelEnv {
	return service.ChannelEnv{
		SMTP:           service.SMTPEnv{Host: c.SMTPHost, Port: c.SMTPPort, User: c.SMTPUser, Password: c.SMTPPassword, From: c.SMTPFrom},
		TelegramToken:  c.TelegramToken != "",
		TelegramChatID: c.TelegramChatID,
		WebhookURL:     c.WebhookURL,
		ReminderDays:   c.ReminderDays,
		DemoReceipts:   c.DemoReceipts,
		LinkSecret:     c.JWTSecret,
		Loc:            c.Loc,
	}
}

func get(k, def string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return def
}

func getInt(k string, def int) int {
	if v, err := strconv.Atoi(get(k, "")); err == nil {
		return v
	}
	return def
}

// getDays — список дней «7,3,1»: по убыванию, без повторов, только положительные.
func getDays(k string, def []int) []int {
	var out []int
	seen := map[int]bool{}
	for _, f := range strings.FieldsFunc(get(k, ""), func(r rune) bool { return r == ',' || r == ' ' || r == ';' }) {
		if n, err := strconv.Atoi(f); err == nil && n > 0 && n <= 60 && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		return def
	}
	sort.Sort(sort.Reverse(sort.IntSlice(out)))
	return out
}

func getDur(k string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(get(k, "")); err == nil {
		return v
	}
	return def
}
