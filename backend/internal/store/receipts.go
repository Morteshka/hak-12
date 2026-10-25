package store

import (
	"context"
	"strconv"
	"time"

	"invoicekeeper/internal/models"
)

// StatusReview — фильтр списка платежей: не оплачено, чек на проверке.
const StatusReview = "review"

// pendingReceiptSQL — у периода есть чек, ожидающий решения администратора.
const pendingReceiptSQL = "EXISTS (SELECT 1 FROM payment_receipts pr WHERE pr.occurrence_id = po.id AND pr.status = 'pending')"

func (s *Store) CreateReceipt(ctx context.Context, r *models.Receipt) error {
	return s.ctx(ctx).Create(r).Error
}

func (s *Store) ReceiptByID(ctx context.Context, id uint) (*models.Receipt, error) {
	var r models.Receipt
	return &r, wrap(s.ctx(ctx).First(&r, id).Error)
}

// ReceiptContent — сам файл (в списках не читается: он может весить мегабайты).
func (s *Store) ReceiptContent(ctx context.Context, id uint) ([]byte, error) {
	var out []struct{ Content []byte }
	if err := s.ctx(ctx).Table("payment_receipts").Select("content").Where("id = ?", id).Scan(&out).Error; err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, ErrNotFound
	}
	return out[0].Content, nil
}

type ReceiptFilter struct {
	Status       string
	OccurrenceID uint
	UserID       *uint // только чеки этого пользователя и общих платежей (для роли user)
	Limit        int
}

func (s *Store) ListReceipts(ctx context.Context, f ReceiptFilter) ([]models.Receipt, error) {
	q := s.ctx(ctx).Model(&models.Receipt{})
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.OccurrenceID != 0 {
		q = q.Where("occurrence_id = ?", f.OccurrenceID)
	}
	if f.UserID != nil {
		q = q.Where("user_id = ?", *f.UserID)
	}
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 200
	}
	order := "id DESC"
	if f.Status == models.ReceiptPending {
		order = "id ASC" // очередь: сначала самые старые
	}
	var out []models.Receipt
	return out, q.Order(order).Limit(f.Limit).Find(&out).Error
}

func (s *Store) CountReceipts(ctx context.Context, status string) (int64, error) {
	var n int64
	return n, s.ctx(ctx).Model(&models.Receipt{}).Where("status = ?", status).Count(&n).Error
}

// ReceiptDuplicates — другие не отклонённые чеки с тем же файлом или номером операции.
func (s *Store) ReceiptDuplicates(ctx context.Context, sha, operationID string, exceptOccurrence uint) ([]models.Receipt, error) {
	q := s.ctx(ctx).Model(&models.Receipt{}).Where("status <> ?", models.ReceiptRejected)
	if operationID != "" {
		q = q.Where("(sha256 = ? OR ext_operation_id = ?)", sha, operationID)
	} else {
		q = q.Where("sha256 = ?", sha)
	}
	// тот же файл к тому же платежу повторно — тоже дубль, но только если он уже принят
	q = q.Where("(occurrence_id IS DISTINCT FROM ? OR status = ?)", exceptOccurrence, models.ReceiptApproved)
	var out []models.Receipt
	return out, q.Order("id").Limit(5).Find(&out).Error
}

// SupersedePending — прежние чеки периода, ещё ждущие проверки, заменены новым.
func (s *Store) SupersedePending(ctx context.Context, occurrenceID, newID uint) (int64, error) {
	res := s.ctx(ctx).Model(&models.Receipt{}).
		Where("occurrence_id = ? AND status = ? AND id <> ?", occurrenceID, models.ReceiptPending, newID).
		Updates(map[string]any{"status": models.ReceiptRejected, "decided_by": "system", "decided_auto": true,
			"decision_comment": "Заменён новым чеком", "decided_at": time.Now(), "updated_at": time.Now()})
	return res.RowsAffected, res.Error
}

// DecideReceipt — атомарно меняет статус, только если чек ещё ждёт решения (двойное нажатие не пройдёт).
func (s *Store) DecideReceipt(ctx context.Context, id uint, status, by, comment string, auto bool) (bool, error) {
	now := time.Now()
	res := s.ctx(ctx).Model(&models.Receipt{}).Where("id = ? AND status = ?", id, models.ReceiptPending).
		Updates(map[string]any{"status": status, "decided_by": by, "decided_auto": auto, "decision_comment": comment,
			"decided_at": now, "updated_at": now})
	return res.RowsAffected == 1, res.Error
}

// SaveReceiptAnalysis — результат повторного распознавания.
func (s *Store) SaveReceiptAnalysis(ctx context.Context, r *models.Receipt) error {
	return s.ctx(ctx).Model(&models.Receipt{}).Where("id = ?", r.ID).Updates(map[string]any{
		"methods": r.Methods, "ext_amount": r.ExtAmount, "ext_currency": r.ExtCurrency, "ext_date": r.ExtDate,
		"ext_operation_id": r.ExtOperationID, "ext_payee": r.ExtPayee, "ext_text": r.ExtText, "ai_comment": r.AIComment,
		"checks": r.Checks, "score": r.Score, "updated_at": time.Now(),
	}).Error
}

// ApprovePendingForOccurrence — администратор отметил оплату вручную: ожидающие чеки считаются принятыми.
func (s *Store) ApprovePendingForOccurrence(ctx context.Context, occurrenceID uint, by string) error {
	now := time.Now()
	return s.ctx(ctx).Model(&models.Receipt{}).Where("occurrence_id = ? AND status = ?", occurrenceID, models.ReceiptPending).
		Updates(map[string]any{"status": models.ReceiptApproved, "decided_by": by, "decided_auto": false,
			"decision_comment": "Оплата отмечена администратором", "decided_at": now, "updated_at": now}).Error
}

// ReceiptHistory — записи журнала по чеку (всё, что с ним происходило).
func (s *Store) ReceiptHistory(ctx context.Context, id uint) ([]models.AuditLog, error) {
	var out []models.AuditLog
	return out, s.ctx(ctx).Table("audit_logs").Where("entity_type = 'receipt' AND entity_id = ?", strconv.FormatUint(uint64(id), 10)).
		Order("id").Find(&out).Error
}

// AdminTelegramChats — личные чаты администраторов с подтверждённым Telegram.
func (s *Store) AdminTelegramChats(ctx context.Context) ([]int64, error) {
	var ids []int64
	return ids, s.ctx(ctx).Model(&models.User{}).
		Where("role = ? AND telegram_id IS NOT NULL AND telegram_verified_at IS NOT NULL", models.RoleAdmin).
		Pluck("telegram_id", &ids).Error
}
