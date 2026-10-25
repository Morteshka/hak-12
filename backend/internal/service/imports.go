package service

import (
	"bytes"
	"context"
	"errors"
	"strings"

	"github.com/shopspring/decimal"

	"invoicekeeper/internal/models"
	"invoicekeeper/internal/statement"
)

// MaxStatementSize — предел файла выписки (CSV или PDF).
const MaxStatementSize = 5 << 20

// ImportService — импорт банковской выписки: находит регулярные списания и добавляет выбранные как платежи.
// Шаг 1 (Analyze) ничего не сохраняет, шаг 2 (Confirm) создаёт платежи. Пользователь получает личные платежи,
// администратор — общие платежи.
type ImportService struct {
	pay     *PaymentService
	audit   *Audit
	pdfText func(ctx context.Context, data []byte) (string, error)
}

func NewImportService(pay *PaymentService, audit *Audit, pdfText func(context.Context, []byte) (string, error)) *ImportService {
	return &ImportService{pay: pay, audit: audit, pdfText: pdfText}
}

// ImportCandidate — найденное регулярное списание; Exists — похожий платёж уже есть в системе.
type ImportCandidate struct {
	statement.Candidate
	Exists bool `json:"exists"`
}

type ImportResult struct {
	File         string                     `json:"file"`
	Transactions int                        `json:"transactions"` // сколько списаний прочитано
	From         string                     `json:"from"`
	To           string                     `json:"to"`
	Candidates   []ImportCandidate          `json:"candidates"`
	Monthly      map[string]decimal.Decimal `json:"monthly"` // в месяц по валютам (новые, без уже добавленных)
}

// Analyze читает выписку и возвращает найденные регулярные платежи.
func (s *ImportService) Analyze(ctx context.Context, a Actor, name string, data []byte) (*ImportResult, error) {
	if len(data) == 0 {
		return nil, Invalid("file is required")
	}
	if len(data) > MaxStatementSize {
		return nil, Invalid("statement is too large: max 5 MB")
	}
	var txns []statement.Txn
	var err error
	switch {
	case bytes.HasPrefix(data, []byte("%PDF")):
		if s.pdfText == nil {
			return nil, Invalid("pdf statements are not supported on this server, upload CSV")
		}
		text, perr := s.pdfText(ctx, data)
		if perr != nil {
			return nil, Invalid("pdf statements are not supported on this server, upload CSV")
		}
		txns, err = statement.ParseText(text)
	case looksBinary(data):
		return nil, Invalid("unsupported statement format: upload CSV or PDF from your bank")
	default:
		txns, err = statement.ParseCSV(data)
		if errors.Is(err, statement.ErrNoTransactions) {
			txns, err = statement.ParseText(string(data))
		}
	}
	if err != nil || len(txns) == 0 {
		return nil, Invalid("no transactions found in the statement")
	}

	res := &ImportResult{File: name, Transactions: len(txns), Monthly: map[string]decimal.Decimal{}, Candidates: []ImportCandidate{}}
	from, to := txns[0].Date, txns[0].Date
	for _, t := range txns {
		if t.Date.Before(from) {
			from = t.Date
		}
		if t.Date.After(to) {
			to = t.Date
		}
	}
	res.From, res.To = from.Format(models.DateLayout), to.Format(models.DateLayout)

	existing, err := s.pay.ListRecurring(ctx, a, "")
	if err != nil {
		return nil, err
	}
	for _, c := range statement.Detect(txns, s.pay.Today().Time) {
		ic := ImportCandidate{Candidate: c, Exists: alreadyTracked(c, existing, a)}
		if !ic.Exists {
			res.Monthly[c.Currency] = res.Monthly[c.Currency].Add(c.Monthly)
		}
		res.Candidates = append(res.Candidates, ic)
	}
	return res, nil
}

// alreadyTracked — у пользователя уже есть платёж с той же валютой и суммой (±1%) или похожим названием.
func alreadyTracked(c statement.Candidate, list []models.RecurringView, a Actor) bool {
	key := strings.ToLower(c.Title)
	for _, r := range list {
		mine := (a.Role == models.RoleAdmin && r.UserID == nil) || (r.UserID != nil && a.UserID != nil && *r.UserID == *a.UserID)
		if !mine || r.Currency != c.Currency {
			continue
		}
		diff := r.Amount.Sub(c.Amount).Abs()
		if diff.LessThanOrEqual(c.Amount.Mul(decimal.NewFromFloat(0.01))) {
			return true
		}
		t := strings.ToLower(r.Title)
		if len([]rune(t)) >= 4 && (strings.Contains(key, t) || strings.Contains(t, key)) {
			return true
		}
	}
	return false
}

func looksBinary(data []byte) bool {
	n := min(len(data), 2048)
	return bytes.IndexByte(data[:n], 0) >= 0
}

// ImportItem — выбранная строка (название и сумму можно поправить перед добавлением).
type ImportItem struct {
	Title    string          `json:"title"`
	Amount   decimal.Decimal `json:"amount"`
	Currency string          `json:"currency"`
	Period   string          `json:"period"`
	DueDate  models.Date     `json:"due_date"`
}

// Confirm добавляет выбранные платежи. Ошибка в одной строке не мешает остальным — возвращается списком.
func (s *ImportService) Confirm(ctx context.Context, a Actor, file string, items []ImportItem) (created []models.RecurringPayment, failed []string, err error) {
	if len(items) == 0 {
		return nil, nil, Invalid("select at least one payment")
	}
	if len(items) > 50 {
		return nil, nil, Invalid("too many payments: max 50 at once")
	}
	for _, it := range items {
		rp, e := s.pay.CreateOwn(ctx, a, CreateRecurringInput{Title: it.Title, Amount: it.Amount, Currency: it.Currency, Period: it.Period, DueDate: it.DueDate})
		if e != nil {
			failed = append(failed, strings.TrimSpace(it.Title)+": "+e.Error())
			continue
		}
		created = append(created, *rp)
	}
	if len(created) > 0 {
		s.audit.Log(ctx, a, ActImport, "recurring_payment", nil, D{"count": len(created), "file": file})
	}
	if created == nil {
		created = []models.RecurringPayment{}
	}
	return created, failed, nil
}
