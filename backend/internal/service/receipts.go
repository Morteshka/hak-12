package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shopspring/decimal"

	"invoicekeeper/internal/models"
	"invoicekeeper/internal/receipt"
	"invoicekeeper/internal/store"
)

// SettingVerification — режим проверки оплат (off | auto | manual), меняет администратор.
const SettingVerification = "payment_verification"

// MaxReceiptSize — предельный размер файла чека.
const MaxReceiptSize = 10 << 20

// AutoActor — от чьего имени автопроверка подтверждает оплату (видно в журнале).
var AutoActor = Actor{Login: "автопроверка", UserAgent: "receipt-check"}

// ReceiptNotifier — уведомления о чеках (Telegram): администратору — новый чек на проверку,
// пользователю — решение по его чеку. Реализует notify.Hub.
type ReceiptNotifier interface {
	ReceiptForReview(r *models.Receipt, file []byte)
	ReceiptDecided(r *models.Receipt)
}

type ReceiptService struct {
	st     *store.Store
	audit  *Audit
	pay    *PaymentService
	an     *receipt.Analyzer
	notify ReceiptNotifier
}

func NewReceiptService(st *store.Store, audit *Audit, pay *PaymentService, an *receipt.Analyzer) *ReceiptService {
	s := &ReceiptService{st: st, audit: audit, pay: pay, an: an}
	pay.requireReceipt = s.RequiresReceipt
	pay.receiptsPaidByAdmin = func(ctx context.Context, occID uint, by string) {
		if err := st.ApprovePendingForOccurrence(ctx, occID, by); err != nil {
			log.Printf("receipts: approve pending for #%d: %v", occID, err)
		}
	}
	return s
}

func (s *ReceiptService) SetNotifier(n ReceiptNotifier) { s.notify = n }

func (s *ReceiptService) Capabilities() receipt.Capabilities { return s.an.Capabilities() }

// Policy — текущий режим проверки (по умолчанию off — «Оплачено» без чека, как в ТЗ).
func (s *ReceiptService) Policy(ctx context.Context) string {
	m, err := s.st.Settings(ctx)
	if err != nil {
		return models.VerifyOff
	}
	switch v := m[SettingVerification]; v {
	case models.VerifyAuto, models.VerifyManual:
		return v
	}
	// по умолчанию — как в ТЗ: пользователь сам отмечает «Оплачено» (проверку по чеку включает администратор)
	return models.VerifyOff
}

func (s *ReceiptService) SetPolicy(ctx context.Context, a Actor, v string) error {
	if err := requireAdmin(a); err != nil {
		return err
	}
	if v != models.VerifyOff && v != models.VerifyAuto && v != models.VerifyManual {
		return Invalid("payment_verification must be off, auto or manual")
	}
	return s.st.SetSetting(ctx, SettingVerification, v)
}

// RequiresReceipt — пользователь подтверждает оплату только чеком (администратор отмечает сам).
func (s *ReceiptService) RequiresReceipt(ctx context.Context, a Actor) bool {
	return a.Role != models.RoleAdmin && s.Policy(ctx) != models.VerifyOff
}

// PendingCount — сколько чеков ждут решения (бейдж у администратора).
func (s *ReceiptService) PendingCount(ctx context.Context) (int64, error) {
	return s.st.CountReceipts(ctx, models.ReceiptPending)
}

// ---------- загрузка ----------

type Upload struct {
	Name   string
	Note   string
	Source string // web | telegram
	Data   []byte
}

// SubmitResult — чек и платёж после проверки.
type SubmitResult struct {
	Receipt    *models.Receipt        `json:"receipt"`
	Occurrence *models.OccurrenceView `json:"occurrence"`
}

var allowedMime = map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp", "application/pdf": ".pdf"}

// Submit — пользователь прикладывает чек к периоду платежа. Чек распознаётся и проверяется сразу:
// если режим auto и все проверки пройдены — оплата подтверждается, иначе чек ждёт администратора.
func (s *ReceiptService) Submit(ctx context.Context, a Actor, occID uint, up Upload) (*SubmitResult, error) {
	occ, err := s.pay.GetOccurrence(ctx, a, occID)
	if err != nil {
		return nil, err
	}
	if occ.PaidAt != nil {
		return nil, Conflict("payment is already marked as paid")
	}
	if len(up.Data) == 0 {
		return nil, Invalid("file is required")
	}
	if len(up.Data) > MaxReceiptSize {
		return nil, Invalid("file is too large: max 10 MB")
	}
	mime := http.DetectContentType(up.Data)
	if i := strings.Index(mime, ";"); i > 0 {
		mime = mime[:i]
	}
	if _, ok := allowedMime[mime]; !ok {
		return nil, Invalid("unsupported file type: upload a photo (JPG, PNG, WebP) or a PDF")
	}
	if utf8.RuneCountInString(up.Note) > 1000 {
		return nil, Invalid("note is too long")
	}
	if up.Source == "" {
		up.Source = "web"
	}
	name := sanitizeName(up.Name, allowedMime[mime])
	sum := sha256.Sum256(up.Data)

	r := &models.Receipt{
		OccurrenceID: &occ.ID, UserID: a.UserID, SubmittedBy: a.Login, Source: up.Source,
		PaymentTitle: occ.Title, PaymentAmount: occ.Amount, PaymentCurrency: occ.Currency, PaymentDueDate: &occ.DueDate, OwnerLogin: occ.OwnerLogin,
		FileName: name, Mime: mime, Size: len(up.Data), SHA256: hex.EncodeToString(sum[:]), Content: up.Data,
		Note: strings.TrimSpace(up.Note), Status: models.ReceiptPending, Checks: json.RawMessage("[]"),
	}
	if err := s.analyze(ctx, r, occ, up.Data); err != nil {
		return nil, err
	}
	if err := s.st.CreateReceipt(ctx, r); err != nil {
		return nil, err
	}
	if n, err := s.st.SupersedePending(ctx, occ.ID, r.ID); err == nil && n > 0 {
		s.audit.Log(ctx, a, ActReceiptReject, "receipt", nil, D{"occurrence_id": occ.ID, "reason": "superseded", "by_receipt": r.ID})
	}
	var checks []models.ReceiptCheck
	_ = json.Unmarshal(r.Checks, &checks)
	s.audit.Log(ctx, a, ActReceiptSubmit, "receipt", r.ID, D{
		"occurrence_id": occ.ID, "payment": occ.Title, "file": name, "size": r.Size, "source": r.Source,
		"methods": r.Methods, "score": r.Score, "checks": summarizeChecks(checks),
	})

	policy := s.Policy(ctx)
	if policy == models.VerifyAuto && allOK(checks) {
		if err := s.approve(ctx, AutoActor, r, nil, "Все проверки пройдены", true); err != nil {
			return nil, err
		}
	} else {
		v, _ := s.pay.occurrence(ctx, occ.ID)
		s.pay.emit(models.EventReceiptPending, v, a.Login)
		if s.notify != nil {
			s.notify.ReceiptForReview(r, up.Data)
		}
	}
	return s.result(ctx, r.ID)
}

func (s *ReceiptService) result(ctx context.Context, id uint) (*SubmitResult, error) {
	r, err := s.st.ReceiptByID(ctx, id)
	if err != nil {
		return nil, err
	}
	out := &SubmitResult{Receipt: r}
	if r.OccurrenceID != nil {
		out.Occurrence, _ = s.pay.occurrence(ctx, *r.OccurrenceID)
	}
	return out, nil
}

// analyze распознаёт файл и заполняет поля чека и проверки.
func (s *ReceiptService) analyze(ctx context.Context, r *models.Receipt, occ *models.OccurrenceView, data []byte) error {
	e := s.an.Analyze(ctx, data, r.Mime)
	for _, msg := range e.Errors {
		log.Printf("receipt: %s: %s", r.FileName, msg)
	}
	r.Methods = strings.Join(e.Methods, ",")
	r.ExtAmount, r.ExtCurrency, r.ExtDate, r.ExtOperationID, r.ExtPayee = e.Total, nil, nil, nil, nil
	if e.Currency != "" {
		c := e.Currency
		r.ExtCurrency = &c
	}
	if e.Date != nil {
		d := models.DateOf(*e.Date)
		r.ExtDate = &d
	}
	if e.OperationID != "" {
		op := truncate(e.OperationID, 200)
		r.ExtOperationID = &op
	}
	if e.Payee != "" {
		p := truncate(e.Payee, 300)
		r.ExtPayee = &p
	}
	r.ExtText = e.Text
	if e.AI != nil {
		r.AIComment = truncate(strings.TrimSpace(e.AI.Comment+" "+strings.Join(e.AI.Suspicious, "; ")), 1000)
	}

	op := ""
	if r.ExtOperationID != nil {
		op = *r.ExtOperationID
	}
	dups, err := s.st.ReceiptDuplicates(ctx, r.SHA256, op, occ.ID)
	if err != nil {
		return err
	}
	if r.ID != 0 { // перепроверка: сам себя дублем не считает
		filtered := dups[:0]
		for _, d := range dups {
			if d.ID != r.ID {
				filtered = append(filtered, d)
			}
		}
		dups = filtered
	}
	period := 1
	if rp, err := s.st.RecurringByID(ctx, occ.RecurringPaymentID); err == nil {
		period = max(1, models.PeriodMonths(rp.Period))
	}
	checks, score := Evaluate(Expected{Amount: occ.Amount, Currency: occ.Currency, DueDate: occ.DueDate, PeriodMonths: period, SHA256: r.SHA256}, e, dups, s.pay.Today())
	r.Checks, _ = json.Marshal(checks)
	r.Score = score
	return nil
}

// ---------- правила ----------

// Expected — что должно быть в чеке.
type Expected struct {
	Amount       decimal.Decimal
	Currency     string
	DueDate      models.Date
	PeriodMonths int
	SHA256       string // хеш проверяемого файла (чтобы отличить дубль файла от дубля номера операции)
}

// Evaluate сверяет распознанный документ с платежом. Ни одна проверка не отклоняет чек сама:
// fail и warn означают «решает администратор»; автоматически подтверждается только полностью чистый чек.
func Evaluate(exp Expected, e *receipt.Extracted, dups []models.Receipt, today models.Date) ([]models.ReceiptCheck, int) {
	var out []models.ReceiptCheck
	add := func(code, status, title, detail string) {
		out = append(out, models.ReceiptCheck{Code: code, Status: status, Title: title, Detail: detail})
	}
	money := func(d decimal.Decimal, cur string) string { return fmtMoney(d) + " " + curSign(cur) }

	// 1. похоже ли это на платёжный документ
	switch {
	case e.Fiscal != nil:
		add("document", models.CheckOK, "Кассовый чек", "QR-код ФНС прочитан: "+e.Fiscal.OperationID())
	case e.AI != nil && !e.AI.IsPaymentDocument:
		add("document", models.CheckFail, "Не похоже на чек", "ИИ-модель не считает документ подтверждением оплаты")
	case e.PaymentLike:
		add("document", models.CheckOK, "Платёжный документ", "Найдены признаки чека или квитанции ("+methodsRU(e.Methods)+")")
	case strings.TrimSpace(e.Text) == "":
		add("document", models.CheckWarn, "Документ не прочитан", "Не удалось распознать текст — проверьте файл вручную")
	default:
		add("document", models.CheckWarn, "Не похоже на чек", "В тексте нет слов «чек», «оплата», «перевод», «итого»")
	}

	// 2. статус операции
	switch {
	case len(e.Negative) > 0:
		add("status", models.CheckFail, "Операция не выполнена", "В документе: «"+strings.Join(e.Negative, "», «")+"»")
	case e.Pending:
		add("status", models.CheckWarn, "Операция в обработке", "Платёж ещё не завершён")
	default:
		add("status", models.CheckOK, "Признаков отказа нет", "Нет слов «отклонено», «возврат», «ошибка»")
	}

	// 3. можно ли доверять документу без человека: нужен фискальный QR или номер операции.
	// Текст «Сумма … Дата … Выполнено» легко набрать вручную, поэтому без этих данных чек всегда смотрит администратор.
	switch {
	case e.Fiscal != nil:
		add("authenticity", models.CheckOK, "Есть фискальные данные", "QR-код ФНС: "+e.Fiscal.OperationID())
	case e.OperationID != "":
		add("authenticity", models.CheckOK, "Есть номер операции", e.OperationID)
	default:
		add("authenticity", models.CheckWarn, "Нет QR-кода и номера операции", "Без них подлинность не проверить автоматически — чек посмотрит администратор")
	}

	// 4. сумма
	gotCur := exp.Currency // в чём сумма в документе (если валюта в нём видна)
	if e.Currency != "" {
		gotCur = e.Currency
	}
	matched := false
	for _, v := range e.Amounts {
		if v.Sub(exp.Amount).Abs().LessThanOrEqual(decimal.NewFromFloat(0.01)) {
			matched = true
		}
	}
	want := money(exp.Amount, exp.Currency)
	switch {
	case e.Total != nil && e.Total.Sub(exp.Amount).Abs().LessThanOrEqual(decimal.NewFromFloat(0.01)):
		add("amount", models.CheckOK, "Сумма совпадает", money(*e.Total, gotCur)+sourceRU(e.TotalSource))
	case e.Total != nil && (e.TotalSource == "qr" || e.TotalSource == "ai") && !matched:
		add("amount", models.CheckFail, "Сумма не совпадает", "В чеке "+money(*e.Total, gotCur)+", ожидалось "+want)
	case matched:
		add("amount", models.CheckOK, "Сумма совпадает", "В документе есть "+want)
	case e.Total != nil && e.Total.GreaterThan(exp.Amount) && gotCur == exp.Currency:
		add("amount", models.CheckWarn, "Сумма больше ожидаемой", "В чеке "+money(*e.Total, gotCur)+", ожидалось "+want)
	case e.Total != nil:
		add("amount", models.CheckFail, "Сумма не совпадает", "В чеке "+money(*e.Total, gotCur)+", ожидалось "+want)
	case len(e.Amounts) > 0:
		var found []string
		for i, v := range e.Amounts {
			if i == 3 {
				break
			}
			found = append(found, money(v, gotCur))
		}
		add("amount", models.CheckFail, "Сумма не совпадает", "В документе: "+strings.Join(found, ", ")+"; ожидалось "+want)
	default:
		add("amount", models.CheckWarn, "Сумма не найдена", "Ожидалось "+want)
	}

	// 5. валюта (если в документе она видна)
	if e.Currency != "" {
		if e.Currency == exp.Currency {
			add("currency", models.CheckOK, "Валюта совпадает", exp.Currency)
		} else {
			add("currency", models.CheckFail, "Другая валюта", "В чеке "+e.Currency+", платёж в "+exp.Currency)
		}
	}

	// 6. дата: не в будущем и не раньше начала оплачиваемого периода
	from := models.DateOf(exp.DueDate.Time.AddDate(0, -exp.PeriodMonths, -7))
	switch {
	case e.Date == nil:
		add("date", models.CheckWarn, "Дата не найдена", "Дату оплаты не удалось определить")
	default:
		d := models.DateOf(*e.Date)
		switch {
		case d.After(today):
			add("date", models.CheckFail, "Дата в будущем", "В чеке "+d.Format("02.01.2006"))
		case d.Before(from):
			add("date", models.CheckFail, "Чек слишком старый", "Оплата "+d.Format("02.01.2006")+" — раньше начала периода (с "+from.Format("02.01.2006")+")")
		default:
			add("date", models.CheckOK, "Дата подходит", "Оплата "+d.Format("02.01.2006")+", срок "+exp.DueDate.Format("02.01.2006"))
		}
	}

	// 7. повторное использование
	if len(dups) > 0 {
		d := dups[0]
		what := "Тот же файл"
		if d.SHA256 != exp.SHA256 {
			what = "Тот же номер операции"
		}
		due := ""
		if d.PaymentDueDate != nil {
			due = " за " + d.PaymentDueDate.Format("02.01.2006")
		}
		add("duplicate", models.CheckFail, "Чек уже использован", fmt.Sprintf("%s в чеке #%d: «%s»%s (%s)", what, d.ID, d.PaymentTitle, due, d.SubmittedBy))
	} else {
		add("duplicate", models.CheckOK, "Чек не использовался", "Такого файла и номера операции в системе нет")
	}

	// 8. замечания ИИ-модели
	if e.AI != nil && len(e.AI.Suspicious) > 0 {
		add("ai", models.CheckWarn, "Замечания ИИ", strings.Join(e.AI.Suspicious, "; "))
	}

	score := 100
	for _, c := range out {
		switch c.Status {
		case models.CheckWarn:
			score -= 15
		case models.CheckFail:
			score -= 40
		}
	}
	return out, max(0, score)
}

func allOK(checks []models.ReceiptCheck) bool {
	for _, c := range checks {
		if c.Status != models.CheckOK {
			return false
		}
	}
	return len(checks) > 0
}

func summarizeChecks(checks []models.ReceiptCheck) map[string]string {
	m := map[string]string{}
	for _, c := range checks {
		m[c.Code] = c.Status
	}
	return m
}

func sourceRU(src string) string {
	switch src {
	case "qr":
		return " (QR-код чека)"
	case "ai":
		return " (ИИ-модель)"
	case "label":
		return " (строка «Итого/Сумма»)"
	}
	return ""
}

func methodsRU(m []string) string {
	ru := map[string]string{"qr": "QR-код", "pdf": "текст PDF", "ocr": "OCR", "ai": "ИИ-модель"}
	var out []string
	for _, v := range m {
		out = append(out, ru[v])
	}
	if len(out) == 0 {
		return "текст"
	}
	return strings.Join(out, ", ")
}

func curSign(c string) string {
	return map[string]string{"RUB": "₽", "USD": "$", "EUR": "€"}[c]
}

// fmtMoney: 1234.5 → «1 234,50».
func fmtMoney(d decimal.Decimal) string {
	s := d.StringFixed(2)
	intPart, frac := s[:len(s)-3], s[len(s)-2:]
	neg := strings.HasPrefix(intPart, "-")
	intPart = strings.TrimPrefix(intPart, "-")
	var b strings.Builder
	for i, r := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteRune(' ')
		}
		b.WriteRune(r)
	}
	if neg {
		return "-" + b.String() + "," + frac
	}
	return b.String() + "," + frac
}

// ---------- решения ----------

// approve — оплата подтверждена: чек принят, период отмечен оплаченным датой из чека.
func (s *ReceiptService) approve(ctx context.Context, a Actor, r *models.Receipt, paidAt *models.Date, comment string, auto bool) error {
	ok, err := s.st.DecideReceipt(ctx, r.ID, models.ReceiptApproved, a.Login, comment, auto)
	if err != nil {
		return err
	}
	if !ok {
		return Conflict("receipt is already decided")
	}
	today := s.pay.Today()
	d := today
	switch {
	case paidAt != nil:
		d = *paidAt
	case r.ExtDate != nil && !r.ExtDate.After(today):
		d = *r.ExtDate
	}
	action := ActReceiptApprove
	if auto {
		action = ActReceiptAuto
	}
	s.audit.Log(ctx, a, action, "receipt", r.ID, D{"occurrence_id": r.OccurrenceID, "payment": r.PaymentTitle, "paid_at": d.String(), "comment": comment})
	if r.OccurrenceID != nil {
		if err := s.pay.ConfirmPaid(ctx, a, *r.OccurrenceID, d, r.SubmittedBy, D{"receipt_id": r.ID, "verified": map[bool]string{true: "auto", false: "admin"}[auto]}); err != nil {
			return err
		}
	}
	if !auto && s.notify != nil {
		if fresh, err := s.st.ReceiptByID(ctx, r.ID); err == nil {
			s.notify.ReceiptDecided(fresh)
		}
	}
	return nil
}

func (s *ReceiptService) Approve(ctx context.Context, a Actor, id uint, paidAt *string, comment string) (*SubmitResult, error) {
	if err := requireAdmin(a); err != nil {
		return nil, err
	}
	r, err := s.st.ReceiptByID(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, NotFound("receipt not found")
	}
	if err != nil {
		return nil, err
	}
	var d *models.Date
	if paidAt != nil && *paidAt != "" {
		v, err := models.ParseDate(*paidAt)
		if err != nil {
			return nil, Invalid(err.Error())
		}
		if v.After(s.pay.Today()) {
			return nil, Invalid("paid_at cannot be in the future")
		}
		d = &v
	}
	if err := s.approve(ctx, a, r, d, truncate(strings.TrimSpace(comment), 1000), false); err != nil {
		return nil, err
	}
	return s.result(ctx, id)
}

func (s *ReceiptService) Reject(ctx context.Context, a Actor, id uint, comment string) (*SubmitResult, error) {
	if err := requireAdmin(a); err != nil {
		return nil, err
	}
	comment = truncate(strings.TrimSpace(comment), 1000)
	if comment == "" {
		return nil, Invalid("comment is required: tell the user why the receipt is rejected")
	}
	r, err := s.st.ReceiptByID(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, NotFound("receipt not found")
	}
	if err != nil {
		return nil, err
	}
	ok, err := s.st.DecideReceipt(ctx, id, models.ReceiptRejected, a.Login, comment, false)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, Conflict("receipt is already decided")
	}
	s.audit.Log(ctx, a, ActReceiptReject, "receipt", id, D{"occurrence_id": r.OccurrenceID, "payment": r.PaymentTitle, "comment": comment})
	if r.OccurrenceID != nil {
		if v, err := s.pay.occurrence(ctx, *r.OccurrenceID); err == nil {
			s.pay.emit(models.EventReceiptRejected, v, a.Login)
		}
	}
	if s.notify != nil {
		if fresh, err := s.st.ReceiptByID(ctx, id); err == nil {
			s.notify.ReceiptDecided(fresh)
		}
	}
	return s.result(ctx, id)
}

// Recheck — повторное распознавание (например, после подключения ИИ-модели). Решение не меняет.
func (s *ReceiptService) Recheck(ctx context.Context, a Actor, id uint) (*SubmitResult, error) {
	if err := requireAdmin(a); err != nil {
		return nil, err
	}
	r, err := s.st.ReceiptByID(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, NotFound("receipt not found")
	}
	if err != nil {
		return nil, err
	}
	data, err := s.st.ReceiptContent(ctx, id)
	if err != nil {
		return nil, err
	}
	occ := &models.OccurrenceView{}
	if r.OccurrenceID != nil {
		if v, err := s.pay.occurrence(ctx, *r.OccurrenceID); err == nil {
			occ = v
		}
	}
	if occ.ID == 0 { // платёж удалён — сверяем со снимком
		occ.Title, occ.Amount, occ.Currency = r.PaymentTitle, r.PaymentAmount, r.PaymentCurrency
		if r.PaymentDueDate != nil {
			occ.DueDate = *r.PaymentDueDate
		}
	}
	if err := s.analyze(ctx, r, occ, data); err != nil {
		return nil, err
	}
	if err := s.st.SaveReceiptAnalysis(ctx, r); err != nil {
		return nil, err
	}
	s.audit.Log(ctx, a, ActReceiptRecheck, "receipt", id, D{"methods": r.Methods, "score": r.Score})
	return s.result(ctx, id)
}

// ---------- просмотр ----------

// visible — администратор видит все чеки, пользователь — только свои.
func (s *ReceiptService) visible(a Actor, r *models.Receipt) bool {
	return a.Role == models.RoleAdmin || (a.UserID != nil && r.UserID != nil && *a.UserID == *r.UserID)
}

func (s *ReceiptService) Get(ctx context.Context, a Actor, id uint) (*models.Receipt, error) {
	r, err := s.st.ReceiptByID(ctx, id)
	if errors.Is(err, store.ErrNotFound) || (err == nil && !s.visible(a, r)) {
		return nil, NotFound("receipt not found")
	}
	return r, err
}

func (s *ReceiptService) File(ctx context.Context, a Actor, id uint) (*models.Receipt, []byte, error) {
	r, err := s.Get(ctx, a, id)
	if err != nil {
		return nil, nil, err
	}
	data, err := s.st.ReceiptContent(ctx, id)
	return r, data, err
}

func (s *ReceiptService) History(ctx context.Context, a Actor, id uint) ([]models.AuditLog, error) {
	if _, err := s.Get(ctx, a, id); err != nil {
		return nil, err
	}
	return s.st.ReceiptHistory(ctx, id)
}

func (s *ReceiptService) List(ctx context.Context, a Actor, status string, occurrenceID uint, limit int) ([]models.Receipt, error) {
	if status != "" && status != models.ReceiptPending && status != models.ReceiptApproved && status != models.ReceiptRejected {
		return nil, Invalid("status must be pending, approved or rejected")
	}
	f := store.ReceiptFilter{Status: status, OccurrenceID: occurrenceID, Limit: limit}
	if a.Role != models.RoleAdmin {
		if a.UserID == nil {
			return []models.Receipt{}, nil
		}
		f.UserID = a.UserID
	}
	return s.st.ListReceipts(ctx, f)
}

// ---------- helpers ----------

func sanitizeName(name, ext string) string {
	name = filepath.Base(strings.TrimSpace(strings.ReplaceAll(name, "\\", "/")))
	name = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`"<>|:*?`, r) {
			return '_'
		}
		return r
	}, name)
	if name == "" || name == "." || name == "/" {
		name = "receipt-" + time.Now().Format("20060102-150405") + ext
	}
	if utf8.RuneCountInString(name) > 200 {
		name = string([]rune(name)[:200])
	}
	return name
}
