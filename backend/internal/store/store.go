// Package store — слой доступа к данным (GORM/PostgreSQL). Без бизнес-логики.
package store

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"invoicekeeper/internal/models"
)

var ErrNotFound = errors.New("not found")
var ErrDuplicate = errors.New("duplicate")

type Store struct{ db *gorm.DB }

func New(db *gorm.DB) *Store { return &Store{db: db} }

func (s *Store) ctx(ctx context.Context) *gorm.DB { return s.db.WithContext(ctx) }

func wrap(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return ErrNotFound
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return ErrDuplicate
	}
	return err
}

// ---------- users ----------

func (s *Store) UserByLogin(ctx context.Context, login string) (*models.User, error) {
	var u models.User
	return &u, wrap(s.ctx(ctx).Where("login = ?", login).First(&u).Error)
}

func (s *Store) UserByID(ctx context.Context, id uint) (*models.User, error) {
	var u models.User
	return &u, wrap(s.ctx(ctx).First(&u, id).Error)
}

func (s *Store) ListUsers(ctx context.Context) ([]models.User, error) {
	var us []models.User
	return us, s.ctx(ctx).Order("id").Find(&us).Error
}

func (s *Store) CreateUser(ctx context.Context, u *models.User) error {
	return wrap(s.ctx(ctx).Create(u).Error)
}

func (s *Store) SaveUser(ctx context.Context, u *models.User) error {
	return wrap(s.ctx(ctx).Save(u).Error)
}

func (s *Store) DeleteUser(ctx context.Context, id uint) error {
	return s.ctx(ctx).Delete(&models.User{}, id).Error
}

// ---------- sessions ----------

func (s *Store) CreateSession(ctx context.Context, sess *models.Session) error {
	return s.ctx(ctx).Create(sess).Error
}

func (s *Store) Session(ctx context.Context, id string) (*models.Session, error) {
	var sess models.Session
	return &sess, wrap(s.ctx(ctx).First(&sess, "id = ?", id).Error)
}

func (s *Store) RevokeSession(ctx context.Context, id string) error {
	return s.ctx(ctx).Model(&models.Session{}).
		Where("id = ? AND revoked_at IS NULL", id).Update("revoked_at", time.Now()).Error
}

// RevokeUserSessions завершает все сессии пользователя, кроме keep (пусто — все).
// Вызывается после смены пароля: украденный токен перестаёт работать.
func (s *Store) RevokeUserSessions(ctx context.Context, userID uint, keep string) error {
	return s.ctx(ctx).Model(&models.Session{}).
		Where("user_id = ? AND id <> ? AND revoked_at IS NULL", userID, keep).Update("revoked_at", time.Now()).Error
}

// ---------- recurring payments ----------

func (s *Store) CreateRecurring(ctx context.Context, rp *models.RecurringPayment) error {
	return s.ctx(ctx).Create(rp).Error
}

func (s *Store) RecurringByID(ctx context.Context, id uint) (*models.RecurringPayment, error) {
	var rp models.RecurringPayment
	return &rp, wrap(s.ctx(ctx).First(&rp, id).Error) // soft-deleted не находятся
}

// ListRecurring — регулярные платежи, видимые в рамках scope (с логином владельца).
func (s *Store) ListRecurring(ctx context.Context, sc Scope) ([]models.RecurringView, error) {
	q := s.ctx(ctx).Table("recurring_payments AS rp").
		Select("rp.*, u.login AS owner_login").
		Joins("LEFT JOIN users u ON u.id = rp.user_id").
		Where("rp.deleted_at IS NULL")
	var out []models.RecurringView
	return out, sc.apply(q).Order("rp.user_id NULLS FIRST, rp.id").Scan(&out).Error
}

// AllRecurring — все активные регулярные платежи (для планировщика).
func (s *Store) AllRecurring(ctx context.Context) ([]models.RecurringPayment, error) {
	var out []models.RecurringPayment
	return out, s.ctx(ctx).Order("id").Find(&out).Error
}

func (s *Store) CountRecurring(ctx context.Context) (int64, error) {
	var n int64
	return n, s.ctx(ctx).Model(&models.RecurringPayment{}).Count(&n).Error
}

// DeleteRecurring прекращает регулярный платёж: помечает удалённым и убирает
// будущие неоплаченные периоды. Оплаченные и прошедшие периоды (история) остаются.
func (s *Store) DeleteRecurring(ctx context.Context, id uint, today models.Date) error {
	return s.ctx(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&models.RecurringPayment{}, id).Error; err != nil {
			return err
		}
		return tx.Where("recurring_payment_id = ? AND paid_at IS NULL AND due_date > ?", id, today).
			Delete(&models.PaymentOccurrence{}).Error
	})
}

// ---------- occurrences ----------

func (s *Store) MaxDueDate(ctx context.Context, recurringID uint) (*models.Date, error) {
	var d *models.Date
	err := s.ctx(ctx).Model(&models.PaymentOccurrence{}).
		Where("recurring_payment_id = ?", recurringID).
		Select("MAX(due_date)").Row().Scan(&d)
	return d, err
}

// InsertOccurrenceIfAbsent идемпотентен: повторный вызов для того же периода ничего не делает.
func (s *Store) InsertOccurrenceIfAbsent(ctx context.Context, o *models.PaymentOccurrence) error {
	return s.ctx(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "recurring_payment_id"}, {Name: "due_date"}},
		DoNothing: true,
	}).Create(o).Error
}

// Scope — чьи платежи видны.
//   - All (администратор) — все; Owner сужает: "shared" — только общие, "<id>" — личные платежи пользователя;
//   - иначе (пользователь) — его собственные платежи, а с WithShared — ещё и общие (общий список платежей по ТЗ).
//     Чужие личные подписки пользователь не видит; общие платежи меняет только администратор.
type Scope struct {
	All    bool
	UserID uint
	Owner  string
	// WithShared — к личным платежам добавить общие (без владельца): для напоминаний администратору
	WithShared bool
}

func (sc Scope) apply(q *gorm.DB) *gorm.DB {
	switch {
	case sc.Owner == "shared": // общие платежи видят все
		return q.Where("rp.user_id IS NULL")
	case sc.Owner == "mine":
		return q.Where("rp.user_id = ?", sc.UserID)
	case sc.All && sc.Owner != "":
		return q.Where("rp.user_id = ?", sc.Owner)
	case sc.All:
		return q
	}
	if sc.WithShared {
		return q.Where("(rp.user_id IS NULL OR rp.user_id = ?)", sc.UserID)
	}
	return q.Where("rp.user_id = ?", sc.UserID)
}

type OccurrenceFilter struct {
	Scope       Scope
	Currency    string
	RecurringID uint
	From, To    *models.Date
	Status      string
	Today       models.Date
	Desc        bool
	Limit       int
}

func (s *Store) occQuery(ctx context.Context) *gorm.DB {
	return s.ctx(ctx).Table("payment_occurrences AS po").
		Select("po.*, (rp.deleted_at IS NOT NULL) AS recurring_deleted, rp.user_id, u.login AS owner_login, rp.service_id, rp.trial_until, rp.period, rp.auto_renew, " +
			"(SELECT r.status FROM payment_receipts r WHERE r.occurrence_id = po.id ORDER BY r.id DESC LIMIT 1) AS receipt_status").
		Joins("JOIN recurring_payments rp ON rp.id = po.recurring_payment_id").
		Joins("LEFT JOIN users u ON u.id = rp.user_id")
}

func (s *Store) ListOccurrences(ctx context.Context, f OccurrenceFilter) ([]models.OccurrenceView, error) {
	q := f.Scope.apply(s.occQuery(ctx))
	if f.Currency != "" {
		q = q.Where("po.currency = ?", f.Currency)
	}
	if f.RecurringID != 0 {
		q = q.Where("po.recurring_payment_id = ?", f.RecurringID)
	}
	if f.From != nil {
		q = q.Where("po.due_date >= ?", *f.From)
	}
	if f.To != nil {
		q = q.Where("po.due_date <= ?", *f.To)
	}
	switch f.Status {
	case models.StatusPaid:
		q = q.Where("po.paid_at IS NOT NULL")
	case models.StatusUnpaid:
		q = q.Where("po.paid_at IS NULL AND po.due_date >= ?", f.Today)
	case models.StatusOverdue:
		q = q.Where("po.paid_at IS NULL AND po.due_date < ?", f.Today)
	case StatusReview:
		q = q.Where("po.paid_at IS NULL AND " + pendingReceiptSQL)
	}
	if f.Desc {
		q = q.Order("po.due_date DESC, po.id DESC")
	} else {
		q = q.Order("po.due_date ASC, po.id ASC")
	}
	if f.Limit <= 0 || f.Limit > 1000 {
		f.Limit = 1000
	}
	var out []models.OccurrenceView
	return out, q.Limit(f.Limit).Scan(&out).Error
}

func (s *Store) OccurrenceByID(ctx context.Context, id uint) (*models.OccurrenceView, error) {
	var out []models.OccurrenceView
	if err := s.occQuery(ctx).Where("po.id = ?", id).Scan(&out).Error; err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, ErrNotFound
	}
	return &out[0], nil
}

func (s *Store) SetPaid(ctx context.Context, id uint, paidAt *models.Date, paidBy *string) error {
	return s.ctx(ctx).Model(&models.PaymentOccurrence{}).Where("id = ?", id).
		Updates(map[string]any{"paid_at": paidAt, "paid_by": paidBy, "updated_at": time.Now()}).Error
}

// DueReminders — неоплаченные периоды активных ОБЩИХ платежей (для общего Telegram-чата),
// по которым ещё не напоминали и срок которых наступает в пределах horizon (включая просроченные).
// Личные подписки в общий чат не попадают — им напоминание приходит лично.
func (s *Store) DueReminders(ctx context.Context, horizon models.Date) ([]models.OccurrenceView, error) {
	var out []models.OccurrenceView
	err := s.occQuery(ctx).
		Where("po.paid_at IS NULL AND po.reminder_sent_at IS NULL AND po.due_date <= ? AND rp.deleted_at IS NULL AND rp.user_id IS NULL", horizon).
		Where("NOT " + pendingReceiptSQL). // чек уже на проверке — не напоминаем
		Order("po.due_date, po.id").Scan(&out).Error
	return out, err
}

// ClaimReminder атомарно «занимает» отправку: вернёт true только одному вызову.
// Это гарантирует отсутствие дублей даже при параллельных проверках.
func (s *Store) ClaimReminder(ctx context.Context, id uint) (bool, error) {
	res := s.ctx(ctx).Model(&models.PaymentOccurrence{}).
		Where("id = ? AND reminder_sent_at IS NULL AND paid_at IS NULL", id).
		Update("reminder_sent_at", time.Now())
	return res.RowsAffected == 1, res.Error
}

func (s *Store) ReleaseReminder(ctx context.Context, id uint) error {
	return s.ctx(ctx).Model(&models.PaymentOccurrence{}).Where("id = ?", id).
		Update("reminder_sent_at", nil).Error
}

// ---------- summary ----------

func (s *Store) Summary(ctx context.Context, sc Scope, today models.Date, from, to *models.Date) ([]models.CurrencySummary, error) {
	q := s.ctx(ctx).Table("payment_occurrences AS po").
		Joins("JOIN recurring_payments rp ON rp.id = po.recurring_payment_id").Select(`
		po.currency AS currency,
		COALESCE(SUM(po.amount), 0) AS total,
		COALESCE(SUM(po.amount) FILTER (WHERE po.paid_at IS NOT NULL), 0) AS paid,
		COALESCE(SUM(po.amount) FILTER (WHERE po.paid_at IS NULL AND po.due_date >= ?), 0) AS unpaid,
		COALESCE(SUM(po.amount) FILTER (WHERE po.paid_at IS NULL AND po.due_date < ?), 0) AS overdue,
		COUNT(*) AS count_total,
		COUNT(*) FILTER (WHERE po.paid_at IS NOT NULL) AS count_paid,
		COUNT(*) FILTER (WHERE po.paid_at IS NULL AND po.due_date >= ?) AS count_unpaid,
		COUNT(*) FILTER (WHERE po.paid_at IS NULL AND po.due_date < ?) AS count_overdue`,
		today, today, today, today)
	q = sc.apply(q)
	if from != nil {
		q = q.Where("po.due_date >= ?", *from)
	}
	if to != nil {
		q = q.Where("po.due_date <= ?", *to)
	}
	var out []models.CurrencySummary
	return out, q.Group("po.currency").Order("po.currency").Scan(&out).Error
}
