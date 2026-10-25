package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
	"unicode/utf8"

	"invoicekeeper/internal/models"
	"invoicekeeper/internal/store"
)

// Действия журнала (названия — как в спецификации).
const (
	ActLogin          = "login"
	ActLoginFailed    = "login_failed"
	ActLogout         = "logout"
	ActRegister       = "register"
	ActPasswordChange = "password_change"
	ActCreatePayment  = "create_payment"
	ActDeletePayment  = "delete_payment"
	ActEditPayment    = "edit_payment"      // администратор изменил платёж (сумма, график, название)
	ActAutoRenew      = "toggle_auto_renew" // включено/выключено автопродление
	ActMarkPaid       = "mark_paid"
	ActUnmarkPaid     = "unmark_paid"
	ActEditDate       = "edit_date"
	ActReceipt        = "update_receipt"
	ActCreateUser     = "create_user"
	ActUpdateUser     = "update_user"
	ActDeleteUser     = "delete_user"
	ActBudget         = "update_budget"
	ActSettings       = "update_settings"
	ActNotifyFailed   = "notification_failed"
	ActSubscribe      = "subscribe"
	ActUnsubscribe    = "unsubscribe"
	ActCreateService  = "create_service"
	ActUpdateService  = "update_service"
	ActDeleteService  = "delete_service"
	ActTestReminder   = "test_reminder"
	ActReceiptSubmit  = "receipt_submit"       // пользователь приложил чек
	ActReceiptAuto    = "receipt_auto_approve" // автопроверка подтвердила оплату
	ActReceiptApprove = "receipt_approve"      // администратор подтвердил
	ActReceiptReject  = "receipt_reject"       // администратор отклонил (или чек заменён новым)
	ActReceiptRecheck = "receipt_recheck"      // повторное распознавание
	ActTimeTravel     = "time_travel"          // машина времени: сдвиг системной даты (демо)
	ActImport         = "import_statement"     // платежи добавлены из банковской выписки
)

// Audit пишет действия пользователей в БД и stdout.
type Audit struct{ st *store.Store }

func NewAudit(st *store.Store) *Audit { return &Audit{st: st} }

// D — короткая запись для деталей журнала.
type D = map[string]any

// Log записывает действие. details — строка (сохранится как {"text": ...}) или map/struct (сохранится как JSON).
func (a *Audit) Log(ctx context.Context, actor Actor, action, entityType string, entityID any, details any) {
	e := &models.AuditLog{
		UserID: actor.UserID, Actor: actor.Login, Action: action, EntityType: entityType,
		IP: actor.IP, UserAgent: truncate(actor.UserAgent, 512), Details: detailsJSON(details),
	}
	if entityID != nil {
		e.EntityID = fmt.Sprint(entityID)
	}
	if err := a.st.CreateAudit(ctx, e); err != nil {
		log.Printf("audit: %v", err)
	}
	log.Printf("AUDIT actor=%q action=%s entity=%s id=%s %s", actor.Login, action, entityType, e.EntityID, e.Details)
}

func detailsJSON(v any) json.RawMessage {
	switch t := v.(type) {
	case nil:
		return json.RawMessage("{}")
	case string:
		if t == "" {
			return json.RawMessage("{}")
		}
		v = D{"text": t}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("{}")
	}
	return b
}

// truncate обрезает до n байт, не разрывая символы UTF-8 (кириллица — 2 байта).
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// AuditQuery — фильтры журнала. Даты включительно, в часовом поясе системы.
type AuditQuery struct {
	From, To *models.Date
	UserID   *uint
	Action   string
	Limit    int
}

// List — записи журнала с фильтрами (только для администратора — проверяется в роутере и здесь).
func (a *Audit) List(ctx context.Context, actor Actor, q AuditQuery, loc *time.Location) ([]models.AuditLog, error) {
	if err := requireAdmin(actor); err != nil {
		return nil, err
	}
	if q.Limit <= 0 || q.Limit > 1000 {
		q.Limit = 300
	}
	f := store.AuditFilter{UserID: q.UserID, Action: q.Action, Limit: q.Limit}
	if q.From != nil {
		t := time.Date(q.From.Year(), q.From.Month(), q.From.Day(), 0, 0, 0, 0, loc)
		f.From = &t
	}
	if q.To != nil {
		t := time.Date(q.To.Year(), q.To.Month(), q.To.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, 1)
		f.To = &t
	}
	return a.st.ListAudit(ctx, f)
}

// Actions — какие действия есть в журнале (для выпадающего фильтра).
func (a *Audit) Actions(ctx context.Context, actor Actor) ([]string, error) {
	if err := requireAdmin(actor); err != nil {
		return nil, err
	}
	return a.st.AuditActions(ctx)
}
