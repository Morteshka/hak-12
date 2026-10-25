package receipt

import (
	"testing"
	"time"
)

// Тексты в формате, в котором их отдаёт pdftotext для квитанций популярных банков (данные вымышленные).
var bankTexts = map[string]struct {
	text   string
	total  string
	date   string
	op     string
	reject bool
}{
	"sber": {text: `Чек по операции
24 октября 2026 15:42:10 (МСК)
Операция
Перевод клиенту СберБанка
ФИО получателя
Иван Иванович И.
Номер карты получателя
•••• 1234
Сумма перевода
1 250,00 ₽
Комиссия
0,00 ₽
Номер документа
1000234567
Код авторизации
482910`, total: "1250", date: "2026-10-24", op: "1000234567"},
	"tbank": {text: `24.10.2026 15:42:10
Итого   399 ₽
Перевод    По номеру телефона
Статус     Успешно
Сумма      399 ₽
Комиссия   Без комиссии
Получатель Иван И.
Квитанция № 1-23-456-789-012`, total: "399", date: "2026-10-24", op: "1-23-456-789-012"},
	"alfa": {text: `Квитанция о переводе
Дата и время операции 24.10.2026 15:42
Сумма списания 2 990,50 RUB
Идентификатор операции A6297154512340000B10050011470501
Статус Исполнен`, total: "2990.5", date: "2026-10-24", op: "A6297154512340000B10050011470501"},
	"declined": {text: `Чек по операции
24.10.2026
Сумма перевода 399,00 ₽
Статус: Операция отклонена`, total: "399", date: "2026-10-24", reject: true},
	"ocr-rub": {text: "Перевод выполнен\n\n399,00 P\n\nДата и время\n\n24.10.2026 15:42\n\nКомиссия\n\n0,00 PB", date: "2026-10-24"},
}

func TestParseBankTexts(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Moscow")
	for name, tc := range bankTexts {
		e := &Extracted{}
		ParseText(e, tc.text, loc)
		if tc.total != "" && (e.Total == nil || e.Total.String() != tc.total) {
			t.Errorf("%s: total = %v, want %s", name, e.Total, tc.total)
		}
		if e.Date == nil || e.Date.Format("2006-01-02") != tc.date {
			t.Errorf("%s: date = %v, want %s", name, e.Date, tc.date)
		}
		if tc.op != "" && e.OperationID != tc.op {
			t.Errorf("%s: operation = %q, want %q", name, e.OperationID, tc.op)
		}
		if (len(e.Negative) > 0) != tc.reject {
			t.Errorf("%s: negative = %v", name, e.Negative)
		}
		if !e.PaymentLike {
			t.Errorf("%s: not recognized as payment", name)
		}
	}
	// «399,00 P» из OCR распознаётся как рубли
	e := &Extracted{}
	ParseText(e, bankTexts["ocr-rub"].text, loc)
	if e.Currency != "RUB" || len(e.Amounts) == 0 || e.Amounts[0].String() != "399" {
		t.Errorf("ocr ruble sign: currency=%q amounts=%v", e.Currency, e.Amounts)
	}
}

func TestParseFiscal(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Moscow")
	f, ok := ParseFiscal("t=20261024T1542&s=399.00&fn=7281440500123456&i=48211&fp=2709316512&n=1", loc)
	if !ok || f.Sum.String() != "399" || f.Time.Format("2006-01-02 15:04") != "2026-10-24 15:42" || f.KindRU() != "приход" {
		t.Fatalf("fiscal: %+v ok=%v", f, ok)
	}
	if _, ok := ParseFiscal("https://example.com", loc); ok {
		t.Error("plain URL must not be a fiscal QR")
	}
}
