package service

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"invoicekeeper/internal/models"
	"invoicekeeper/internal/receipt"
	"invoicekeeper/internal/store"
)

// Ключи глобальных настроек (таблица app_settings).
const (
	SettingWebhookURL       = "webhook_url"
	SettingTelegramChatID   = "telegram_chat_id"
	SettingTelegramBot      = "telegram_bot_username" // пишет сам бот при старте
	SettingTelegramBotSeen  = "telegram_bot_seen"     // бот на связи с Telegram (RFC3339, обновляется ~раз в 20 с)
	SettingTelegramBotError = "telegram_bot_error"    // последняя ошибка бота; пусто — всё работает
)

// ChannelEnv — что задано в .env (значения из .env имеют приоритет над настройками в панели).
type ChannelEnv struct {
	SMTP           SMTPEnv // почтовый сервер из .env (пусто — настраивается в панели)
	TelegramToken  bool
	TelegramChatID int64
	WebhookURL     string
	ReminderDays   []int          // дни до срока, за которые приходят напоминания (7, 3, 1)
	Loc            *time.Location // единый часовой пояс системы (TZ)
	DemoReceipts   bool           // DEMO_MODE: тестовая версия, на сайте есть образцы чеков
	LinkSecret     string         // ключ подписи ссылок «Открыть бота» (берётся из JWT_SECRET)
}

// SettingsService — личные настройки уведомлений и (для админа) глобальные интеграции.
type SettingsService struct {
	st       *store.Store
	audit    *Audit
	env      ChannelEnv
	receipts *ReceiptService
}

// UseReceipts — режим проверки оплат показывается и меняется в настройках.
func (s *SettingsService) UseReceipts(r *ReceiptService) { s.receipts = r }

// Verification — как подтверждается оплата.
type Verification struct {
	// Policy: off — кнопка «Оплачено» без чека; auto — нужен чек, чистый чек подтверждается сам;
	// manual — нужен чек, решение всегда за администратором.
	Policy       string                `json:"policy"`
	Capabilities *receipt.Capabilities `json:"capabilities,omitempty"` // только администратору
	Pending      *int64                `json:"pending,omitempty"`      // чеков на проверке (администратору)
	Demo         bool                  `json:"demo,omitempty"`         // тестовая версия: показываем образцы чеков
}

func NewSettingsService(st *store.Store, audit *Audit, env ChannelEnv) *SettingsService {
	return &SettingsService{st: st, audit: audit, env: env}
}

type ProfileSettings struct {
	Login              string  `json:"login"`
	Email              *string `json:"email"`
	EmailNotifications bool    `json:"email_notifications"`
	TelegramUsername   *string `json:"telegram_username"`
	// TelegramStatus: none — не привязан, pending — ждём «Это я» в боте, linked — подтверждён.
	TelegramStatus string `json:"telegram_status"`
	TelegramBot    string `json:"telegram_bot"` // @имя бота без «@», чтобы показать ссылку
	// TelegramStartLink — ссылка t.me/<бот>?start=<подписанный код>: нажатие Start в боте сразу подключает Telegram
	TelegramStartLink string `json:"telegram_start_link,omitempty"`
	// TelegramContact — бот уже видел этот @username (человек писал ему): запрос дойдёт
	TelegramContact bool `json:"telegram_contact"`
}

// BotStatus — работает ли бот прямо сейчас: online — был на связи с Telegram за последние 90 секунд.
type BotStatus struct {
	Online   bool   `json:"online"`
	Error    string `json:"error,omitempty"` // conflict / unauthorized / network: … — причина, почему бот молчит
	LastSeen string `json:"last_seen,omitempty"`
}

func telegramStatus(u *models.User) string {
	switch {
	case u.TelegramVerifiedAt != nil && u.TelegramID != nil:
		return "linked" // в т.ч. подключённые по ссылке из сайта без @имени
	case u.TelegramUsername == nil:
		return "none"
	}
	return "pending"
}

// Integrations — глобальные каналы. Webhook один на всю систему:
// одна точка интеграции на всю систему.
type Integrations struct {
	WebhookURL          string        `json:"webhook_url"`
	WebhookFromEnv      bool          `json:"webhook_from_env"`
	Mail                *MailSettings `json:"mail,omitempty"`
	TelegramChatID      string        `json:"telegram_chat_id"`
	TelegramChatFromEnv bool          `json:"telegram_chat_from_env"`
}

type ChannelsStatus struct {
	Email        bool      `json:"email"`             // SMTP настроен
	Telegram     bool      `json:"telegram"`          // бот настроен (есть токен): личные напоминания работают
	TelegramChat bool      `json:"telegram_chat"`     // задан общий чат (напоминания об общих платежах)
	Webhook      *bool     `json:"webhook,omitempty"` // только для администратора
	ReminderDays []int     `json:"reminder_days"`
	Bot          BotStatus `json:"bot"`
}

type Settings struct {
	Profile      ProfileSettings `json:"profile"`
	Verification *Verification   `json:"verification,omitempty"`
	Integrations *Integrations   `json:"integrations,omitempty"` // только для администратора
	Channels     ChannelsStatus  `json:"channels"`
	// Clock — единое время системы: часовой пояс и «сегодня» (сайт сравнивает с календарём устройства)
	Clock Clock `json:"clock"`
}

type Clock struct {
	Timezone string `json:"timezone"`
	Today    string `json:"today"`
	Now      string `json:"now"` // RFC3339 с поправкой пояса
}

// Effective — итоговые значения глобальных каналов (.env важнее панели).
type Effective struct {
	WebhookURL     string
	TelegramChatID int64
}

// EffectiveChannels читает настройки из БД и накладывает значения из .env.
func EffectiveChannels(ctx context.Context, st *store.Store, env ChannelEnv) (Effective, map[string]string, error) {
	m, err := st.Settings(ctx)
	if err != nil {
		return Effective{}, nil, err
	}
	e := Effective{WebhookURL: m[SettingWebhookURL]}
	if env.WebhookURL != "" {
		e.WebhookURL = env.WebhookURL
	}
	e.TelegramChatID, _ = strconv.ParseInt(m[SettingTelegramChatID], 10, 64)
	if env.TelegramChatID != 0 {
		e.TelegramChatID = env.TelegramChatID
	}
	return e, m, nil
}

func (s *SettingsService) Get(ctx context.Context, u *models.User) (*Settings, error) {
	eff, m, err := EffectiveChannels(ctx, s.st, s.env)
	if err != nil {
		return nil, err
	}
	out := &Settings{
		Profile: ProfileSettings{Login: u.Login, Email: u.Email, EmailNotifications: u.EmailNotifications, TelegramUsername: u.TelegramUsername, TelegramStatus: telegramStatus(u), TelegramBot: m[SettingTelegramBot]},
		Channels: ChannelsStatus{
			Telegram: s.env.TelegramToken, TelegramChat: s.env.TelegramToken && eff.TelegramChatID != 0,
			ReminderDays: s.env.ReminderDays,
		},
	}
	ms, err := s.mailSettings(ctx)
	if err != nil {
		return nil, err
	}
	out.Channels.Email = ms.Configured
	if seen, err := time.Parse(time.RFC3339, m[SettingTelegramBotSeen]); err == nil {
		out.Channels.Bot = BotStatus{Online: time.Since(seen) < 90*time.Second, Error: m[SettingTelegramBotError], LastSeen: seen.Format(time.RFC3339)}
	}
	if u.TelegramUsername != nil && out.Profile.TelegramStatus == "pending" {
		_, err := s.st.TelegramContact(ctx, *u.TelegramUsername)
		out.Profile.TelegramContact = err == nil
	}
	if bot := m[SettingTelegramBot]; bot != "" && s.env.TelegramToken && out.Profile.TelegramStatus != "linked" {
		if tok := TelegramLinkToken(s.env.LinkSecret, u.ID, time.Now()); tok != "" {
			out.Profile.TelegramStartLink = "https://t.me/" + bot + "?start=" + tok
		}
	}
	if loc := s.env.Loc; loc != nil {
		now := time.Now().In(loc)
		out.Clock = Clock{Timezone: loc.String(), Today: now.Format(models.DateLayout), Now: now.Format(time.RFC3339)}
	}
	if s.receipts != nil {
		out.Verification = &Verification{Policy: s.receipts.Policy(ctx), Demo: s.env.DemoReceipts}
		if u.Role == models.RoleAdmin {
			c := s.receipts.Capabilities()
			out.Verification.Capabilities = &c
			if n, err := s.receipts.PendingCount(ctx); err == nil {
				out.Verification.Pending = &n
			}
		}
	}
	if u.Role == models.RoleAdmin {
		hook := eff.WebhookURL != ""
		out.Channels.Webhook = &hook
		in := &Integrations{
			WebhookURL: m[SettingWebhookURL], TelegramChatID: m[SettingTelegramChatID],
			WebhookFromEnv: s.env.WebhookURL != "", TelegramChatFromEnv: s.env.TelegramChatID != 0,
		}
		if in.WebhookFromEnv {
			in.WebhookURL = s.env.WebhookURL
		}
		if in.TelegramChatFromEnv {
			in.TelegramChatID = strconv.FormatInt(s.env.TelegramChatID, 10)
		}
		in.Mail = ms
		out.Integrations = in
	}
	return out, nil
}

// UpdateSettingsInput — PATCH /settings. Пустая строка очищает значение.
type UpdateSettingsInput struct {
	Email              *string `json:"email"`
	EmailNotifications *bool   `json:"email_notifications"`
	TelegramUsername   *string `json:"telegram_username"`
	// только администратор:
	WebhookURL     *string `json:"webhook_url"`
	TelegramChatID *string `json:"telegram_chat_id"`
	// PaymentVerification — режим проверки оплат: off | auto | manual
	PaymentVerification *string `json:"payment_verification"`
	// SMTP — почтовый сервер для писем (только администратор)
	SMTP *SMTPInput `json:"smtp"`
}

func (s *SettingsService) Update(ctx context.Context, a Actor, u *models.User, in UpdateSettingsInput) (*Settings, error) {
	changed := D{}

	// --- личные ---
	profileChanged := false
	if in.Email != nil {
		em, err := normEmail(*in.Email, false)
		if err != nil {
			return nil, err
		}
		if err := checkEmailFree(ctx, s.st, em, u.ID); err != nil {
			return nil, err
		}
		u.Email = em
		if em == nil {
			u.EmailNotifications = false
		}
		changed["email"], profileChanged = em, true
	}
	if in.EmailNotifications != nil {
		if *in.EmailNotifications && u.Email == nil {
			return nil, Invalid("set email to receive email reminders")
		}
		u.EmailNotifications = *in.EmailNotifications
		changed["email_notifications"], profileChanged = *in.EmailNotifications, true
	}
	if in.TelegramUsername != nil {
		name, err := NormTelegramUsername(*in.TelegramUsername)
		if err != nil {
			return nil, err
		}
		if name == "" {
			u.TelegramUsername = nil
		} else {
			if other, err := s.st.UserByTelegramUsername(ctx, name); err == nil && other.ID != u.ID {
				return nil, Conflict("telegram_username is linked to another user")
			} else if err != nil && !errors.Is(err, store.ErrNotFound) {
				return nil, err
			}
			if u.TelegramUsername == nil || !strings.EqualFold(*u.TelegramUsername, name) {
				// новый аккаунт: старое подтверждение недействительно, бот попросит подтвердить заново
				u.TelegramID, u.TelegramVerifiedAt, u.TelegramPromptAt = nil, nil, nil
			}
			u.TelegramUsername = &name
		}
		if name == "" {
			u.TelegramID, u.TelegramVerifiedAt, u.TelegramPromptAt = nil, nil, nil
		}
		changed["telegram_username"], profileChanged = u.TelegramUsername, true
	}
	if profileChanged {
		if err := s.st.SaveUser(ctx, u); err != nil {
			if errors.Is(err, store.ErrDuplicate) {
				return nil, Conflict("email already taken")
			}
			return nil, err
		}
	}

	// --- глобальные интеграции ---
	if in.WebhookURL != nil || in.TelegramChatID != nil {
		if err := requireAdmin(a); err != nil {
			return nil, err
		}
		if in.WebhookURL != nil {
			v := strings.TrimSpace(*in.WebhookURL)
			if v != "" && validHTTPURL(v, 1000) != nil {
				return nil, Invalid("webhook_url must be a valid http(s) link")
			}
			if err := s.st.SetSetting(ctx, SettingWebhookURL, v); err != nil {
				return nil, err
			}
			changed["webhook_url"] = v
		}
		if in.TelegramChatID != nil {
			v := strings.TrimSpace(*in.TelegramChatID)
			if v != "" {
				if _, err := strconv.ParseInt(v, 10, 64); err != nil {
					return nil, Invalid("telegram_chat_id must be a number")
				}
			}
			if err := s.st.SetSetting(ctx, SettingTelegramChatID, v); err != nil {
				return nil, err
			}
			changed["telegram_chat_id"] = v
		}
	}
	if in.SMTP != nil {
		if err := requireAdmin(a); err != nil {
			return nil, err
		}
		d, err := s.saveSMTP(ctx, in.SMTP)
		if err != nil {
			return nil, err
		}
		for k, v := range d {
			changed[k] = v
		}
	}
	if in.PaymentVerification != nil {
		if s.receipts == nil {
			return nil, Invalid("payment verification is not available")
		}
		if err := s.receipts.SetPolicy(ctx, a, strings.TrimSpace(*in.PaymentVerification)); err != nil {
			return nil, err
		}
		changed["payment_verification"] = strings.TrimSpace(*in.PaymentVerification)
	}
	if len(changed) > 0 {
		s.audit.Log(ctx, a, ActSettings, "settings", u.ID, changed)
	}
	return s.Get(ctx, u)
}

// LinkTelegramChat сохраняет чат для напоминаний, если он ещё не задан (команда /start в боте).
// Возвращает true, если чат привязан этим вызовом.
func LinkTelegramChat(ctx context.Context, st *store.Store, env ChannelEnv, chatID int64) (bool, error) {
	eff, _, err := EffectiveChannels(ctx, st, env)
	if err != nil {
		return false, err
	}
	if eff.TelegramChatID != 0 {
		return false, nil
	}
	return true, st.SetSetting(ctx, SettingTelegramChatID, strconv.FormatInt(chatID, 10))
}

var tgUsernameRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{4,31}$`)

// NormTelegramUsername приводит «@zeronid», «t.me/zeronid» или «zeronid» к «zeronid».
// Пустая строка — отвязать.
func NormTelegramUsername(v string) (string, error) {
	v = strings.TrimSpace(v)
	for _, p := range []string{"https://", "http://", "t.me/", "telegram.me/", "@"} {
		v = strings.TrimPrefix(v, p)
	}
	if v == "" {
		return "", nil
	}
	if !tgUsernameRe.MatchString(v) {
		return "", Invalid("telegram_username is invalid")
	}
	return v, nil
}

// ResendTelegramRequest — «Отправить запрос ещё раз»: бот повторит «Это вы?» в течение нескольких секунд.
// Если бот ещё не видел этот @username, запрос уйдёт сразу после того, как человек нажмёт Start.
func (s *SettingsService) ResendTelegramRequest(ctx context.Context, u *models.User) (*Settings, error) {
	if u.TelegramUsername == nil {
		return nil, Invalid("set telegram_username first")
	}
	if u.TelegramVerifiedAt != nil && u.TelegramID != nil {
		return nil, Conflict("telegram is already linked")
	}
	if err := s.st.ResetTelegramPrompt(ctx, *u.TelegramUsername); err != nil {
		return nil, err
	}
	u.TelegramPromptAt = nil
	return s.Get(ctx, u)
}
