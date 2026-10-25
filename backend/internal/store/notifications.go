package store

import (
	"context"
	"time"

	"invoicekeeper/internal/models"
)

// ---------- уведомления в веб-панели (колокольчик) ----------

func (s *Store) CreateNotification(ctx context.Context, n *models.Notification) error {
	return s.ctx(ctx).Create(n).Error
}

// ListNotifications — последние уведомления пользователя, новые сверху.
func (s *Store) ListNotifications(ctx context.Context, userID uint, unreadOnly bool, limit int) ([]models.Notification, error) {
	q := s.ctx(ctx).Where("user_id = ?", userID)
	if unreadOnly {
		q = q.Where("read_at IS NULL")
	}
	var out []models.Notification
	return out, q.Order("id DESC").Limit(limit).Find(&out).Error
}

func (s *Store) UnreadNotifications(ctx context.Context, userID uint) (int64, error) {
	var n int64
	return n, s.ctx(ctx).Model(&models.Notification{}).Where("user_id = ? AND read_at IS NULL", userID).Count(&n).Error
}

// MarkNotificationsRead отмечает прочитанными указанные уведомления (ids пусто — все).
func (s *Store) MarkNotificationsRead(ctx context.Context, userID uint, ids []uint) error {
	q := s.ctx(ctx).Model(&models.Notification{}).Where("user_id = ? AND read_at IS NULL", userID)
	if len(ids) > 0 {
		q = q.Where("id IN ?", ids)
	}
	return q.Update("read_at", time.Now()).Error
}

// DeleteNotifications очищает уведомления пользователя (только прочитанные, если readOnly).
func (s *Store) DeleteNotifications(ctx context.Context, userID uint, readOnly bool) error {
	q := s.ctx(ctx).Where("user_id = ?", userID)
	if readOnly {
		q = q.Where("read_at IS NOT NULL")
	}
	return q.Delete(&models.Notification{}).Error
}

// PruneNotifications удаляет прочитанные уведомления старше срока (чтобы таблица не росла).
func (s *Store) PruneNotifications(ctx context.Context, olderThan time.Duration) error {
	return s.ctx(ctx).Where("read_at IS NOT NULL AND created_at < ?", time.Now().Add(-olderThan)).Delete(&models.Notification{}).Error
}

// ActiveUserIDs — получатели общих уведомлений: все пользователи (или только администраторы).
func (s *Store) ActiveUserIDs(ctx context.Context, adminsOnly bool) ([]uint, error) {
	q := s.ctx(ctx).Model(&models.User{})
	if adminsOnly {
		q = q.Where("role = ?", models.RoleAdmin)
	}
	var ids []uint
	return ids, q.Order("id").Pluck("id", &ids).Error
}

// ---------- редактирование регулярного платежа ----------

func (s *Store) UpdateRecurringFields(ctx context.Context, id uint, fields map[string]any) error {
	fields["updated_at"] = time.Now()
	return s.ctx(ctx).Model(&models.RecurringPayment{}).Where("id = ?", id).Updates(fields).Error
}

// UpdateUnpaidOccurrences переносит новое название, сумму и валюту на неоплаченные периоды, срок которых не прошёл.
func (s *Store) UpdateUnpaidOccurrences(ctx context.Context, recurringID uint, from models.Date, fields map[string]any) error {
	fields["updated_at"] = time.Now()
	return s.ctx(ctx).Model(&models.PaymentOccurrence{}).
		Where("recurring_payment_id = ? AND paid_at IS NULL AND due_date >= ?", recurringID, from).Updates(fields).Error
}

// DeleteUnpaidFrom убирает неоплаченные периоды со сроком не раньше from (оплаченные и прошедшие остаются).
func (s *Store) DeleteUnpaidFrom(ctx context.Context, recurringID uint, from models.Date) error {
	return s.ctx(ctx).Where("recurring_payment_id = ? AND paid_at IS NULL AND due_date >= ?", recurringID, from).
		Delete(&models.PaymentOccurrence{}).Error
}

// ---------- итоги по месяцам ----------

// MonthlySummary — по каждому месяцу и валюте: оплачено, просрочено, предстоит. Валюты не конвертируются.
func (s *Store) MonthlySummary(ctx context.Context, sc Scope, today, from, to models.Date) ([]models.MonthSummary, error) {
	q := s.ctx(ctx).Table("payment_occurrences AS po").
		Joins("JOIN recurring_payments rp ON rp.id = po.recurring_payment_id").
		Select(`to_char(po.due_date, 'YYYY-MM') AS month, po.currency AS currency,
		COALESCE(SUM(po.amount), 0) AS total,
		COALESCE(SUM(po.amount) FILTER (WHERE po.paid_at IS NOT NULL), 0) AS paid,
		COALESCE(SUM(po.amount) FILTER (WHERE po.paid_at IS NULL AND po.due_date < ?), 0) AS overdue,
		COALESCE(SUM(po.amount) FILTER (WHERE po.paid_at IS NULL AND po.due_date >= ?), 0) AS upcoming,
		COUNT(*) AS count,
		COUNT(*) FILTER (WHERE po.paid_at IS NOT NULL) AS count_paid,
		COUNT(*) FILTER (WHERE po.paid_at IS NULL AND po.due_date < ?) AS count_overdue`, today, today, today).
		Where("po.due_date BETWEEN ? AND ?", from, to)
	var out []models.MonthSummary
	return out, sc.apply(q).Group("1, 2").Order("1 DESC, 2").Scan(&out).Error
}

// RecurringView — один регулярный платёж с логином владельца.
func (s *Store) RecurringView(ctx context.Context, id uint) (*models.RecurringView, error) {
	var out models.RecurringView
	res := s.ctx(ctx).Table("recurring_payments AS rp").
		Select("rp.*, u.login AS owner_login").
		Joins("LEFT JOIN users u ON u.id = rp.user_id").
		Where("rp.id = ? AND rp.deleted_at IS NULL", id).Limit(1).Scan(&out)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	return &out, nil
}

// NextUnpaidDue — срок ближайшего неоплаченного периода не раньше from (nil — такого нет).
func (s *Store) NextUnpaidDue(ctx context.Context, recurringID uint, from models.Date) (*models.Date, error) {
	var d *models.Date
	err := s.ctx(ctx).Model(&models.PaymentOccurrence{}).
		Where("recurring_payment_id = ? AND paid_at IS NULL AND due_date >= ?", recurringID, from).
		Select("MIN(due_date)").Row().Scan(&d)
	return d, err
}
