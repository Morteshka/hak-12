// Package receipt распознаёт платёжные документы: кассовые чеки (QR ФНС), банковские квитанции (PDF),
// фото и скриншоты (OCR Tesseract) и, если задан ключ, проверяет их ИИ-моделью.
// Пакет только извлекает данные; решение (подтвердить или отдать администратору) принимает service.
package receipt

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// Extracted — что удалось достать из документа.
type Extracted struct {
	Text        string            // распознанный текст (для администратора и поиска)
	Methods     []string          // qr, pdf, ocr, ai
	Total       *decimal.Decimal  // итоговая сумма (QR, ИИ или строка «Итого/Сумма»)
	TotalSource string            // откуда сумма: qr, ai, label
	Amounts     []decimal.Decimal // все суммы, найденные в тексте
	Currency    string            // RUB / USD / EUR, если удалось понять
	Date        *time.Time        // дата операции
	OperationID string            // номер операции / документа / ФН-ФД-ФП
	Payee       string            // получатель
	Fiscal      *Fiscal           // данные QR кассового чека
	PaymentLike bool              // документ похож на платёжный (ключевые слова, QR или ИИ)
	Negative    []string          // признаки отказа/возврата («отклонено», «возврат», …)
	Pending     bool              // «в обработке»
	AI          *AIResult         // ответ ИИ-модели (nil — не использовалась)
	Errors      []string          // технические ошибки распознавания (для журнала)
}

func (e *Extracted) addMethod(m string) {
	for _, v := range e.Methods {
		if v == m {
			return
		}
	}
	e.Methods = append(e.Methods, m)
}

// ---------- текст ----------

var (
	spaceRe = regexp.MustCompile(`[\x{00A0}\x{2007}\x{202F}\x{2009}]`)

	// сумма с валютой: 1 234,56 ₽ · 399.00 руб · $15.00 · 12,99 € · 399 RUB
	moneyRe = regexp.MustCompile(`(?i)(?:([$€])\s?)?(\d{1,3}(?:[ ']\d{3})+|\d+)(?:[.,](\d{1,2}))?\s?(₽|руб\.?|рублей|рубля|р\.|rub|usd|eur|\$|€|долл\.?|евро)?`)
	// число с копейками (для строк «Итого», «Сумма» без значка валюты)
	plainAmountRe = regexp.MustCompile(`(\d{1,3}(?:[ ']\d{3})+|\d+)[.,](\d{2})\b`)

	totalLabels = []string{"итого", "итог", "сумма перевода", "сумма платежа", "сумма операции", "сумма списания",
		"к оплате", "оплачено", "списано", "переведено", "сумма", "всего", "total", "amount", "paid"}
	// строки, где число — не сумма платежа
	notAmountLabels = []string{"комисси", "остаток", "баланс", "доступно", "кэшбэк", "cashback", "бонус", "ндс", "налог",
		"inn", "инн", "бик", "счёт", "счет", "карта", "card", "телефон", "phone", "кпп", "огрн", "ккт", "фн", "фд", "фп", "смена"}

	ocrRubRe = regexp.MustCompile(`(\d[.,]\d{2})\s?(?:P|Р)(?:B|В|Б)?([\s.,]|$)`)

	dateNumRe  = regexp.MustCompile(`\b(\d{1,2})[./](\d{1,2})[./](\d{4}|\d{2})\b`)
	dateISORe  = regexp.MustCompile(`\b(\d{4})-(\d{2})-(\d{2})\b`)
	dateWordRe = regexp.MustCompile(`(?i)\b(\d{1,2})\s+(январ\S*|феврал\S*|март\S*|апрел\S*|ма[йя]\S*|июн\S*|июл\S*|август\S*|сентябр\S*|октябр\S*|ноябр\S*|декабр\S*|янв|фев|мар|апр|авг|сен|сент|окт|ноя|дек)\.?\s+(\d{4})`)

	opRe    = regexp.MustCompile(`(?i)(номер операции|№ операции|идентификатор операции|id операции|номер документа|№ документа|номер квитанции|№ квитанции|квитанция №|квитанция|номер транзакции|номер платежа|код операции|код авторизации|идентификатор платежа|operation id|transaction id|transaction|сбп)[\s:№#]*([A-Za-zА-Яа-я0-9][A-Za-zА-Яа-я0-9\-]{5,})`)
	payeeRe = regexp.MustCompile(`(?i)(получатель|кому|магазин|организация|продавец|recipient|payee)[\s:]+(.{3,120})`)
)

var monthsRU = map[string]int{"янв": 1, "фев": 2, "мар": 3, "апр": 4, "мая": 5, "май": 5, "июн": 6, "июл": 7, "авг": 8, "сен": 9, "окт": 10, "ноя": 11, "дек": 12}

var (
	paymentWords  = []string{"чек", "оплат", "перевод", "платеж", "платёж", "квитанц", "итого", "сумма", "списан", "receipt", "payment", "total", "сбп", "кассов", "приход"}
	negativeWords = []string{"отклон", "не выполнен", "не проведен", "не проведён", "ошибка", "отменен", "отменён", "declined", "failed", "rejected", "возврат", "refund"}
	pendingWords  = []string{"в обработке", "обрабатывается", "processing", "ожидает"}
)

// NormalizeText приводит пробелы и регистр, чтобы регулярные выражения работали одинаково для PDF и OCR.
func NormalizeText(s string) string {
	s = spaceRe.ReplaceAllString(s, " ")
	// OCR читает «₽» как латинскую или русскую «P» (иногда «PB»): 399,00 P → 399,00 ₽
	s = ocrRubRe.ReplaceAllString(s, "$1 ₽$2")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\t", " ")
	return s
}

// ParseText достаёт из текста суммы, валюту, дату, номер операции и получателя.
func ParseText(e *Extracted, text string, loc *time.Location) {
	text = NormalizeText(text)
	if strings.TrimSpace(text) == "" {
		return
	}
	if e.Text != "" {
		e.Text += "\n"
	}
	e.Text += text
	low := strings.ToLower(text)

	for _, w := range paymentWords {
		if strings.Contains(low, w) {
			e.PaymentLike = true
			break
		}
	}
	for _, w := range negativeWords {
		if strings.Contains(low, w) && !strings.Contains(low, "без возврат") {
			e.Negative = appendUniq(e.Negative, w)
		}
	}
	for _, w := range pendingWords {
		if strings.Contains(low, w) {
			e.Pending = true
		}
	}

	// валюта: по количеству упоминаний
	cur := map[string]int{
		"RUB": strings.Count(low, "₽") + strings.Count(low, "руб") + strings.Count(low, " rub"),
		"USD": strings.Count(low, "$") + strings.Count(low, "usd") + strings.Count(low, "долл"),
		"EUR": strings.Count(low, "€") + strings.Count(low, "eur") + strings.Count(low, "евро"),
	}
	best, n := "", 0
	for _, c := range []string{"RUB", "USD", "EUR"} {
		if cur[c] > n {
			best, n = c, cur[c]
		}
	}
	if best != "" && e.Currency == "" {
		e.Currency = best
	}

	lines := strings.Split(text, "\n")
	// суммы: сначала строки с меткой «Итого / Сумма», затем все суммы со значком валюты
	var labeled []decimal.Decimal
	for i, ln := range lines {
		l := strings.ToLower(ln)
		if hasAny(l, notAmountLabels) && !hasAny(l, []string{"итого", "сумма", "total"}) {
			continue
		}
		if hasAny(l, totalLabels) {
			vals := amountsIn(ln, false)
			if len(vals) == 0 { // значение часто на следующей непустой строке
				if next := nextLine(lines, i); next != "" {
					vals = amountsIn(next, false)
				}
			}
			labeled = append(labeled, vals...)
		}
		e.Amounts = appendDec(e.Amounts, amountsIn(ln, true)...)
	}
	e.Amounts = appendDec(e.Amounts, labeled...)
	if e.Total == nil && len(labeled) > 0 {
		t := labeled[0]
		// «Итого» важнее «Сумма»: ищем строку с итогом
		for i, ln := range lines {
			if strings.Contains(strings.ToLower(ln), "итог") {
				if v := amountsIn(ln, false); len(v) > 0 {
					t = v[0]
					break
				} else if v := amountsIn(nextLine(lines, i), false); len(v) > 0 {
					t = v[0]
					break
				}
			}
		}
		e.Total, e.TotalSource = &t, "label"
	}

	if e.Date == nil {
		e.Date = findDate(lines, loc)
	}
	if e.OperationID == "" {
		if m := opRe.FindStringSubmatch(text); m != nil {
			e.OperationID = strings.TrimSpace(m[2])
		}
	}
	if e.Payee == "" {
		if m := payeeRe.FindStringSubmatch(text); m != nil {
			e.Payee = strings.TrimSpace(strings.SplitN(m[2], "\n", 2)[0])
		}
	}
}

// nextLine — следующая непустая строка (до трёх строк вниз): в PDF и OCR значение часто под подписью.
func nextLine(lines []string, i int) string {
	for j := i + 1; j < len(lines) && j <= i+4; j++ {
		if t := strings.TrimSpace(lines[j]); t != "" {
			return t
		}
	}
	return ""
}

// amountsIn — суммы в строке. needCurrency — только со значком валюты (иначе — любые числа с копейками).
func amountsIn(line string, needCurrency bool) []decimal.Decimal {
	var out []decimal.Decimal
	for _, m := range moneyRe.FindAllStringSubmatch(line, -1) {
		hasCur := m[1] != "" || m[4] != ""
		if needCurrency && !hasCur {
			continue
		}
		if !hasCur && m[3] == "" {
			continue // «12» без валюты и копеек — скорее номер или дата
		}
		if d, ok := toDec(m[2], m[3]); ok {
			out = append(out, d)
		}
	}
	if !needCurrency && len(out) == 0 {
		for _, m := range plainAmountRe.FindAllStringSubmatch(line, -1) {
			if d, ok := toDec(m[1], m[2]); ok {
				out = append(out, d)
			}
		}
	}
	return out
}

func toDec(intPart, frac string) (decimal.Decimal, bool) {
	intPart = strings.NewReplacer(" ", "", "'", "").Replace(intPart)
	if frac == "" {
		frac = "00"
	} else if len(frac) == 1 {
		frac += "0"
	}
	d, err := decimal.NewFromString(intPart + "." + frac)
	if err != nil || d.Sign() <= 0 || d.GreaterThan(decimal.NewFromInt(100_000_000)) {
		return decimal.Decimal{}, false
	}
	return d, true
}

// findDate — сначала строки с «дата», затем первая дата в документе.
func findDate(lines []string, loc *time.Location) *time.Time {
	var first *time.Time
	for _, ln := range lines {
		d := dateIn(ln, loc)
		if d == nil {
			continue
		}
		l := strings.ToLower(ln)
		if strings.Contains(l, "дата") || strings.Contains(l, "date") || strings.Contains(l, "время") {
			return d
		}
		if first == nil {
			first = d
		}
	}
	return first
}

func dateIn(s string, loc *time.Location) *time.Time {
	mk := func(y, m, d int) *time.Time {
		if y < 100 {
			y += 2000
		}
		if m < 1 || m > 12 || d < 1 || d > 31 || y < 2000 || y > 2100 {
			return nil
		}
		t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, loc)
		if t.Day() != d {
			return nil
		}
		return &t
	}
	if m := dateISORe.FindStringSubmatch(s); m != nil {
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		d, _ := strconv.Atoi(m[3])
		if t := mk(y, mo, d); t != nil {
			return t
		}
	}
	if m := dateWordRe.FindStringSubmatch(s); m != nil {
		d, _ := strconv.Atoi(m[1])
		y, _ := strconv.Atoi(m[3])
		w := strings.ToLower(m[2])
		for k, mo := range monthsRU {
			if strings.HasPrefix(w, k) {
				if t := mk(y, mo, d); t != nil {
					return t
				}
			}
		}
	}
	if m := dateNumRe.FindStringSubmatch(s); m != nil {
		d, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		y, _ := strconv.Atoi(m[3])
		if t := mk(y, mo, d); t != nil {
			return t
		}
	}
	return nil
}

func hasAny(s string, words []string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

func appendUniq(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}

func appendDec(list []decimal.Decimal, vals ...decimal.Decimal) []decimal.Decimal {
	for _, v := range vals {
		dup := false
		for _, x := range list {
			if x.Equal(v) {
				dup = true
				break
			}
		}
		if !dup {
			list = append(list, v)
		}
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].GreaterThan(list[j]) })
	return list
}
