package store

import (
	"context"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"invoicekeeper/internal/models"
)

// ---------- users: email и Telegram ----------

// UserByLoginOrEmail — вход по логину или по почте.
func (s *Store) UserByLoginOrEmail(ctx context.Context, v string) (*models.User, error) {
	var u models.User
	err := s.ctx(ctx).Where("login = ? OR (email IS NOT NULL AND lower(email) = lower(?))", v, v).
		Order(clause.Expr{SQL: "CASE WHEN login = ? THEN 0 ELSE 1 END, id", Vars: []any{v}}).First(&u).Error
	return &u, wrap(err)
}

func (s *Store) UserByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User
	return &u, wrap(s.ctx(ctx).Where("lower(email) = lower(?)", strings.TrimSpace(email)).First(&u).Error)
}

func (s *Store) UserByTelegramID(ctx context.Context, tgID int64) (*models.User, error) {
	var u models.User
	return &u, wrap(s.ctx(ctx).Where("telegram_id = ? AND telegram_verified_at IS NOT NULL", tgID).First(&u).Error)
}

// UserByTelegramUsername — владелец @имени (без «@»).
func (s *Store) UserByTelegramUsername(ctx context.Context, username string) (*models.User, error) {
	var u models.User
	return &u, wrap(s.ctx(ctx).Where("lower(telegram_username) = lower(?)", username).First(&u).Error)
}

// UpsertTelegramContact запоминает, из какого чата пишет @имя.
func (s *Store) UpsertTelegramContact(ctx context.Context, username string, tgID, chatID int64) error {
	c := models.TelegramContact{Username: strings.ToLower(username), TgID: tgID, ChatID: chatID, UpdatedAt: time.Now()}
	return s.ctx(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "username"}},
		DoUpdates: clause.AssignmentColumns([]string{"tg_id", "chat_id", "updated_at"}),
	}).Create(&c).Error
}

func (s *Store) TelegramContact(ctx context.Context, username string) (*models.TelegramContact, error) {
	var c models.TelegramContact
	return &c, wrap(s.ctx(ctx).Where("username = ?", strings.ToLower(username)).First(&c).Error)
}

// PendingTelegramLinks — пользователи, которым ещё не отправляли запрос на подтверждение.
func (s *Store) PendingTelegramLinks(ctx context.Context) ([]models.User, error) {
	var us []models.User
	return us, s.ctx(ctx).Where("telegram_username IS NOT NULL AND telegram_verified_at IS NULL AND telegram_prompt_at IS NULL").Find(&us).Error
}

func (s *Store) MarkTelegramPrompted(ctx context.Context, userID uint) error {
	return s.ctx(ctx).Model(&models.User{}).Where("id = ?", userID).Update("telegram_prompt_at", time.Now()).Error
}

// VerifyTelegramLink подтверждает привязку; тот же Telegram у другого пользователя снимается.
func (s *Store) VerifyTelegramLink(ctx context.Context, userID uint, tgID int64) error {
	db := s.ctx(ctx)
	if err := db.Model(&models.User{}).Where("telegram_id = ? AND id <> ?", tgID, userID).
		Updates(map[string]any{"telegram_id": nil, "telegram_verified_at": nil}).Error; err != nil {
		return err
	}
	return db.Model(&models.User{}).Where("id = ?", userID).
		Updates(map[string]any{"telegram_id": tgID, "telegram_verified_at": time.Now()}).Error
}

// RejectTelegramLink — «это не я»: привязка полностью сбрасывается.
func (s *Store) RejectTelegramLink(ctx context.Context, userID uint) error {
	return s.ctx(ctx).Model(&models.User{}).Where("id = ?", userID).Updates(map[string]any{
		"telegram_username": nil, "telegram_id": nil, "telegram_verified_at": nil, "telegram_prompt_at": nil,
	}).Error
}

// TelegramRecipients — пользователи с подтверждённым Telegram (личные напоминания по подпискам).
func (s *Store) TelegramRecipients(ctx context.Context) ([]models.User, error) {
	var us []models.User
	return us, s.ctx(ctx).Where("telegram_id IS NOT NULL AND telegram_verified_at IS NOT NULL").Order("id").Find(&us).Error
}

// EmailRecipients — пользователи, включившие email-напоминания.
func (s *Store) EmailRecipients(ctx context.Context) ([]models.User, error) {
	var us []models.User
	return us, s.ctx(ctx).Where("email_notifications AND email IS NOT NULL AND email <> ''").Order("id").Find(&us).Error
}

// ---------- чек к оплате ----------

func (s *Store) SetReceipt(ctx context.Context, id uint, url *string) error {
	return s.ctx(ctx).Model(&models.PaymentOccurrence{}).Where("id = ?", id).
		Updates(map[string]any{"receipt_url": url, "updated_at": time.Now()}).Error
}

// ---------- бюджеты ----------

func (s *Store) ListBudgets(ctx context.Context, userID uint, period string) ([]models.Budget, error) {
	q := s.ctx(ctx).Where("user_id = ?", userID)
	if period != "" {
		q = q.Where("period = ?", period)
	}
	var out []models.Budget
	return out, q.Order("period, currency").Find(&out).Error
}

// UpsertBudget создаёт или обновляет личный бюджет пользователя b.UserID для пары (валюта, период).
func (s *Store) UpsertBudget(ctx context.Context, b *models.Budget) error {
	return s.ctx(ctx).Transaction(func(tx *gorm.DB) error {
		var cur models.Budget
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND currency = ? AND period = ?", b.UserID, b.Currency, b.Period).First(&cur).Error
		if err == gorm.ErrRecordNotFound {
			return tx.Create(b).Error
		}
		if err != nil {
			return err
		}
		cur.Amount = b.Amount
		if err := tx.Save(&cur).Error; err != nil {
			return err
		}
		*b = cur
		return nil
	})
}

func (s *Store) DeleteBudget(ctx context.Context, userID, id uint) (bool, error) {
	res := s.ctx(ctx).Where("user_id = ?", userID).Delete(&models.Budget{}, id)
	return res.RowsAffected > 0, res.Error
}

// ---------- настройки ----------

func (s *Store) Settings(ctx context.Context) (map[string]string, error) {
	var rows []models.AppSetting
	if err := s.ctx(ctx).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.Key] = r.Value
	}
	return out, nil
}

// Setting — одно значение настройки; ErrNotFound, если не задано.
func (s *Store) Setting(ctx context.Context, key string) (string, error) {
	var row models.AppSetting
	if err := s.ctx(ctx).Where("key = ?", key).First(&row).Error; err != nil {
		return "", wrap(err)
	}
	return row.Value, nil
}

// ResetUnpaidReminders — забыть отправленные напоминания по неоплаченным периодам (машина времени назад).
func (s *Store) ResetUnpaidReminders(ctx context.Context) error {
	if err := s.ctx(ctx).Exec(`DELETE FROM notification_deliveries WHERE occurrence_id IN
		(SELECT id FROM payment_occurrences WHERE paid_at IS NULL)`).Error; err != nil {
		return err
	}
	return s.ctx(ctx).Exec(`UPDATE payment_occurrences SET reminder_sent_at = NULL WHERE paid_at IS NULL`).Error
}

func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	return s.ctx(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&models.AppSetting{Key: key, Value: value, UpdatedAt: time.Now()}).Error
}

// ---------- напоминания по каналам (email, webhook) ----------

// DueRemindersFor — неоплаченные периоды, по которым пора напомнить в этот канал.
// У напоминания несколько ступеней (за 7, 3 и 1 день, затем «просрочено»): на каждую ступень — одно письмо.
// Ступень хранится в ключе доставки «канал#d3». Старый ключ без ступени считается «уже напоминали всё».
// scope ограничивает, чьи платежи попадают в канал (email и Telegram пользователя — только его платежи).
func (s *Store) DueRemindersFor(ctx context.Context, channel string, sc Scope, today models.Date, stages []int, limit int) ([]models.OccurrenceView, error) {
	if len(stages) == 0 {
		return nil, nil
	}
	// CASE по ступеням: просрочено → od, иначе самая узкая подходящая из stages (по возрастанию)
	expr := "CASE WHEN po.due_date < ? THEN 'od'"
	args := []any{today}
	for i := len(stages) - 1; i >= 0; i-- {
		if i == 0 {
			expr += " ELSE 'd" + strconv.Itoa(stages[0]) + "' END"
			break
		}
		expr += " WHEN po.due_date <= ? THEN 'd" + strconv.Itoa(stages[i]) + "'"
		args = append(args, today.AddDays(stages[i]))
	}
	q := sc.apply(s.occQuery(ctx)).
		Where("po.paid_at IS NULL AND po.due_date <= ? AND rp.deleted_at IS NULL", today.AddDays(stages[0])).
		Where("NOT "+pendingReceiptSQL).
		Where("NOT EXISTS (SELECT 1 FROM notification_deliveries nd WHERE nd.occurrence_id = po.id AND (nd.channel = ? OR nd.channel = ? || '#' || ("+expr+")))",
			append([]any{channel, channel}, args...)...)
	if channel == "chat" {
		q = q.Where("po.reminder_sent_at IS NULL") // общий чат раньше отмечал отправку здесь
	}
	var out []models.OccurrenceView
	return out, q.Order("po.due_date, po.id").Limit(limit).Scan(&out).Error
}

// ClaimDelivery атомарно занимает отправку в канал: true вернётся только один раз
// и только если период всё ещё не оплачен. Первичный ключ (occurrence_id, channel) исключает дубли.
func (s *Store) ClaimDelivery(ctx context.Context, occurrenceID uint, channel string) (bool, error) {
	res := s.ctx(ctx).Exec(`INSERT INTO notification_deliveries (occurrence_id, channel)
		SELECT id, ? FROM payment_occurrences WHERE id = ? AND paid_at IS NULL
		ON CONFLICT DO NOTHING`, channel, occurrenceID)
	return res.RowsAffected == 1, res.Error
}

func (s *Store) ReleaseDelivery(ctx context.Context, occurrenceID uint, channel string) error {
	return s.ctx(ctx).Exec(`DELETE FROM notification_deliveries WHERE occurrence_id = ? AND channel = ?`,
		occurrenceID, channel).Error
}

// ---------- бюджет: суммы за период ----------

// OccurrencesBetween — все периоды (включая прекращённые платежи) со сроком в [from, to].
func (s *Store) OccurrencesBetween(ctx context.Context, sc Scope, from, to models.Date) ([]models.OccurrenceView, error) {
	var out []models.OccurrenceView
	return out, sc.apply(s.occQuery(ctx)).Where("po.due_date BETWEEN ? AND ?", from, to).Order("po.due_date").Scan(&out).Error
}

// LinkTelegramDirect — подключение по ссылке из сайта (/start с кодом): сразу подтверждено.
// Этот Telegram и это @имя снимаются с других пользователей, чтобы не было двух владельцев.
func (s *Store) LinkTelegramDirect(ctx context.Context, userID uint, tgID int64, username string) error {
	if userID == 0 || tgID == 0 {
		return ErrNotFound
	}
	return s.ctx(ctx).Transaction(func(tx *gorm.DB) error {
		clear := map[string]any{"telegram_id": nil, "telegram_verified_at": nil, "telegram_prompt_at": nil, "telegram_username": nil}
		q := tx.Model(&models.User{}).Where("id <> ?", userID)
		if username != "" {
			q = q.Where("telegram_id = ? OR lower(telegram_username) = lower(?)", tgID, username)
		} else {
			q = q.Where("telegram_id = ?", tgID)
		}
		if err := q.Updates(clear).Error; err != nil {
			return err
		}
		var name any
		if username != "" {
			name = username
		}
		now := time.Now()
		return tx.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]any{
			"telegram_id": tgID, "telegram_verified_at": now, "telegram_prompt_at": now, "telegram_username": name,
		}).Error
	})
}

// ResetTelegramPrompt — человек нажал /start: неподтверждённый запрос по его @имени отправим заново
// (например, если прошлый не дошёл, потому что он ещё не писал боту).
func (s *Store) ResetTelegramPrompt(ctx context.Context, username string) error {
	return s.ctx(ctx).Model(&models.User{}).
		Where("lower(telegram_username) = lower(?) AND telegram_verified_at IS NULL", username).
		Update("telegram_prompt_at", nil).Error
}
