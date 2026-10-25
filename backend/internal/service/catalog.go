package service

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/shopspring/decimal"

	"invoicekeeper/internal/models"
	"invoicekeeper/internal/store"
)

// CatalogService — каталог услуг. Администратор публикует услуги, пользователь сам подключает нужные.
// Подключение создаёт личный регулярный платёж (подписку) с периодами, напоминаниями и отметками оплаты.
type CatalogService struct {
	st    *store.Store
	audit *Audit
	pay   *PaymentService
}

func NewCatalogService(st *store.Store, audit *Audit, pay *PaymentService) *CatalogService {
	return &CatalogService{st: st, audit: audit, pay: pay}
}

// List — каталог. Пользователь видит только открытые услуги, администратор — также скрытые.
func (s *CatalogService) List(ctx context.Context, a Actor) ([]models.ServiceView, error) {
	var uid uint
	if a.UserID != nil {
		uid = *a.UserID
	}
	out, err := s.st.ListServices(ctx, uid, a.Role == models.RoleAdmin)
	if out == nil {
		out = []models.ServiceView{}
	}
	return out, err
}

// ServiceInput — создание (все поля) и изменение (только переданные поля) услуги.
type ServiceInput struct {
	Title       *string          `json:"title"`
	Description *string          `json:"description"`
	Amount      *decimal.Decimal `json:"amount"`
	Currency    *string          `json:"currency"`
	Period      *string          `json:"period"`     // monthly, quarterly, yearly
	TrialDays   *int             `json:"trial_days"` // 0 — первый платёж сразу; N — пробный период N дней
	Active      *bool            `json:"active"`     // false — скрыть из каталога (подписки продолжают работать)
}

func (in ServiceInput) apply(sv *models.Service) error {
	if in.Title != nil {
		sv.Title = *in.Title
	}
	if in.Description != nil {
		sv.Description = strings.TrimSpace(*in.Description)
	}
	if in.Amount != nil {
		sv.Amount = *in.Amount
	}
	if in.Currency != nil {
		sv.Currency = *in.Currency
	}
	if in.Period != nil {
		sv.Period = *in.Period
	}
	if in.TrialDays != nil {
		sv.TrialDays = *in.TrialDays
	}
	if in.Active != nil {
		sv.Active = *in.Active
	}
	if sv.Period == "" {
		sv.Period = models.PeriodMonthly
	}
	if sv.Period == models.PeriodOnce { // услуга из каталога — всегда подписка с продлением
		return Invalid("period must be monthly, quarterly or yearly")
	}
	title, err := validMoney(sv.Title, sv.Amount, sv.Currency, sv.Period)
	if err != nil {
		return err
	}
	sv.Title = title
	if utf8.RuneCountInString(sv.Description) > 1000 {
		return Invalid("description is too long (max 1000 characters)")
	}
	if sv.TrialDays < 0 || sv.TrialDays > 365 {
		return Invalid("trial_days must be between 0 and 365")
	}
	return nil
}

func serviceDetails(sv *models.Service) D {
	return D{"title": sv.Title, "amount": sv.Amount.StringFixed(2), "currency": sv.Currency,
		"period": sv.Period, "trial_days": sv.TrialDays, "active": sv.Active}
}

func (s *CatalogService) Create(ctx context.Context, a Actor, in ServiceInput) (*models.Service, error) {
	if err := requireAdmin(a); err != nil {
		return nil, err
	}
	sv := &models.Service{Active: true, CreatedBy: a.UserID}
	if in.Amount == nil {
		return nil, Invalid("amount must be positive with at most 2 decimal places")
	}
	if err := in.apply(sv); err != nil {
		return nil, err
	}
	if err := s.st.CreateService(ctx, sv); err != nil {
		return nil, err
	}
	s.audit.Log(ctx, a, ActCreateService, "service", sv.ID, serviceDetails(sv))
	return sv, nil
}

func (s *CatalogService) get(ctx context.Context, id uint) (*models.Service, error) {
	sv, err := s.st.ServiceByID(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, NotFound("service not found")
	}
	return sv, err
}

// Update меняет услугу в каталоге. Уже подключённые подписки сохраняют свои условия (цену и срок),
// новые условия действуют для новых подключений.
func (s *CatalogService) Update(ctx context.Context, a Actor, id uint, in ServiceInput) (*models.Service, error) {
	if err := requireAdmin(a); err != nil {
		return nil, err
	}
	sv, err := s.get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := in.apply(sv); err != nil {
		return nil, err
	}
	if err := s.st.SaveService(ctx, sv); err != nil {
		return nil, err
	}
	s.audit.Log(ctx, a, ActUpdateService, "service", sv.ID, serviceDetails(sv))
	return sv, nil
}

// Delete убирает услугу из каталога. Действующие подписки продолжают работать — их отключает
// сам пользователь (или администратор на странице подписок).
func (s *CatalogService) Delete(ctx context.Context, a Actor, id uint) error {
	if err := requireAdmin(a); err != nil {
		return err
	}
	sv, err := s.get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.st.DeleteService(ctx, id); err != nil {
		return err
	}
	s.audit.Log(ctx, a, ActDeleteService, "service", id, D{"title": sv.Title})
	return nil
}

// FirstDue — когда первый платёж при подключении сегодня: сразу или после пробного периода.
func FirstDue(today models.Date, trialDays int) (due models.Date, trialUntil *models.Date) {
	if trialDays <= 0 {
		return today, nil
	}
	d := today.AddDays(trialDays)
	return d, &d
}

// Subscribe — пользователь подключает услугу. Создаётся личный регулярный платёж:
// условия (название, цена, валюта, периодичность) копируются из каталога на момент подключения.
//   - без пробного периода — первый платёж сегодня (напоминание приходит сразу);
//   - с пробным периодом N дней — первый платёж через N дней, дальше по периодичности.
func (s *CatalogService) Subscribe(ctx context.Context, a Actor, id uint) (*models.RecurringPayment, error) {
	if a.UserID == nil {
		return nil, Unauthorized("authorization required")
	}
	sv, err := s.get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !sv.Active {
		return nil, Invalid("service is not available")
	}
	if _, err := s.st.ActiveSubscription(ctx, *a.UserID, sv.ID); err == nil {
		return nil, Conflict("already subscribed")
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	due, trial := FirstDue(s.pay.Today(), sv.TrialDays)
	uid, sid := *a.UserID, sv.ID
	rp := &models.RecurringPayment{
		Title: sv.Title, Amount: sv.Amount, Currency: sv.Currency, Period: sv.Period,
		DayOfMonth: due.Day(), FirstDueDate: due, UserID: &uid, ServiceID: &sid, TrialUntil: trial, CreatedBy: a.UserID,
	}
	details := D{"service": sv.Title, "service_id": sv.ID, "amount": sv.Amount.StringFixed(2), "currency": sv.Currency,
		"period": sv.Period, "first_due_date": due.String(), "owner": a.Login}
	if trial != nil {
		details["trial_until"] = trial.String()
	}
	if err := s.pay.add(ctx, a, rp, ActSubscribe, models.EventSubscribed, details); err != nil {
		return nil, err
	}
	return rp, nil
}
