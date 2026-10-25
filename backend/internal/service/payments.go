package service

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shopspring/decimal"

	"invoicekeeper/internal/models"
	"invoicekeeper/internal/store"
)

type PaymentService struct {
	st       *store.Store
	audit    *Audit
	loc      *time.Location
	stages   []int // за сколько дней до срока напоминать: 7, 3, 1
	events   EventSink
	onChange func() // «разбудить» рассылку напоминаний (новый платёж — напоминание сразу)
	// requireReceipt — пользователь подтверждает оплату только чеком (см. ReceiptService)
	requireReceipt func(ctx context.Context, a Actor) bool
	// receiptsPaidByAdmin — администратор отметил оплату вручную: ожидающие чеки принимаются
	receiptsPaidByAdmin func(ctx context.Context, occID uint, by string)
	// машина времени для демо (см. clock.go)
	timeTravel bool
	clk        clockCache
}

// RequiresReceipt — нужно ли этому пользователю подтверждать оплату чеком.
func (s *PaymentService) RequiresReceipt(ctx context.Context, a Actor) bool {
	return s.requireReceipt != nil && s.requireReceipt(ctx, a)
}

// ErrReceiptRequired — пользователь пытается отметить оплату без чека.
var ErrReceiptRequired = Invalid("receipt required: attach a payment receipt (POST /api/v1/payment-occurrences/{id}/receipts)")

// OnChange — вызывается после добавления платежа/подписки, чтобы напоминание ушло без ожидания тика.
func (s *PaymentService) OnChange(fn func()) { s.onChange = fn }

func (s *PaymentService) kick() {
	if s.onChange != nil {
		s.onChange()
	}
}

// wrapDup — нарушение уникального индекса (повторная подписка) как store.ErrDuplicate.
func wrapDup(err error) error {
	if errors.Is(err, store.ErrDuplicate) || strings.Contains(err.Error(), "uq_active_subscription") || strings.Contains(err.Error(), "SQLSTATE 23505") {
		return store.ErrDuplicate
	}
	return err
}

// EventSink получает события платежей (webhook). Отправка — асинхронная, ошибки не роняют API.
type EventSink interface {
	Emit(ev models.PaymentEvent)
}

// SetEvents подключает получателя событий.
func (s *PaymentService) SetEvents(e EventSink) { s.events = e }

// Loc — единый часовой пояс системы.
func (s *PaymentService) Loc() *time.Location { return s.loc }

// ReminderStages — за сколько дней до срока напоминать (по убыванию: 7, 3, 1).
func (s *PaymentService) ReminderStages() []int { return s.stages }

func (s *PaymentService) emit(event string, v *models.OccurrenceView, actor string) {
	if s.events == nil || v == nil {
		return
	}
	s.events.Emit(EventFor(event, v, actor))
}

// EventFor собирает тело события из периода платежа.
func EventFor(event string, v *models.OccurrenceView, actor string) models.PaymentEvent {
	ev := models.PaymentEvent{
		Event: event, PaymentID: v.ID, RecurringID: v.RecurringPaymentID, Name: v.Title,
		Amount: v.Amount.StringFixed(2), Currency: v.Currency, DueDate: v.DueDate.String(),
		Status: v.Status, ReceiptURL: v.ReceiptURL, Actor: actor, Owner: v.OwnerLogin, Timestamp: time.Now().UTC(),
	}
	if v.PaidAt != nil {
		p := v.PaidAt.String()
		ev.PaidAt = &p
	}
	return ev
}

func NewPaymentService(st *store.Store, audit *Audit, loc *time.Location, stages []int) *PaymentService {
	if len(stages) == 0 {
		stages = []int{7, 3, 1}
	}
	return &PaymentService{st: st, audit: audit, loc: loc, stages: stages}
}

// Today — «сегодня» в едином часовом поясе системы (с учётом сдвига машины времени).
func (s *PaymentService) Today() models.Date {
	return models.DateOf(time.Now().In(s.loc).AddDate(0, 0, s.Offset()))
}

// horizon — до какой даты заранее создаём периоды (чтобы следующий период был виден заранее).
func (s *PaymentService) horizon() models.Date {
	return models.DateOf(s.Today().Time.AddDate(0, 1, 0))
}

// ---------- права и видимость ----------

// ScopeFor — какие платежи видит actor. owner: "" — все доступные; "shared" — общие платежи;
// "mine" — личные подписки actor; число — личные платежи этого пользователя (только администратор).
func ScopeFor(a Actor, owner string) (store.Scope, error) {
	sc := store.Scope{All: a.Role == models.RoleAdmin, Owner: owner}
	if a.UserID != nil {
		sc.UserID = *a.UserID
	}
	// пользователь видит общий список платежей (ТЗ) и свои личные подписки; чужие подписки — нет
	sc.WithShared = !sc.All
	switch owner {
	case "", "shared":
	case "mine":
		if a.UserID == nil {
			return sc, Invalid("owner=mine requires a user")
		}
	default:
		if _, err := strconv.ParseUint(owner, 10, 64); err != nil {
			return sc, Invalid("owner must be shared, mine or a user id")
		}
		if !sc.All {
			return sc, Forbidden("forbidden: admin role required")
		}
	}
	return sc, nil
}

// canSee — общие платежи (без владельца) видят все; личный платёж — владелец и администратор.
// Менять общие платежи (сумма, график, удаление) может только администратор — это проверяется отдельно,
// а отмечать оплату и исправлять дату оплаты могут все (по ТЗ пользователь «отмечает оплату и следит за просрочками»).
func canSee(a Actor, owner *uint) bool {
	return a.Role == models.RoleAdmin || owner == nil || (a.UserID != nil && *a.UserID == *owner)
}

// ---------- регулярные платежи ----------

type CreateRecurringInput struct {
	Title    string          `json:"title"`
	Amount   decimal.Decimal `json:"amount"`
	Currency string          `json:"currency"`
	Period   string          `json:"period"`   // monthly (по умолчанию), quarterly, yearly, once — разовый
	DueDate  models.Date     `json:"due_date"` // дата ближайшей оплаты
	// UserID — личный платёж конкретного пользователя (видит он и администратор); пусто — общий платёж.
	UserID *uint `json:"user_id"`
}

// maxAmount — предел колонки numeric(14,2): больше база не сохранит (раньше это давало ошибку 500).
var maxAmount = decimal.RequireFromString("999999999999.99")

// validAmount — сумма положительная, не больше maxAmount и не точнее копеек («1.50» и «1.500» — допустимо).
func validAmount(d decimal.Decimal) bool {
	return d.IsPositive() && d.LessThanOrEqual(maxAmount) && d.Equal(d.Round(2))
}

func validMoney(title string, amount decimal.Decimal, currency, period string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" || utf8.RuneCountInString(title) > 200 {
		return "", Invalid("title is required (max 200 characters)")
	}
	if !validAmount(amount) {
		return "", Invalid("amount must be positive, at most 999 999 999 999.99, with at most 2 decimal places")
	}
	if !models.ValidCurrency(currency) {
		return "", Invalid("currency must be one of RUB, USD, EUR")
	}
	if period != models.PeriodOnce && models.PeriodMonths(period) == 0 {
		return "", Invalid("period must be monthly, quarterly, yearly or once")
	}
	return title, nil
}

// CreateRecurring — администратор добавляет общий платёж (или личный платёж пользователю).
func (s *PaymentService) CreateRecurring(ctx context.Context, a Actor, in CreateRecurringInput) (*models.RecurringPayment, error) {
	if err := requireAdmin(a); err != nil {
		return nil, err
	}
	return s.create(ctx, a, in)
}

// CreateOwn — платёж из импорта выписки: администратор добавляет общий платёж,
// пользователь — свой личный (другим он не виден).
func (s *PaymentService) CreateOwn(ctx context.Context, a Actor, in CreateRecurringInput) (*models.RecurringPayment, error) {
	if a.Role == models.RoleAdmin {
		in.UserID = nil
	} else {
		if a.UserID == nil {
			return nil, Forbidden("login required")
		}
		in.UserID = a.UserID
	}
	return s.create(ctx, a, in)
}

func (s *PaymentService) create(ctx context.Context, a Actor, in CreateRecurringInput) (*models.RecurringPayment, error) {
	if in.Period == "" {
		in.Period = models.PeriodMonthly
	}
	title, err := validMoney(in.Title, in.Amount, in.Currency, in.Period)
	if err != nil {
		return nil, err
	}
	if in.DueDate.IsZero() {
		return nil, Invalid("due_date is required (YYYY-MM-DD)")
	}
	var owner *models.User
	if in.UserID != nil {
		if owner, err = s.st.UserByID(ctx, *in.UserID); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, Invalid("user not found")
			}
			return nil, err
		}
	}
	rp := &models.RecurringPayment{
		Title: title, Amount: in.Amount, Currency: in.Currency, Period: in.Period,
		DayOfMonth: in.DueDate.Day(), FirstDueDate: in.DueDate, UserID: in.UserID, CreatedBy: a.UserID,
	}
	details := D{"title": rp.Title, "amount": rp.Amount.StringFixed(2), "currency": rp.Currency, "due_date": rp.FirstDueDate.String(), "period": rp.Period}
	if owner != nil {
		details["owner"] = owner.Login
	}
	if err := s.add(ctx, a, rp, ActCreatePayment, models.EventPaymentCreated, details); err != nil {
		return nil, err
	}
	return rp, nil
}

// add сохраняет регулярный платёж, сразу создаёт периоды, пишет журнал и событие webhook.
func (s *PaymentService) add(ctx context.Context, a Actor, rp *models.RecurringPayment, action, event string, details D) error {
	if err := s.st.CreateRecurring(ctx, rp); err != nil {
		if errors.Is(wrapDup(err), store.ErrDuplicate) {
			return Conflict("already subscribed")
		}
		return err
	}
	if rp.Period == models.PeriodOnce { // у разового платежа продления нет (gorm подставил бы default true)
		if err := s.st.UpdateRecurringFields(ctx, rp.ID, map[string]any{"auto_renew": false}); err != nil {
			return err
		}
		rp.AutoRenew = false
	}
	if err := s.EnsureOccurrences(ctx, rp); err != nil {
		return err
	}
	s.audit.Log(ctx, a, action, "recurring_payment", rp.ID, details)
	if s.events != nil {
		var owner *string
		if o, ok := details["owner"].(string); ok {
			owner = &o
		}
		s.events.Emit(models.PaymentEvent{
			Event: event, PaymentID: rp.ID, RecurringID: rp.ID, Name: rp.Title,
			Amount: rp.Amount.StringFixed(2), Currency: rp.Currency, DueDate: rp.FirstDueDate.String(),
			Status: s.statusFor(rp.FirstDueDate), Actor: a.Login, Owner: owner, Timestamp: time.Now().UTC(),
		})
	}
	s.kick()
	return nil
}

func (s *PaymentService) ListRecurring(ctx context.Context, a Actor, owner string) ([]models.RecurringView, error) {
	sc, err := ScopeFor(a, owner)
	if err != nil {
		return nil, err
	}
	return s.st.ListRecurring(ctx, sc)
}

// DeleteRecurring прекращает платёж (история сохраняется). Общий платёж — только администратор;
// личную подписку — её владелец или администратор.
func (s *PaymentService) DeleteRecurring(ctx context.Context, a Actor, id uint) error {
	rp, err := s.st.RecurringByID(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return NotFound("recurring payment not found")
	}
	if err != nil {
		return err
	}
	if !canSee(a, rp.UserID) {
		return NotFound("recurring payment not found")
	}
	if rp.UserID == nil {
		if err := requireAdmin(a); err != nil {
			return err
		}
	}
	// общий платёж: текущий период (срок сегодня) остаётся к оплате; подписка: отключили — сегодняшний
	// неоплаченный период тоже снимается. Прошлые и оплаченные периоды остаются в истории.
	cutoff := s.Today()
	if rp.ServiceID != nil {
		cutoff = cutoff.AddDays(-1)
	}
	if err := s.st.DeleteRecurring(ctx, id, cutoff); err != nil {
		return err
	}
	action := ActDeletePayment
	if rp.ServiceID != nil {
		action = ActUnsubscribe
	}
	s.audit.Log(ctx, a, action, "recurring_payment", id, D{"title": rp.Title})
	if s.events != nil && rp.ServiceID != nil {
		s.events.Emit(models.PaymentEvent{
			Event: models.EventUnsubscribed, PaymentID: rp.ID, RecurringID: rp.ID, Name: rp.Title,
			Amount: rp.Amount.StringFixed(2), Currency: rp.Currency, Actor: a.Login, Timestamp: time.Now().UTC(),
		})
	}
	return nil
}

// ---------- периоды ----------

// EnsureOccurrences создаёт недостающие периоды вплоть до горизонта. Идемпотентно.
// Новый период никогда не затирает прошлые: у каждого своя запись и своя отметка.
func (s *PaymentService) EnsureOccurrences(ctx context.Context, rp *models.RecurringPayment) error {
	maxDue, err := s.st.MaxDueDate(ctx, rp.ID)
	if err != nil {
		return err
	}
	if maxDue != nil && (rp.Period == models.PeriodOnce || !rp.AutoRenew) {
		return nil // разовый платёж или автопродление выключено: новые периоды не создаются
	}
	limit := s.horizon()
	if maxDue == nil && limit.Before(rp.FirstDueDate) && (rp.Period == models.PeriodOnce || !rp.AutoRenew) {
		limit = rp.FirstDueDate // разовый платёж в далёком будущем — период создаём сразу, чтобы он был виден
	}
	for n := 0; ; n++ {
		due := DueDateOf(rp, n)
		if due.After(limit) {
			return nil
		}
		if maxDue != nil && !due.After(*maxDue) {
			continue
		}
		o := &models.PaymentOccurrence{
			RecurringPaymentID: rp.ID, Title: rp.Title, Amount: rp.Amount, Currency: rp.Currency, DueDate: due,
		}
		if err := s.st.InsertOccurrenceIfAbsent(ctx, o); err != nil {
			return err
		}
	}
}

// EnsureAll обновляет периоды всех активных платежей (вызывается планировщиком).
func (s *PaymentService) EnsureAll(ctx context.Context) error {
	list, err := s.st.AllRecurring(ctx)
	if err != nil {
		return err
	}
	for i := range list {
		if err := s.EnsureOccurrences(ctx, &list[i]); err != nil {
			return err
		}
	}
	return nil
}

type OccurrenceQuery struct {
	Owner       string // "", shared, mine или id пользователя (админ)
	Status      string
	Currency    string
	RecurringID uint
	From, To    *models.Date
	Desc        bool
	Limit       int
}

func (s *PaymentService) ListOccurrences(ctx context.Context, a Actor, q OccurrenceQuery) ([]models.OccurrenceView, error) {
	sc, err := ScopeFor(a, q.Owner)
	if err != nil {
		return nil, err
	}
	if q.Status != "" && q.Status != models.StatusPaid && q.Status != models.StatusUnpaid && q.Status != models.StatusOverdue && q.Status != store.StatusReview {
		return nil, Invalid("status must be paid, unpaid, overdue or review")
	}
	if q.Currency != "" && !models.ValidCurrency(q.Currency) {
		return nil, Invalid("currency must be one of RUB, USD, EUR")
	}
	today := s.Today()
	rows, err := s.st.ListOccurrences(ctx, store.OccurrenceFilter{
		Scope: sc, Currency: q.Currency, RecurringID: q.RecurringID, From: q.From, To: q.To,
		Status: q.Status, Today: today, Desc: q.Desc, Limit: q.Limit,
	})
	if err != nil {
		return nil, err
	}
	for i := range rows {
		s.decorate(&rows[i], today)
	}
	return rows, nil
}

// GetOccurrence — период, если он виден actor (чужие личные подписки — «не найдено»).
func (s *PaymentService) GetOccurrence(ctx context.Context, a Actor, id uint) (*models.OccurrenceView, error) {
	v, err := s.occurrence(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canSee(a, v.UserID) {
		return nil, NotFound("payment occurrence not found")
	}
	return v, nil
}

func (s *PaymentService) occurrence(ctx context.Context, id uint) (*models.OccurrenceView, error) {
	v, err := s.st.OccurrenceByID(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, NotFound("payment occurrence not found")
	}
	if err != nil {
		return nil, err
	}
	s.decorate(v, s.Today())
	return v, nil
}

// statusFor — статус неоплаченного периода с таким сроком на сегодня.
func (s *PaymentService) statusFor(due models.Date) string {
	if due.Before(s.Today()) {
		return models.StatusOverdue
	}
	return models.StatusUnpaid
}

// decorate вычисляет статус и просрочку.
//   - оплачено → paid; просрочка = дни между сроком и фактической оплатой (в день срока — 0);
//   - не оплачено и срок прошёл (со следующего дня) → overdue;
//   - иначе → unpaid.
func (s *PaymentService) decorate(v *models.OccurrenceView, today models.Date) {
	switch {
	case v.PaidAt != nil:
		v.Status = models.StatusPaid
		v.OverdueDays = max(0, v.DueDate.DaysUntil(*v.PaidAt))
	case v.DueDate.Before(today):
		v.Status = models.StatusOverdue
		v.OverdueDays = v.DueDate.DaysUntil(today)
	default:
		v.Status = models.StatusUnpaid
		v.Future = v.DueDate.After(today) // будущий период — свой статус «предстоит»
	}
}

// UpdateOccurrenceInput — тело PATCH. paid=true/false ставит или снимает отметку,
// paid_at задаёт/исправляет фактическую дату оплаты, receipt_url — ссылка на чек ("" — убрать).
type UpdateOccurrenceInput struct {
	Paid       *bool   `json:"paid"`
	PaidAt     *string `json:"paid_at"`
	ReceiptURL *string `json:"receipt_url"`
}

// UpdateOccurrence доступен и пользователю, и администратору: общие платежи отмечает любой участник,
// личную подписку — её владелец (и администратор).
func (s *PaymentService) UpdateOccurrence(ctx context.Context, a Actor, id uint, in UpdateOccurrenceInput) (*models.OccurrenceView, error) {
	cur, err := s.GetOccurrence(ctx, a, id)
	if err != nil {
		return nil, err
	}
	today := s.Today()

	var paidAt *models.Date
	if in.PaidAt != nil {
		d, err := models.ParseDate(*in.PaidAt)
		if err != nil {
			return nil, Invalid(err.Error())
		}
		if d.After(today) {
			return nil, Invalid("paid_at cannot be in the future")
		}
		paidAt = &d
	}
	var receipt *string
	if in.ReceiptURL != nil {
		receipt, err = normReceiptURL(*in.ReceiptURL)
		if err != nil {
			return nil, err
		}
	}
	if in.Paid == nil && paidAt == nil && in.ReceiptURL == nil {
		return nil, Invalid("provide paid, paid_at and/or receipt_url")
	}
	if in.Paid != nil && !*in.Paid && paidAt != nil {
		return nil, Invalid("paid_at cannot be set together with paid=false")
	}
	// в режиме «по чеку» чек нужен, чтобы отметить неоплаченный период оплаченным;
	// снять отметку и исправить дату уже оплаченного периода можно без нового чека
	if cur.PaidAt == nil && ((in.Paid != nil && *in.Paid) || paidAt != nil) && s.RequiresReceipt(ctx, a) {
		return nil, ErrReceiptRequired
	}

	if in.ReceiptURL != nil {
		if err := s.st.SetReceipt(ctx, id, receipt); err != nil {
			return nil, err
		}
		url := ""
		if receipt != nil {
			url = *receipt
		}
		s.audit.Log(ctx, a, ActReceipt, "payment_occurrence", id, D{"receipt_url": url})
	}

	event := ""
	switch {
	case in.Paid != nil && !*in.Paid: // отмена оплаты → дата очищается
		if err := s.st.SetPaid(ctx, id, nil, nil); err != nil {
			return nil, err
		}
		s.audit.Log(ctx, a, ActUnmarkPaid, "payment_occurrence", id, D{"due_date": cur.DueDate.String()})
		event = models.EventPaymentUnpaid
	case in.Paid != nil || paidAt != nil: // отметить оплату или исправить дату
		d := today
		switch {
		case paidAt != nil:
			d = *paidAt
		case cur.PaidAt != nil:
			d = *cur.PaidAt
		}
		by := a.Login
		if err := s.st.SetPaid(ctx, id, &d, &by); err != nil {
			return nil, err
		}
		action, details := ActMarkPaid, D{"due_date": cur.DueDate.String(), "paid_at": d.String()}
		event = models.EventPaymentPaid
		if cur.PaidAt == nil && s.receiptsPaidByAdmin != nil {
			s.receiptsPaidByAdmin(ctx, id, a.Login)
		}
		if cur.PaidAt != nil {
			action, event = ActEditDate, models.EventPaymentDateFix
			details["previous_paid_at"] = cur.PaidAt.String()
		}
		s.audit.Log(ctx, a, action, "payment_occurrence", id, details)
	}

	v, err := s.occurrence(ctx, id) // срок (due_date) не меняется; просрочка пересчитана
	if err == nil && event != "" {
		s.emit(event, v, a.Login)
	}
	return v, err
}

// normReceiptURL: пустая строка — убрать ссылку; иначе только http(s), до 1000 символов.
func normReceiptURL(v string) (*string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	if err := validHTTPURL(v, 1000); err != nil {
		return nil, Invalid("receipt_url must be a valid http(s) link")
	}
	return &v, nil
}

// ---------- итоги ----------

func (s *PaymentService) Summary(ctx context.Context, a Actor, owner string, from, to *models.Date) ([]models.CurrencySummary, error) {
	sc, err := ScopeFor(a, owner)
	if err != nil {
		return nil, err
	}
	rows, err := s.st.Summary(ctx, sc, s.Today(), from, to)
	if err != nil {
		return nil, err
	}
	// Всегда отдаём все три валюты — суммы по каждой отдельно, без конвертации.
	byCur := map[string]models.CurrencySummary{}
	for _, r := range rows {
		byCur[r.Currency] = r
	}
	out := make([]models.CurrencySummary, 0, len(models.Currencies))
	for _, c := range models.Currencies {
		r, ok := byCur[c]
		if !ok {
			r = models.CurrencySummary{Currency: c}
		}
		r.Currency = c
		out = append(out, r)
	}
	return out, nil
}

// MonthlySummary — итоги по месяцам (по сроку платежа) и валютам: оплачено, просрочено, предстоит.
// months — сколько месяцев назад от текущего (включая текущий), ahead — сколько вперёд.
func (s *PaymentService) MonthlySummary(ctx context.Context, a Actor, owner string, months, ahead int) ([]models.MonthSummary, error) {
	sc, err := ScopeFor(a, owner)
	if err != nil {
		return nil, err
	}
	months = min(max(months, 1), 36)
	ahead = min(max(ahead, 0), 12)
	today := s.Today()
	first := models.NewDate(today.Year(), today.Month(), 1)
	from := models.DateOf(first.Time.AddDate(0, -(months - 1), 0))
	to := models.DateOf(first.Time.AddDate(0, ahead+1, -1))
	return s.st.MonthlySummary(ctx, sc, today, from, to)
}

// ---------- изменение регулярного платежа ----------

// UpdateRecurringInput — PATCH регулярного платежа. Все поля необязательны.
// Пользователь меняет только автопродление своей подписки; администратор — всё.
type UpdateRecurringInput struct {
	Title     *string          `json:"title"`
	Amount    *decimal.Decimal `json:"amount"`
	Currency  *string          `json:"currency"`
	Period    *string          `json:"period"`
	DueDate   *models.Date     `json:"due_date"` // новая дата ближайшей оплаты (график строится от неё)
	AutoRenew *bool            `json:"auto_renew"`
}

func (s *PaymentService) UpdateRecurring(ctx context.Context, a Actor, id uint, in UpdateRecurringInput) (*models.RecurringView, error) {
	rp, err := s.st.RecurringByID(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, NotFound("recurring payment not found")
	}
	if err != nil {
		return nil, err
	}
	if !canSee(a, rp.UserID) {
		return nil, NotFound("recurring payment not found")
	}
	edits := in.Title != nil || in.Amount != nil || in.Currency != nil || in.Period != nil || in.DueDate != nil
	if edits || rp.UserID == nil {
		if err := requireAdmin(a); err != nil {
			return nil, err // общий платёж и параметры (сумма, график) меняет только администратор
		}
	}
	if !edits && in.AutoRenew == nil {
		return nil, Invalid("nothing to update")
	}
	today := s.Today()
	next := *rp
	if in.Title != nil {
		next.Title = *in.Title
	}
	if in.Amount != nil {
		next.Amount = *in.Amount
	}
	if in.Currency != nil {
		next.Currency = *in.Currency
	}
	if in.Period != nil {
		next.Period = *in.Period
	}
	if next.Title, err = validMoney(next.Title, next.Amount, next.Currency, next.Period); err != nil {
		return nil, err
	}
	reschedule := (in.Period != nil && *in.Period != rp.Period) || in.DueDate != nil
	if in.DueDate != nil {
		if in.DueDate.IsZero() {
			return nil, Invalid("due_date is invalid (YYYY-MM-DD)")
		}
		next.FirstDueDate, next.DayOfMonth = *in.DueDate, in.DueDate.Day()
	} else if reschedule {
		// сменилась только периодичность — график начинается с ближайшего неоплаченного срока
		if d, err := s.st.NextUnpaidDue(ctx, rp.ID, today); err == nil && d != nil {
			next.FirstDueDate, next.DayOfMonth = *d, d.Day()
		} else {
			next.FirstDueDate, next.DayOfMonth = today, today.Day()
		}
	}
	if in.AutoRenew != nil {
		next.AutoRenew = *in.AutoRenew
	}
	if next.Period == models.PeriodOnce {
		next.AutoRenew = false
	}

	fields := map[string]any{}
	details := D{"title": next.Title}
	if next.Title != rp.Title {
		fields["title"], details["previous_title"] = next.Title, rp.Title
	}
	if !next.Amount.Equal(rp.Amount) {
		fields["amount"], details["amount"], details["previous_amount"] = next.Amount, next.Amount.StringFixed(2), rp.Amount.StringFixed(2)
	}
	if next.Currency != rp.Currency {
		fields["currency"], details["currency"], details["previous_currency"] = next.Currency, next.Currency, rp.Currency
	}
	if reschedule {
		fields["period"], fields["first_due_date"], fields["day_of_month"] = next.Period, next.FirstDueDate, next.DayOfMonth
		details["period"], details["due_date"] = next.Period, next.FirstDueDate.String()
	}
	if next.AutoRenew != rp.AutoRenew {
		fields["auto_renew"], details["auto_renew"] = next.AutoRenew, next.AutoRenew
	}
	if len(fields) == 0 {
		return s.st.RecurringView(ctx, id)
	}
	if err := s.st.UpdateRecurringFields(ctx, id, fields); err != nil {
		return nil, err
	}
	// неоплаченные периоды, срок которых не прошёл, получают новые название/сумму/валюту;
	// прошлые и оплаченные периоды — история, не меняются
	occ := map[string]any{}
	for _, k := range []string{"title", "amount", "currency"} {
		if v, ok := fields[k]; ok {
			occ[k] = v
		}
	}
	if len(occ) > 0 {
		if err := s.st.UpdateUnpaidOccurrences(ctx, id, today, occ); err != nil {
			return nil, err
		}
	}
	switch {
	case reschedule: // новый график: будущие неоплаченные периоды строятся заново
		if err := s.st.DeleteUnpaidFrom(ctx, id, today); err != nil {
			return nil, err
		}
	case rp.AutoRenew && !next.AutoRenew: // продление выключено: остаётся только ближайший неоплаченный период
		if d, err := s.st.NextUnpaidDue(ctx, id, today); err == nil && d != nil {
			if err := s.st.DeleteUnpaidFrom(ctx, id, d.AddDays(1)); err != nil {
				return nil, err
			}
		}
	}
	fresh, err := s.st.RecurringByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.EnsureOccurrences(ctx, fresh); err != nil {
		return nil, err
	}
	action := ActEditPayment
	if len(fields) == 2 && fields["auto_renew"] != nil { // только автопродление (+updated_at)
		action = ActAutoRenew
	}
	s.audit.Log(ctx, a, action, "recurring_payment", id, details)
	s.kick()
	return s.st.RecurringView(ctx, id)
}

// ---------- напоминания (используются Telegram-ботом) ----------

// ProcessReminders — напоминания в общий Telegram-чат: общие платежи, те же ступени, что у остальных каналов.
func (s *PaymentService) ProcessReminders(ctx context.Context, send func(models.OccurrenceView) error) error {
	return s.ProcessChannelReminders(ctx, "chat", store.Scope{All: true, Owner: "shared"}, send)
}

// ReminderStage — ступень напоминания для периода на сегодня (d7, d3, d1, od или "").
func (s *PaymentService) ReminderStage(v models.OccurrenceView) string {
	return models.ReminderStage(s.Today().DaysUntil(v.DueDate), s.stages)
}

// MarkPaidFromBot — кнопка «Оплачено» в Telegram. Датой оплаты считается сегодня.
// alreadyPaid=true — период был оплачен раньше (ничего не меняем).
func (s *PaymentService) MarkPaidFromBot(ctx context.Context, a Actor, id uint) (v *models.OccurrenceView, alreadyPaid bool, err error) {
	cur, err := s.GetOccurrence(ctx, a, id)
	if err != nil {
		return nil, false, err
	}
	if cur.PaidAt != nil {
		return cur, true, nil
	}
	d := s.Today()
	by := a.Login
	if err := s.st.SetPaid(ctx, id, &d, &by); err != nil {
		return nil, false, err
	}
	s.audit.Log(ctx, a, ActMarkPaid, "payment_occurrence", id, D{"via": "telegram", "paid_at": d.String()})
	v, err = s.occurrence(ctx, id)
	if err == nil {
		s.emit(models.EventPaymentPaid, v, a.Login)
	}
	return v, false, err
}

// ProcessChannelReminders — напоминания в канал (chat, telegram:<id>, email:<id>, app:<id>, webhook).
// Ступени: за 7, 3 и 1 день до срока и один раз после просрочки — в каждый канал по одному сообщению на ступень.
// Добавленный поздно платёж получает только текущую ступень (без пачки пропущенных).
// Оплаченные, удалённые и с чеком на проверке не напоминают. Доставка фиксируется в notification_deliveries
// (ключ «канал#ступень», атомарно); при ошибке отправки отметка снимается и попытка повторится.
// scope — чьи платежи идут в канал: письмо и личный Telegram получают только свои платежи.
func (s *PaymentService) ProcessChannelReminders(ctx context.Context, channel string, sc store.Scope, send func(models.OccurrenceView) error) error {
	today := s.Today()
	list, err := s.st.DueRemindersFor(ctx, channel, sc, today, s.stages, 50)
	if err != nil {
		return err
	}
	for _, v := range list {
		stage := models.ReminderStage(today.DaysUntil(v.DueDate), s.stages)
		if stage == "" {
			continue
		}
		key := channel + "#" + stage
		ok, err := s.st.ClaimDelivery(ctx, v.ID, key)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		s.decorate(&v, today)
		if err := send(v); err != nil {
			log.Printf("reminder #%d via %s: send failed, will retry: %v", v.ID, key, err)
			_ = s.st.ReleaseDelivery(ctx, v.ID, key)
		}
	}
	return nil
}

// Upcoming — ближайшие неоплаченные периоды, видимые actor (команда бота /upcoming, тест напоминания).
func (s *PaymentService) Upcoming(ctx context.Context, a Actor, owner string, limit int) ([]models.OccurrenceView, error) {
	rows, err := s.ListOccurrences(ctx, a, OccurrenceQuery{Owner: owner, Limit: 1000})
	if err != nil {
		return nil, err
	}
	var out []models.OccurrenceView
	for _, r := range rows {
		if r.Status != models.StatusPaid && !r.RecurringDeleted {
			out = append(out, r)
			if len(out) == limit {
				break
			}
		}
	}
	return out, nil
}

// ConfirmPaid — оплата подтверждена по чеку (автопроверкой или администратором).
// by — кто платил (тот, кто приложил чек); actor — кто подтвердил.
func (s *PaymentService) ConfirmPaid(ctx context.Context, a Actor, id uint, d models.Date, by string, details D) error {
	cur, err := s.occurrence(ctx, id)
	if err != nil {
		return err
	}
	if cur.PaidAt != nil {
		return nil // уже оплачено (например, администратор отметил вручную)
	}
	if err := s.st.SetPaid(ctx, id, &d, &by); err != nil {
		return err
	}
	details["due_date"], details["paid_at"], details["paid_by"] = cur.DueDate.String(), d.String(), by
	s.audit.Log(ctx, a, ActMarkPaid, "payment_occurrence", id, details)
	if v, err := s.occurrence(ctx, id); err == nil {
		s.emit(models.EventPaymentPaid, v, a.Login)
	}
	return nil
}
