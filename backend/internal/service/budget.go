package service

import (
	"context"
	"time"

	"github.com/shopspring/decimal"

	"invoicekeeper/internal/models"
	"invoicekeeper/internal/store"
)

// BudgetService — плановые расходы на месяц/год по каждой валюте и сравнение с фактом.
// Бюджет личный: каждый задаёт свой план; факт и прогноз считаются по видимым ему платежам
// (общие платежи + свои подписки; у администратора — все платежи).
type BudgetService struct {
	st    *store.Store
	audit *Audit
	pay   *PaymentService
}

func NewBudgetService(st *store.Store, audit *Audit, pay *PaymentService) *BudgetService {
	return &BudgetService{st: st, audit: audit, pay: pay}
}

func validPeriod(p string) bool { return p == models.BudgetMonth || p == models.BudgetYear }

func uidOf(a Actor) (uint, error) {
	if a.UserID == nil {
		return 0, Unauthorized("authorization required")
	}
	return *a.UserID, nil
}

func (s *BudgetService) List(ctx context.Context, a Actor, period string) ([]models.Budget, error) {
	if period != "" && !validPeriod(period) {
		return nil, Invalid("period must be month or year")
	}
	uid, err := uidOf(a)
	if err != nil {
		return nil, err
	}
	return s.st.ListBudgets(ctx, uid, period)
}

type BudgetInput struct {
	Currency string          `json:"currency"`
	Period   string          `json:"period"`
	Amount   decimal.Decimal `json:"amount"`
}

// Upsert создаёт или обновляет бюджет для пары (валюта, период).
func (s *BudgetService) Upsert(ctx context.Context, a Actor, in BudgetInput) (*models.Budget, error) {
	uid, err := uidOf(a)
	if err != nil {
		return nil, err
	}
	if !models.ValidCurrency(in.Currency) {
		return nil, Invalid("currency must be one of RUB, USD, EUR")
	}
	if !validPeriod(in.Period) {
		return nil, Invalid("period must be month or year")
	}
	if !validAmount(in.Amount) {
		return nil, Invalid("amount must be positive, at most 999 999 999 999.99, with at most 2 decimal places")
	}
	b := &models.Budget{UserID: &uid, Currency: in.Currency, Period: in.Period, Amount: in.Amount}
	if err := s.st.UpsertBudget(ctx, b); err != nil {
		return nil, err
	}
	s.audit.Log(ctx, a, ActBudget, "budget", b.ID, D{"currency": b.Currency, "period": b.Period, "amount": b.Amount.StringFixed(2)})
	return b, nil
}

func (s *BudgetService) Delete(ctx context.Context, a Actor, id uint) error {
	uid, err := uidOf(a)
	if err != nil {
		return err
	}
	ok, err := s.st.DeleteBudget(ctx, uid, id)
	if err != nil {
		return err
	}
	if !ok {
		return NotFound("budget not found")
	}
	s.audit.Log(ctx, a, ActBudget, "budget", id, D{"deleted": true})
	return nil
}

// BudgetLine — план/факт/прогноз по одной валюте.
//   - plan — бюджет (null, если не задан);
//   - fact — сумма оплаченных периодов со сроком в этом периоде;
//   - forecast — все платежи периода: уже созданные + будущие по активным регулярным платежам;
//   - spent_percent — сколько % бюджета израсходовано (fact / plan);
//   - over_budget — прогноз больше плана; overspent — факт уже больше плана.
type BudgetLine struct {
	Currency        string           `json:"currency"`
	BudgetID        *uint            `json:"budget_id"`
	Plan            *decimal.Decimal `json:"plan"`
	Fact            decimal.Decimal  `json:"fact"`
	Forecast        decimal.Decimal  `json:"forecast"`
	Remaining       *decimal.Decimal `json:"remaining"`
	SpentPercent    *float64         `json:"spent_percent"`
	ForecastPercent *float64         `json:"forecast_percent"`
	OverBudget      bool             `json:"over_budget"`
	Overspent       bool             `json:"overspent"`
	CountTotal      int              `json:"count_total"`
	CountPaid       int              `json:"count_paid"`
}

type BudgetStatus struct {
	Period     string       `json:"period"`
	From       models.Date  `json:"from"`
	To         models.Date  `json:"to"`
	Currencies []BudgetLine `json:"currencies"`
}

// PeriodBounds — границы месяца или года, в который попадает дата at.
func PeriodBounds(period string, at models.Date) (models.Date, models.Date) {
	if period == models.BudgetYear {
		return models.NewDate(at.Year(), time.January, 1), models.NewDate(at.Year(), time.December, 31)
	}
	from := models.NewDate(at.Year(), at.Month(), 1)
	return from, models.DateOf(from.Time.AddDate(0, 1, -1))
}

func (s *BudgetService) Status(ctx context.Context, a Actor, period string, at *models.Date) (*BudgetStatus, error) {
	uid, err := uidOf(a)
	if err != nil {
		return nil, err
	}
	sc, err := ScopeFor(a, "")
	if err != nil {
		return nil, err
	}
	if period == "" {
		period = models.BudgetMonth
	}
	if !validPeriod(period) {
		return nil, Invalid("period must be month or year")
	}
	day := s.pay.Today()
	if at != nil {
		day = *at
	}
	from, to := PeriodBounds(period, day)

	lines := map[string]*BudgetLine{}
	for _, c := range models.Currencies {
		lines[c] = &BudgetLine{Currency: c}
	}

	// 1) уже созданные периоды (включая прекращённые платежи — это реальные расходы)
	occ, err := s.st.OccurrencesBetween(ctx, sc, from, to)
	if err != nil {
		return nil, err
	}
	for _, o := range occ {
		l := lines[o.Currency]
		if l == nil {
			continue
		}
		l.Forecast = l.Forecast.Add(o.Amount)
		l.CountTotal++
		if o.PaidAt != nil {
			l.Fact = l.Fact.Add(o.Amount)
			l.CountPaid++
		}
	}

	// 2) будущие периоды, которые ещё не созданы (периоды создаются на месяц вперёд)
	rps, err := s.st.ListRecurring(ctx, sc)
	if err != nil {
		return nil, err
	}
	for i := range rps {
		rp := &rps[i].RecurringPayment
		l := lines[rp.Currency]
		if l == nil {
			continue
		}
		maxDue, err := s.st.MaxDueDate(ctx, rp.ID)
		if err != nil {
			return nil, err
		}
		for n := 0; ; n++ {
			due := DueDateOf(rp, n)
			if due.After(to) {
				break
			}
			if due.Before(from) || (maxDue != nil && !due.After(*maxDue)) {
				continue
			}
			l.Forecast = l.Forecast.Add(rp.Amount)
			l.CountTotal++
		}
	}

	// 3) план
	budgets, err := s.st.ListBudgets(ctx, uid, period)
	if err != nil {
		return nil, err
	}
	hundred := decimal.NewFromInt(100)
	for _, b := range budgets {
		l := lines[b.Currency]
		if l == nil {
			continue
		}
		id, plan := b.ID, b.Amount
		l.BudgetID, l.Plan = &id, &plan
		rem := plan.Sub(l.Forecast)
		l.Remaining = &rem
		sp, _ := l.Fact.Div(plan).Mul(hundred).Round(1).Float64()
		fp, _ := l.Forecast.Div(plan).Mul(hundred).Round(1).Float64()
		l.SpentPercent, l.ForecastPercent = &sp, &fp
		l.OverBudget = l.Forecast.GreaterThan(plan)
		l.Overspent = l.Fact.GreaterThan(plan)
	}

	out := &BudgetStatus{Period: period, From: from, To: to}
	for _, c := range models.Currencies {
		out.Currencies = append(out.Currencies, *lines[c])
	}
	return out, nil
}
