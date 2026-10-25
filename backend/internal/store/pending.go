package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"invoicekeeper/internal/models"
)

// PendingRegistration — ожидающая регистрация по почте (без учёта регистра).
func (s *Store) PendingRegistration(ctx context.Context, email string) (*models.PendingRegistration, error) {
	var p models.PendingRegistration
	err := s.db.WithContext(ctx).Where("lower(email) = ?", strings.ToLower(email)).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &p, err
}

// SavePendingRegistration создаёт или обновляет запись и заодно чистит давно истёкшие.
func (s *Store) SavePendingRegistration(ctx context.Context, p *models.PendingRegistration) error {
	db := s.db.WithContext(ctx)
	if err := db.Exec(`DELETE FROM pending_registrations WHERE expires_at < ? OR lower(email) = ? AND email <> ?`,
		time.Now().Add(-24*time.Hour), strings.ToLower(p.Email), p.Email).Error; err != nil {
		return err
	}
	return db.Save(p).Error
}

func (s *Store) DeletePendingRegistration(ctx context.Context, email string) error {
	return s.db.WithContext(ctx).Where("lower(email) = ?", strings.ToLower(email)).Delete(&models.PendingRegistration{}).Error
}
