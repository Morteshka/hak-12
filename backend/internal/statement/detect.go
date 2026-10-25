package statement

import (
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/shopspring/decimal"
)

// Candidate — найденное регулярное списание, которое можно добавить как платёж.
type Candidate struct {
	Key        string          `json:"key"`
	Title      string          `json:"title"`
	Amount     decimal.Decimal `json:"amount"`   // последняя сумма
	Currency   string          `json:"currency"` // RUB, USD, EUR
	Period     string          `json:"period"`   // monthly, quarterly, yearly
	NextDue    string          `json:"next_due"` // YYYY-MM-DD — ближайшее списание от сегодня
	Count      int             `json:"count"`    // сколько раз встретилось
	Dates      []string        `json:"dates"`    // даты списаний (последние 6)
	Confidence string          `json:"confidence"`
	Monthly    decimal.Decimal `json:"monthly"` // сколько это в месяц
	Category   string          `json:"category"`
}

// periodRules — интервал между списаниями (дней) → периодичность и число месяцев.
var periodRules = []struct {
	period   string
	min, max int
	months   int
}{
	{"monthly", 25, 36, 1},
	{"quarterly", 80, 100, 3},
	{"yearly", 345, 385, 12},
}

// Detect группирует списания по получателю и находит повторяющиеся с одинаковым шагом и похожей суммой.
// today — от него считается ближайшая дата списания.
func Detect(txns []Txn, today time.Time) []Candidate {
	groups := map[string][]Txn{}
	titles := map[string]string{}
	for _, t := range txns {
		k := merchantKey(t.Description)
		if k == "" {
			continue
		}
		k += "|" + t.Currency
		groups[k] = append(groups[k], t)
		if _, ok := titles[k]; !ok {
			titles[k] = prettyTitle(t.Description)
		}
	}
	var out []Candidate
	for k, list := range groups {
		if c, ok := analyze(k, titles[k], list, today); ok {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Monthly.Equal(out[j].Monthly) {
			return out[i].Monthly.GreaterThan(out[j].Monthly)
		}
		return out[i].Title < out[j].Title
	})
	return out
}

func analyze(key, title string, list []Txn, today time.Time) (Candidate, bool) {
	sort.Slice(list, func(i, j int) bool { return list[i].Date.Before(list[j].Date) })
	// одно списание в день (дробные платежи одного дня не считаем отдельными)
	var uniq []Txn
	for _, t := range list {
		if n := len(uniq); n > 0 && uniq[n-1].Date.Equal(t.Date) {
			continue
		}
		uniq = append(uniq, t)
	}
	if len(uniq) < 2 {
		return Candidate{}, false
	}
	var gaps []int
	for i := 1; i < len(uniq); i++ {
		gaps = append(gaps, int(uniq[i].Date.Sub(uniq[i-1].Date).Hours()/24))
	}
	med := median(gaps)
	rule := -1
	for i, r := range periodRules {
		if med >= r.min && med <= r.max {
			rule = i
		}
	}
	if rule < 0 {
		return Candidate{}, false
	}
	r := periodRules[rule]
	// не меньше 2/3 интервалов в окне периода
	fit := 0
	for _, g := range gaps {
		if g >= r.min && g <= r.max {
			fit++
		}
	}
	if fit*3 < len(gaps)*2 {
		return Candidate{}, false
	}
	// месячные — минимум 2 раза; суммы похожи (±20% от медианы — цены иногда меняются)
	amts := make([]decimal.Decimal, len(uniq))
	for i, t := range uniq {
		amts[i] = t.Amount
	}
	medAmt := medianDec(amts)
	same, stable := 0, 0
	for _, a := range amts {
		if a.Equal(medAmt) {
			same++
		}
		if a.Sub(medAmt).Abs().LessThanOrEqual(medAmt.Mul(decimal.NewFromFloat(0.2))) {
			stable++
		}
	}
	if stable*3 < len(amts)*2 {
		return Candidate{}, false
	}
	last := uniq[len(uniq)-1]
	next := last.Date.AddDate(0, r.months, 0)
	for next.Before(truncDay(today)) {
		next = next.AddDate(0, r.months, 0)
	}
	conf := "medium"
	if len(uniq) >= 3 && same == len(amts) {
		conf = "high"
	} else if len(uniq) == 2 && r.months == 1 {
		conf = "low"
	}
	var dates []string
	for _, t := range uniq {
		dates = append(dates, t.Date.Format("2006-01-02"))
	}
	if len(dates) > 6 {
		dates = dates[len(dates)-6:]
	}
	return Candidate{
		Key: key, Title: title, Amount: last.Amount, Currency: last.Currency, Period: r.period,
		NextDue: next.Format("2006-01-02"), Count: len(uniq), Dates: dates, Confidence: conf,
		Monthly:  last.Amount.Div(decimal.NewFromInt(int64(r.months))).Round(2),
		Category: categoryOf(title),
	}, true
}

func truncDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func median(xs []int) int {
	s := append([]int(nil), xs...)
	sort.Ints(s)
	return s[len(s)/2]
}

func medianDec(xs []decimal.Decimal) decimal.Decimal {
	s := append([]decimal.Decimal(nil), xs...)
	sort.Slice(s, func(i, j int) bool { return s[i].LessThan(s[j]) })
	return s[len(s)/2]
}

// шум в описаниях операций: номера карт, коды, города, служебные слова банков
var (
	noiseRe    = regexp.MustCompile(`(?i)(\*+\d+|\d{4,}|\b\d+\b|https?://\S+|www\.\S+|\.(ru|com|net|io|org)\b)`)
	stopWords  = map[string]bool{"оплата": true, "покупка": true, "списание": true, "платеж": true, "платёж": true, "по": true, "карте": true, "карта": true, "услуг": true, "услуги": true, "товаров": true, "в": true, "на": true, "за": true, "и": true, "rus": true, "russia": true, "moscow": true, "москва": true, "spb": true, "g": true, "gor": true, "ooo": true, "ооо": true, "ип": true, "ao": true, "ао": true, "pao": true, "пао": true, "llc": true, "inc": true, "ltd": true, "retail": true, "operation": true, "операция": true, "безналичная": true, "мск": true, "автоплатеж": true, "автоплатёж": true, "регулярный": true, "sbp": true, "сбп": true}
	categories = []struct {
		words []string
		name  string
	}{
		{[]string{"yandex", "яндекс", "plus", "плюс", "kinopoisk", "кинопоиск", "ivi", "okko", "netflix", "spotify", "youtube", "кино", "music", "музык", "premier", "wink", "more.tv", "start"}, "Кино и музыка"},
		{[]string{"apple", "icloud", "google", "microsoft", "adobe", "figma", "notion", "jetbrains", "openai", "chatgpt", "anthropic", "claude", "github", "zoom", "dropbox", "kaspersky", "антивирус"}, "Сервисы и софт"},
		{[]string{"vps", "hosting", "хостинг", "сервер", "server", "timeweb", "reg.ru", "beget", "selectel", "aeza", "cloud", "облак", "domain", "домен", "digitalocean", "hetzner"}, "Хостинг и облако"},
		{[]string{"мтс", "mts", "билайн", "beeline", "мегафон", "megafon", "tele2", "теле2", "ростелеком", "rostelecom", "интернет", "связь", "yota"}, "Связь и интернет"},
		{[]string{"аренд", "rent", "жкх", "квартплат", "коммунал", "электроэнерг", "мосэнерго", "газ"}, "Аренда и ЖКХ"},
		{[]string{"fitness", "фитнес", "спорт", "gym", "зал"}, "Спорт"},
		{[]string{"страхов", "insurance", "кредит", "loan", "ипотек"}, "Страховки и кредиты"},
	}
)

// merchantKey — устойчивый ключ получателя: без номеров, дат, городов и служебных слов.
func merchantKey(desc string) string {
	d := strings.ToLower(desc)
	d = noiseRe.ReplaceAllString(d, " ")
	var words []string
	for _, w := range strings.FieldsFunc(d, func(r rune) bool { return !unicode.IsLetter(r) && r != '+' }) {
		if len([]rune(w)) < 2 || stopWords[w] {
			continue
		}
		words = append(words, w)
		if len(words) == 3 {
			break
		}
	}
	return strings.Join(words, " ")
}

// brands — как правильно пишутся частые получатели (в выписках они обычно капсом).
var brands = map[string]string{
	"openai": "OpenAI", "chatgpt": "ChatGPT", "apple": "Apple", "icloud": "iCloud", "google": "Google", "youtube": "YouTube",
	"timeweb": "Timeweb", "notion": "Notion", "reg": "REG.RU", "beget": "Beget", "selectel": "Selectel", "aeza": "Aéza",
	"netflix": "Netflix", "spotify": "Spotify", "adobe": "Adobe", "figma": "Figma", "github": "GitHub", "jetbrains": "JetBrains",
	"microsoft": "Microsoft", "zoom": "Zoom", "dropbox": "Dropbox", "vps": "VPS", "мтс": "МТС", "ivi": "Иви", "okko": "Okko",
}

// служебные слова биллинга — в название платежа не идут
var billingWords = map[string]bool{"bill": true, "billing": true, "subscr": true, "subscription": true, "payment": true, "pay": true, "labs": true}

// prettyTitle — название для платежа: описание без номеров и служебных слов, бренды в правильном написании.
func prettyTitle(desc string) string {
	d := noiseRe.ReplaceAllString(desc, " ")
	d = strings.NewReplacer("/", " ", "*", " ", "_", " ").Replace(d)
	var words []string
	for _, w := range strings.Fields(d) {
		w = strings.Trim(w, ".,;:-—")
		lw := strings.ToLower(w)
		if lw == "" || stopWords[lw] || billingWords[lw] {
			continue
		}
		switch {
		case brands[lw] != "":
			w = brands[lw]
		case len([]rune(w)) > 3 && w == strings.ToUpper(w): // КАПС → Обычный
			r := []rune(lw)
			r[0] = unicode.ToUpper(r[0])
			w = string(r)
		}
		words = append(words, w)
		if len(words) == 5 {
			break
		}
	}
	t := strings.Join(words, " ")
	if t == "" {
		t = strings.TrimSpace(desc)
	}
	r := []rune(t)
	if len(r) > 60 {
		r = r[:60]
	}
	if len(r) > 0 && unicode.IsLower(r[0]) {
		r[0] = unicode.ToUpper(r[0])
	}
	return string(r)
}

func categoryOf(title string) string {
	t := strings.ToLower(title)
	for _, c := range categories {
		for _, w := range c.words {
			if strings.Contains(t, w) {
				return c.name
			}
		}
	}
	return "Другое"
}
