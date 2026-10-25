package store

import (
	"context"

	"invoicekeeper/internal/models"
)

// webhookLogKeep — сколько последних отправок хранить в журнале.
const webhookLogKeep = 200

// AddWebhookDelivery записывает отправку и удаляет записи старше последних 200.
func (s *Store) AddWebhookDelivery(ctx context.Context, d *models.WebhookDelivery) error {
	if err := s.db.WithContext(ctx).Create(d).Error; err != nil {
		return err
	}
	return s.db.WithContext(ctx).Exec(
		`DELETE FROM webhook_deliveries WHERE id <= (SELECT id FROM webhook_deliveries ORDER BY id DESC OFFSET ? LIMIT 1)`,
		webhookLogKeep).Error
}

// WebhookDeliveries — последние отправки, новые сверху.
func (s *Store) WebhookDeliveries(ctx context.Context, limit int) ([]models.WebhookDelivery, error) {
	if limit <= 0 || limit > webhookLogKeep {
		limit = 20
	}
	var out []models.WebhookDelivery
	err := s.db.WithContext(ctx).Order("id DESC").Limit(limit).Find(&out).Error
	return out, err
}
