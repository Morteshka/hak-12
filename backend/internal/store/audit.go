package store

import (
	"context"
	"encoding/json"
	"time"

	"invoicekeeper/internal/models"
)

// CreateAudit пишет запись журнала. details — JSON-текст, приводится к JSONB в самом запросе.
func (s *Store) CreateAudit(ctx context.Context, a *models.AuditLog) error {
	details := string(a.Details)
	if details == "" {
		details = "{}"
	}
	return s.ctx(ctx).Exec(`INSERT INTO audit_logs (user_id, actor, action, entity_type, entity_id, details, ip, user_agent)
		VALUES (?, ?, ?, ?, ?, ?::jsonb, ?, ?)`,
		a.UserID, a.Actor, a.Action, a.EntityType, a.EntityID, details, a.IP, a.UserAgent).Error
}

type AuditFilter struct {
	From, To *time.Time // created_at >= From, created_at < To
	UserID   *uint
	Action   string
	Limit    int
}

type auditRow struct {
	ID         uint
	UserID     *uint
	Actor      string
	Action     string
	EntityType string
	EntityID   string
	Details    string
	IP         string
	UserAgent  string
	CreatedAt  time.Time
}

func (s *Store) ListAudit(ctx context.Context, f AuditFilter) ([]models.AuditLog, error) {
	q := s.ctx(ctx).Table("audit_logs").Select(`id, user_id, actor, action, entity_type, entity_id,
		details::text AS details, ip, user_agent, created_at`)
	if f.From != nil {
		q = q.Where("created_at >= ?", *f.From)
	}
	if f.To != nil {
		q = q.Where("created_at < ?", *f.To)
	}
	if f.UserID != nil {
		q = q.Where("user_id = ?", *f.UserID)
	}
	if f.Action != "" {
		q = q.Where("action = ?", f.Action)
	}
	var rows []auditRow
	if err := q.Order("created_at DESC, id DESC").Limit(f.Limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]models.AuditLog, len(rows))
	for i, r := range rows {
		d := json.RawMessage(r.Details)
		if !json.Valid(d) {
			d = json.RawMessage("{}")
		}
		out[i] = models.AuditLog{ID: r.ID, UserID: r.UserID, Actor: r.Actor, Action: r.Action, EntityType: r.EntityType,
			EntityID: r.EntityID, Details: d, IP: r.IP, UserAgent: r.UserAgent, CreatedAt: r.CreatedAt}
	}
	return out, nil
}

// AuditActions — список действий, встречающихся в журнале (для фильтра).
func (s *Store) AuditActions(ctx context.Context) ([]string, error) {
	var out []string
	return out, s.ctx(ctx).Table("audit_logs").Distinct("action").Order("action").Pluck("action", &out).Error
}
