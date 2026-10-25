package store

import (
	"context"
	"time"

	"invoicekeeper/internal/models"
)

// ---------- каталог услуг ----------

func (s *Store) CreateService(ctx context.Context, sv *models.Service) error {
	return s.ctx(ctx).Create(sv).Error
}

func (s *Store) SaveService(ctx context.Context, sv *models.Service) error {
	sv.UpdatedAt = time.Now()
	return s.ctx(ctx).Save(sv).Error
}

func (s *Store) ServiceByID(ctx context.Context, id uint) (*models.Service, error) {
	var sv models.Service
	return &sv, wrap(s.ctx(ctx).First(&sv, id).Error) // удалённые не находятся
}

func (s *Store) DeleteService(ctx context.Context, id uint) error {
	return s.ctx(ctx).Delete(&models.Service{}, id).Error
}

func (s *Store) CountServices(ctx context.Context) (int64, error) {
	var n int64
	return n, s.ctx(ctx).Unscoped().Model(&models.Service{}).Count(&n).Error
}

// ListServices — каталог: подписка userID на каждую услугу и число активных подписчиков.
// includeHidden — показать и скрытые услуги (для администратора).
func (s *Store) ListServices(ctx context.Context, userID uint, includeHidden bool) ([]models.ServiceView, error) {
	q := s.ctx(ctx).Table("services AS sv").Select(`sv.*,
		(SELECT rp.id FROM recurring_payments rp WHERE rp.service_id = sv.id AND rp.user_id = ? AND rp.deleted_at IS NULL LIMIT 1) AS subscription_id,
		(SELECT COUNT(*) FROM recurring_payments rp WHERE rp.service_id = sv.id AND rp.deleted_at IS NULL) AS subscribers`, userID).
		Where("sv.deleted_at IS NULL")
	if !includeHidden {
		q = q.Where("sv.active")
	}
	var out []models.ServiceView
	return out, q.Order("sv.id").Scan(&out).Error
}

// ActiveSubscription — действующая подписка пользователя на услугу.
func (s *Store) ActiveSubscription(ctx context.Context, userID, serviceID uint) (*models.RecurringPayment, error) {
	var rp models.RecurringPayment
	return &rp, wrap(s.ctx(ctx).Where("user_id = ? AND service_id = ?", userID, serviceID).First(&rp).Error)
}
