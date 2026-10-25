// Package models содержит доменные сущности и DTO.
package models

import (
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	RoleAdmin = "admin"
	RoleUser  = "user"

	StatusPaid    = "paid"    // Оплачено
	StatusUnpaid  = "unpaid"  // Не оплачено (срок ещё не прошёл)
	StatusOverdue = "overdue" // Просрочено (со следующего дня после срока)

	PeriodMonthly   = "monthly"
	PeriodQuarterly = "quarterly"
	PeriodYearly    = "yearly"
	PeriodOnce      = "once" // разовый платёж: один период, продления нет
)

// PeriodMonths — сколько месяцев между оплатами (0 — неизвестная периодичность).
func PeriodMonths(p string) int {
	switch p {
	case PeriodMonthly:
		return 1
	case PeriodQuarterly:
		return 3
	case PeriodYearly:
		return 12
	}
	return 0
}

var Currencies = []string{"RUB", "USD", "EUR"}

func ValidCurrency(c string) bool {
	for _, v := range Currencies {
		if v == c {
			return true
		}
	}
	return false
}

type User struct {
	ID           uint    `gorm:"primaryKey" json:"id"`
	Login        string  `gorm:"size:64;uniqueIndex" json:"login"`
	PasswordHash string  `json:"-"`
	Role         string  `json:"role"`
	IsSystem     bool    `json:"is_system"` // администратор из .env — через панель не меняется
	Email        *string `json:"email"`
	// EmailNotifications — получать напоминания на почту (галочка в настройках).
	EmailNotifications bool `json:"email_notifications"`
	// EmailVerifiedAt — почта подтверждена кодом из письма при регистрации.
	EmailVerifiedAt *time.Time `json:"email_verified_at"`
	// TelegramID — числовой ID в Telegram: кто нажал «Оплачено» в боте.
	TelegramID *int64 `json:"telegram_id"`
	// TelegramUsername — @имя в Telegram без «@»; по нему бот опознаёт, кто нажал «Оплачено».
	TelegramUsername *string `json:"telegram_username"`
	// TelegramVerifiedAt — когда человек подтвердил привязку в самом боте; до этого «Оплачено» ему недоступно.
	TelegramVerifiedAt *time.Time `json:"telegram_verified_at"`
	// TelegramPromptAt — когда боту уже отправлен запрос на подтверждение (чтобы не слать повторно).
	TelegramPromptAt *time.Time `json:"-"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type Session struct {
	ID        string `gorm:"primaryKey"`
	UserID    uint
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

// RecurringPayment — шаблон регулярного платежа.
// UserID == nil — общий платёж (видит только администратор); иначе — личный платёж или подписка пользователя.
type RecurringPayment struct {
	ID           uint            `gorm:"primaryKey" json:"id"`
	Title        string          `json:"title"`
	Amount       decimal.Decimal `gorm:"type:numeric(14,2)" json:"amount"`
	Currency     string          `json:"currency"`
	Period       string          `json:"period"`
	DayOfMonth   int             `json:"day_of_month"`
	FirstDueDate Date            `json:"first_due_date"`
	UserID       *uint           `json:"user_id"`
	ServiceID    *uint           `json:"service_id"`
	TrialUntil   *Date           `json:"trial_until"` // первый платёж после пробного периода
	// AutoRenew — продлевать ли платёж: false — новые периоды не создаются (разовые платежи — всегда false по смыслу)
	AutoRenew bool           `gorm:"default:true" json:"auto_renew"`
	CreatedBy *uint          `json:"created_by,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// RecurringView — регулярный платёж + логин владельца (для админа).
type RecurringView struct {
	RecurringPayment `gorm:"embedded"`
	OwnerLogin       *string `gorm:"column:owner_login" json:"owner_login"`
}

// Service — услуга в каталоге администратора. Пользователь подключает её сам.
type Service struct {
	ID          uint            `gorm:"primaryKey" json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Amount      decimal.Decimal `gorm:"type:numeric(14,2)" json:"amount"`
	Currency    string          `json:"currency"`
	Period      string          `json:"period"`
	TrialDays   int             `json:"trial_days"` // 0 — первый платёж сразу при подключении
	Active      bool            `json:"active"`
	CreatedBy   *uint           `json:"-"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	DeletedAt   gorm.DeletedAt  `gorm:"index" json:"-"`
}

// ServiceView — услуга + подписка текущего пользователя и число подписчиков.
type ServiceView struct {
	Service        `gorm:"embedded"`
	SubscriptionID *uint `gorm:"column:subscription_id" json:"subscription_id"` // активная подписка текущего пользователя
	Subscribers    int   `gorm:"column:subscribers" json:"subscribers"`
}

// PaymentOccurrence — конкретный период платежа со своей датой и отметкой.
// Название/сумма/валюта копируются из шаблона — история не меняется задним числом.
type PaymentOccurrence struct {
	ID                 uint            `gorm:"primaryKey" json:"id"`
	RecurringPaymentID uint            `json:"recurring_payment_id"`
	Title              string          `json:"title"`
	Amount             decimal.Decimal `gorm:"type:numeric(14,2)" json:"amount"`
	Currency           string          `json:"currency"`
	DueDate            Date            `json:"due_date"`
	PaidAt             *Date           `json:"paid_at"`
	PaidBy             *string         `json:"paid_by"`
	ReminderSentAt     *time.Time      `json:"reminder_sent_at"`
	ReceiptURL         *string         `json:"receipt_url"` // ссылка на чек / квитанцию
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// OccurrenceView — период платежа + вычисляемые поля (статус и просрочка).
type OccurrenceView struct {
	PaymentOccurrence `gorm:"embedded"`
	RecurringDeleted  bool    `gorm:"column:recurring_deleted" json:"recurring_deleted"`
	UserID            *uint   `gorm:"column:user_id" json:"user_id"`         // владелец (nil — общий платёж)
	OwnerLogin        *string `gorm:"column:owner_login" json:"owner_login"` // логин владельца
	ServiceID         *uint   `gorm:"column:service_id" json:"service_id"`
	TrialUntil        *Date   `gorm:"column:trial_until" json:"trial_until"`
	// ReceiptStatus — статус последнего чека: pending (на проверке), approved, rejected; nil — чеков нет.
	ReceiptStatus *string `gorm:"column:receipt_status" json:"receipt_status"`
	Status        string  `gorm:"-" json:"status"`
	OverdueDays   int     `gorm:"-" json:"overdue_days"`
	// Future — будущий период: срок ещё не наступил (завтра и позже), оплата не требуется прямо сейчас
	Future bool `gorm:"-" json:"future"`
	// Period — периодичность платежа (monthly, quarterly, yearly, once)
	Period    string `gorm:"column:period" json:"period"`
	AutoRenew bool   `gorm:"column:auto_renew" json:"auto_renew"`
}

// AuditLog — запись журнала. Details — произвольный JSON (колонка JSONB).
type AuditLog struct {
	ID         uint            `json:"id"`
	UserID     *uint           `json:"user_id"`
	Actor      string          `json:"actor"`
	Action     string          `json:"action"`
	EntityType string          `json:"entity_type"`
	EntityID   string          `json:"entity_id"`
	Details    json.RawMessage `json:"details"`
	IP         string          `json:"ip"`
	UserAgent  string          `json:"user_agent"`
	CreatedAt  time.Time       `json:"created_at"`
}

const (
	BudgetMonth = "month"
	BudgetYear  = "year"
)

// Budget — плановая сумма расходов на месяц или год в одной валюте.
// Бюджет личный: у каждого пользователя свой план (считается по видимым ему платежам).
type Budget struct {
	ID        uint            `gorm:"primaryKey" json:"id"`
	UserID    *uint           `json:"user_id"`
	Currency  string          `json:"currency"`
	Period    string          `json:"period"`
	Amount    decimal.Decimal `gorm:"type:numeric(14,2)" json:"amount"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// AppSetting — глобальная настройка (key/value): webhook_url, telegram_chat_id, telegram_bot_username.
type AppSetting struct {
	Key       string `gorm:"primaryKey"`
	Value     string
	UpdatedAt time.Time
}

func (AppSetting) TableName() string { return "app_settings" }

// События для webhook.
const (
	EventPaymentCreated  = "payment.created"
	EventSubscribed      = "subscription.created"
	EventUnsubscribed    = "subscription.cancelled"
	EventPaymentReminder = "payment.reminder"
	EventPaymentPaid     = "payment.paid"
	EventPaymentUnpaid   = "payment.unpaid"
	EventPaymentDateFix  = "payment.date_fixed"
	EventReceiptPending  = "receipt.pending"  // чек ждёт решения администратора
	EventReceiptRejected = "receipt.rejected" // чек отклонён
	EventTest            = "test"
)

// PaymentEvent — тело POST-запроса на webhook.
type PaymentEvent struct {
	Event       string    `json:"event"`
	PaymentID   uint      `json:"payment_id"`           // ID периода (для payment.created — ID регулярного платежа)
	RecurringID uint      `json:"recurring_payment_id"` // ID регулярного платежа
	Name        string    `json:"name"`
	Amount      string    `json:"amount"`
	Currency    string    `json:"currency"`
	DueDate     string    `json:"due_date"`
	Status      string    `json:"status"`
	PaidAt      *string   `json:"paid_at,omitempty"`
	ReceiptURL  *string   `json:"receipt_url,omitempty"`
	Actor       string    `json:"actor,omitempty"`
	Owner       *string   `json:"owner,omitempty"` // логин владельца подписки (нет — общий платёж)
	Timestamp   time.Time `json:"timestamp"`
}

// PendingRegistration — регистрация, ожидающая кода из письма.
type PendingRegistration struct {
	Email        string `gorm:"primaryKey"`
	Login        string
	PasswordHash string
	CodeHash     string
	Attempts     int
	Resends      int
	ExpiresAt    time.Time
	SentAt       time.Time
	CreatedAt    time.Time
}

func (PendingRegistration) TableName() string { return "pending_registrations" }

// WebhookDelivery — одна отправка на webhook (журнал в «Настройки → Webhook»).
type WebhookDelivery struct {
	ID         uint64    `gorm:"primaryKey" json:"id"`
	Event      string    `json:"event"`
	PaymentID  *uint     `json:"payment_id"`
	Name       string    `json:"name"`
	URL        string    `json:"url"`
	OK         bool      `json:"ok"`
	StatusCode int       `json:"status_code"` // 0 — ответа не было (таймаут, нет соединения)
	DurationMs int       `json:"duration_ms"`
	Error      string    `json:"error"`    // понятное описание ошибки на русском
	Payload    string    `json:"payload"`  // отправленный JSON
	Response   string    `json:"response"` // начало ответа сервера
	CreatedAt  time.Time `json:"created_at"`
}

func (WebhookDelivery) TableName() string { return "webhook_deliveries" }

// CurrencySummary — итоги по одной валюте (валюты НЕ конвертируются).
type CurrencySummary struct {
	Currency     string          `json:"currency"`
	Total        decimal.Decimal `json:"total"`
	Paid         decimal.Decimal `json:"paid"`
	Unpaid       decimal.Decimal `json:"unpaid"`
	Overdue      decimal.Decimal `json:"overdue"`
	CountTotal   int             `json:"count_total"`
	CountPaid    int             `json:"count_paid"`
	CountUnpaid  int             `json:"count_unpaid"`
	CountOverdue int             `json:"count_overdue"`
}

// TelegramContact — кто писал боту: @имя (в нижнем регистре) → числовой ID и личный чат.
type TelegramContact struct {
	Username  string `gorm:"primaryKey;size:64"`
	TgID      int64
	ChatID    int64
	UpdatedAt time.Time
}

// ---------- проверка оплат по чекам ----------

const (
	ReceiptPending  = "pending"  // на проверке у администратора
	ReceiptApproved = "approved" // оплата подтверждена (автоматически или администратором)
	ReceiptRejected = "rejected" // отклонён

	CheckOK   = "ok"
	CheckWarn = "warn" // не удалось проверить — решает администратор
	CheckFail = "fail" // не совпадает — решает администратор

	VerifyOff    = "off"    // чек не нужен: «Оплачено» сразу отмечает оплату
	VerifyAuto   = "auto"   // нужен чек; если все проверки пройдены — подтверждается автоматически
	VerifyManual = "manual" // нужен чек; решение всегда принимает администратор
)

// ReceiptCheck — результат одной проверки чека.
type ReceiptCheck struct {
	Code   string `json:"code"`
	Status string `json:"status"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

// Receipt — чек (подтверждение оплаты). Хранится всегда, вместе со снимком данных платежа.
type Receipt struct {
	ID              uint             `gorm:"primaryKey" json:"id"`
	OccurrenceID    *uint            `json:"occurrence_id"`
	UserID          *uint            `json:"user_id"`
	SubmittedBy     string           `json:"submitted_by"`
	Source          string           `json:"source"` // web | telegram
	PaymentTitle    string           `json:"payment_title"`
	PaymentAmount   decimal.Decimal  `gorm:"type:numeric(14,2)" json:"payment_amount"`
	PaymentCurrency string           `json:"payment_currency"`
	PaymentDueDate  *Date            `json:"payment_due_date"`
	OwnerLogin      *string          `json:"owner_login"`
	FileName        string           `json:"file_name"`
	Mime            string           `json:"mime"`
	Size            int              `json:"size"`
	SHA256          string           `gorm:"column:sha256" json:"sha256"`
	Content         []byte           `gorm:"->:false;<-:create" json:"-"` // файл отдаётся отдельным запросом
	Note            string           `json:"note"`
	Methods         string           `json:"methods"`
	ExtAmount       *decimal.Decimal `gorm:"type:numeric(14,2)" json:"ext_amount"`
	ExtCurrency     *string          `json:"ext_currency"`
	ExtDate         *Date            `json:"ext_date"`
	ExtOperationID  *string          `json:"ext_operation_id"`
	ExtPayee        *string          `json:"ext_payee"`
	ExtText         string           `json:"ext_text"`
	AIComment       string           `gorm:"column:ai_comment" json:"ai_comment"`
	Checks          json.RawMessage  `gorm:"type:jsonb" json:"checks"`
	Score           int              `json:"score"`
	Status          string           `json:"status"`
	DecidedBy       *string          `json:"decided_by"`
	DecidedAuto     bool             `json:"decided_auto"`
	DecisionComment string           `json:"decision_comment"`
	DecidedAt       *time.Time       `json:"decided_at"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

func (Receipt) TableName() string { return "payment_receipts" }

// ReminderStage — ступень напоминания: d7 / d3 / d1 (осталось столько дней или меньше) или od (срок прошёл).
// stages — дни по убыванию (7, 3, 1). Пустая строка — напоминать ещё рано.
func ReminderStage(daysLeft int, stages []int) string {
	if daysLeft < 0 {
		return "od"
	}
	for i := len(stages) - 1; i >= 0; i-- {
		if daysLeft <= stages[i] {
			return "d" + itoa(stages[i])
		}
	}
	return ""
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// Notification — уведомление в колокольчике веб-панели.
type Notification struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	UserID       uint       `json:"-"`
	Kind         string     `json:"kind"`  // reminder, paid, created, receipt, system
	Level        string     `json:"level"` // info, success, warn, danger
	Title        string     `json:"title"`
	Body         string     `json:"body"`
	OccurrenceID *uint      `json:"occurrence_id"`
	ReceiptID    *uint      `json:"receipt_id"`
	ReadAt       *time.Time `json:"read_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

// MonthSummary — итоги за месяц по одной валюте: что оплачено, что просрочено, что ещё предстоит.
type MonthSummary struct {
	Month        string          `json:"month"` // YYYY-MM
	Currency     string          `json:"currency"`
	Total        decimal.Decimal `json:"total"`
	Paid         decimal.Decimal `json:"paid"`
	Overdue      decimal.Decimal `json:"overdue"`
	Upcoming     decimal.Decimal `json:"upcoming"`
	Count        int             `json:"count"`
	CountPaid    int             `json:"count_paid"`
	CountOverdue int             `json:"count_overdue"`
}
