// Package statement — разбор банковской выписки (CSV или текст PDF) и поиск регулярных списаний:
// подписок, аренды, хостинга. Ничего не сохраняет — только предлагает, что добавить.
package statement

import (
	"bytes"
	"encoding/csv"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shopspring/decimal"
)

// Txn — одно списание из выписки. Amount всегда положительный (расход).
type Txn struct {
	Date        time.Time
	Amount      decimal.Decimal
	Currency    string
	Description string
}

// ErrNoTransactions — в файле не нашлось ни одной операции списания.
var ErrNoTransactions = errors.New("no transactions found")

// ParseCSV разбирает CSV-выписку: Т-Банк, Сбер, Альфа и любой файл с колонками «дата», «сумма», «описание».
// Кодировка — UTF-8 или Windows-1251, разделитель — «;», «,» или табуляция.
func ParseCSV(data []byte) ([]Txn, error) {
	text := decode(data)
	delim := detectDelimiter(text)
	r := csv.NewReader(strings.NewReader(text))
	r.Comma = delim
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil && len(rows) == 0 {
		return nil, err
	}
	head, cols := findHeader(rows)
	if head < 0 { // заголовка нет — разбираем строки как текст
		var lines []string
		for _, row := range rows {
			lines = append(lines, strings.Join(row, "  "))
		}
		return ParseText(strings.Join(lines, "\n"))
	}
	// суммы со знаком: если в файле есть отрицательные — списания это они (Т-Банк, Альфа),
	// иначе выписка показывает только расходы без знака
	var all []Txn
	neg := false
	for _, row := range rows[head+1:] {
		if t, ok := cols.txn(row); ok {
			all = append(all, t)
			neg = neg || t.Amount.IsNegative()
		}
	}
	var out []Txn
	for _, t := range all {
		if neg && !t.Amount.IsNegative() {
			continue // зачисление
		}
		t.Amount = t.Amount.Abs()
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil, ErrNoTransactions
	}
	return out, nil
}

// columns — номера колонок в CSV (-1 — колонки нет).
type columns struct {
	date, amount, debit, credit, currency, desc, category, status int
}

func (c columns) txn(row []string) (Txn, bool) {
	cell := func(i int) string {
		if i < 0 || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}
	if st := strings.ToLower(cell(c.status)); st != "" && (strings.Contains(st, "fail") || strings.Contains(st, "отклон") || strings.Contains(st, "отмен")) {
		return Txn{}, false
	}
	d, ok := parseDate(cell(c.date))
	if !ok {
		return Txn{}, false
	}
	// amt — со знаком: минус — списание. Колонка «расход» всегда списание.
	var amt decimal.Decimal
	switch {
	case c.debit >= 0 && cell(c.debit) != "":
		v, ok := parseAmount(cell(c.debit))
		if !ok || v.IsZero() {
			return Txn{}, false
		}
		amt = v.Abs().Neg()
	case c.amount >= 0:
		raw := cell(c.amount)
		v, ok := parseAmount(raw)
		if !ok || v.IsZero() || strings.HasPrefix(raw, "+") {
			return Txn{}, false // пусто или явное зачисление
		}
		amt = v
	default:
		return Txn{}, false
	}
	desc := cell(c.desc)
	if desc == "" {
		desc = cell(c.category)
	}
	if desc == "" || isTransferToSelf(desc) {
		return Txn{}, false
	}
	return Txn{Date: d, Amount: amt, Currency: currencyOf(cell(c.currency)), Description: desc}, true
}

var headerWords = map[string][]string{
	"date":     {"дата операции", "дата транзакции", "дата", "date", "posted"},
	"amount":   {"сумма операции", "сумма в валюте счета", "сумма платежа", "сумма", "amount"},
	"debit":    {"расход", "списание", "дебет", "debit", "withdrawal"},
	"credit":   {"приход", "зачисление", "кредит", "credit", "deposit"},
	"currency": {"валюта операции", "валюта", "currency"},
	"desc":     {"описание", "назначение", "контрагент", "получатель", "merchant", "description", "details", "payee"},
	"category": {"категория", "category"},
	"status":   {"статус", "status"},
}

// findHeader ищет строку заголовка в первых 15 строках и сопоставляет колонки.
func findHeader(rows [][]string) (int, columns) {
	for i := 0; i < len(rows) && i < 15; i++ {
		c := columns{-1, -1, -1, -1, -1, -1, -1, -1}
		set := func(field string, j int) {
			ptr := map[string]*int{"date": &c.date, "amount": &c.amount, "debit": &c.debit, "credit": &c.credit,
				"currency": &c.currency, "desc": &c.desc, "category": &c.category, "status": &c.status}[field]
			if *ptr < 0 {
				*ptr = j
			}
		}
		// сначала точные длинные названия («дата операции» важнее «дата платежа»)
		for _, field := range []string{"date", "amount", "debit", "credit", "currency", "desc", "category", "status"} {
			for _, w := range headerWords[field] {
				for j, h := range rows[i] {
					hh := strings.ToLower(strings.TrimSpace(strings.Trim(h, "\ufeff\"")))
					if hh == w || strings.HasPrefix(hh, w) {
						set(field, j)
					}
				}
			}
		}
		if c.date >= 0 && (c.amount >= 0 || c.debit >= 0) {
			return i, c
		}
	}
	return -1, columns{}
}

// ParseText разбирает текст выписки (pdftotext или CSV без заголовка): строка с датой и суммой — операция.
func ParseText(text string) ([]Txn, error) {
	var out []Txn
	lines := strings.Split(strings.ReplaceAll(text, "\r", ""), "\n")
	for i, line := range lines {
		dm := dateRe.FindStringSubmatchIndex(line)
		if dm == nil {
			continue
		}
		d, ok := parseDate(line[dm[0]:dm[1]])
		if !ok {
			continue
		}
		rest := line[dm[1]:]
		// сумма — последнее «денежное» число в строке
		ams := amountRe.FindAllStringSubmatchIndex(rest, -1)
		if len(ams) == 0 {
			continue
		}
		m := ams[len(ams)-1]
		raw := rest[m[0]:m[1]]
		if strings.Contains(raw, "+") {
			continue // зачисление
		}
		amt, ok := parseAmount(raw)
		if !ok || amt.IsZero() {
			continue
		}
		desc := strings.TrimSpace(rest[:m[0]])
		desc = timeRe.ReplaceAllString(desc, "")
		desc = dateRe.ReplaceAllString(desc, "")
		desc = strings.Join(strings.Fields(desc), " ")
		if len([]rune(desc)) < 3 && i+1 < len(lines) { // описание на следующей строке (Сбер)
			next := strings.TrimSpace(lines[i+1])
			if next != "" && dateRe.FindStringIndex(next) == nil {
				desc = strings.Join(strings.Fields(next), " ")
			}
		}
		if desc == "" || isTransferToSelf(desc) {
			continue
		}
		out = append(out, Txn{Date: d, Amount: amt.Abs(), Currency: currencyOf(rest[m[0]:m[1]] + " " + line), Description: desc})
	}
	if len(out) == 0 {
		return nil, ErrNoTransactions
	}
	return out, nil
}

var (
	dateRe   = regexp.MustCompile(`\b(\d{2})[./](\d{2})[./](\d{4}|\d{2})\b|\b(\d{4})-(\d{2})-(\d{2})\b`)
	timeRe   = regexp.MustCompile(`\b\d{1,2}:\d{2}(:\d{2})?\b`)
	amountRe = regexp.MustCompile(`[-−–+]?\s?\d{1,3}(?:[ \x{00a0}\x{202f}]\d{3})*(?:[.,]\d{2})(?:\s?(?:₽|руб\.?|RUB|р\.|\$|USD|€|EUR))?`)
)

func parseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if len(s) >= 10 {
		s2 := s[:10]
		for _, layout := range []string{"02.01.2006", "2006-01-02", "02/01/2006"} {
			if t, err := time.Parse(layout, s2); err == nil {
				return t, true
			}
		}
	}
	if len(s) >= 8 {
		if t, err := time.Parse("02.01.06", s[:8]); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// parseAmount: «-1 234,56», «−299.00 ₽», «15,00 $» → число со знаком.
func parseAmount(s string) (decimal.Decimal, bool) {
	s = strings.TrimSpace(s)
	neg := strings.HasPrefix(s, "-") || strings.HasPrefix(s, "−") || strings.HasPrefix(s, "–")
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ',' || r == '.':
			b.WriteRune('.')
		}
	}
	v := b.String()
	// «1.234.56» → оставляем только последнюю точку как десятичную
	if strings.Count(v, ".") > 1 {
		i := strings.LastIndex(v, ".")
		v = strings.ReplaceAll(v[:i], ".", "") + v[i:]
	}
	d, err := decimal.NewFromString(v)
	if err != nil {
		return decimal.Zero, false
	}
	if neg {
		d = d.Neg()
	}
	return d, true
}

func currencyOf(s string) string {
	u := strings.ToUpper(s)
	switch {
	case strings.Contains(u, "USD") || strings.Contains(s, "$"):
		return "USD"
	case strings.Contains(u, "EUR") || strings.Contains(s, "€"):
		return "EUR"
	}
	return "RUB"
}

// isTransferToSelf — переводы между своими счетами и снятие наличных подписками не бывают.
func isTransferToSelf(desc string) bool {
	d := strings.ToLower(desc)
	for _, w := range []string{"между своими", "перевод себе", "снятие налич", "банкомат", "atm", "пополнение", "cashback", "кэшбэк", "кешбэк"} {
		if strings.Contains(d, w) {
			return true
		}
	}
	return false
}

func detectDelimiter(text string) rune {
	sample := text
	if len(sample) > 4000 {
		sample = sample[:4000]
	}
	best, n := ';', strings.Count(sample, ";")
	for _, c := range []rune{',', '\t'} {
		if k := strings.Count(sample, string(c)); k > n {
			best, n = c, k
		}
	}
	return best
}

// decode — UTF-8 (с BOM или без) или Windows-1251.
func decode(data []byte) string {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if utf8.Valid(data) {
		return string(data)
	}
	var b strings.Builder
	for _, c := range data {
		if c < 0x80 {
			b.WriteByte(c)
			continue
		}
		b.WriteRune(cp1251[c-0x80])
	}
	return b.String()
}

// cp1251 — вторая половина таблицы Windows-1251.
var cp1251 = [128]rune{
	'Ђ', 'Ѓ', '‚', 'ѓ', '„', '…', '†', '‡', '€', '‰', 'Љ', '‹', 'Њ', 'Ќ', 'Ћ', 'Џ',
	'ђ', '‘', '’', '“', '”', '•', '–', '—', '�', '™', 'љ', '›', 'њ', 'ќ', 'ћ', 'џ',
	' ', 'Ў', 'ў', 'Ј', '¤', 'Ґ', '¦', '§', 'Ё', '©', 'Є', '«', '¬', '­', '®', 'Ї',
	'°', '±', 'І', 'і', 'ґ', 'µ', '¶', '·', 'ё', '№', 'є', '»', 'ј', 'Ѕ', 'ѕ', 'ї',
	'А', 'Б', 'В', 'Г', 'Д', 'Е', 'Ж', 'З', 'И', 'Й', 'К', 'Л', 'М', 'Н', 'О', 'П',
	'Р', 'С', 'Т', 'У', 'Ф', 'Х', 'Ц', 'Ч', 'Ш', 'Щ', 'Ъ', 'Ы', 'Ь', 'Э', 'Ю', 'Я',
	'а', 'б', 'в', 'г', 'д', 'е', 'ж', 'з', 'и', 'й', 'к', 'л', 'м', 'н', 'о', 'п',
	'р', 'с', 'т', 'у', 'ф', 'х', 'ц', 'ч', 'ш', 'щ', 'ъ', 'ы', 'ь', 'э', 'ю', 'я',
}
