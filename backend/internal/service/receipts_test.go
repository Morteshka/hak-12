package service

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"invoicekeeper/internal/models"
	"invoicekeeper/internal/receipt"
)

func status(checks []models.ReceiptCheck, code string) string {
	for _, c := range checks {
		if c.Code == code {
			return c.Status
		}
	}
	return ""
}

func TestEvaluate(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Moscow")
	today := models.DateOf(time.Date(2026, 10, 24, 0, 0, 0, 0, loc))
	exp := Expected{Amount: decimal.RequireFromString("399"), Currency: "RUB", DueDate: today.AddDays(3), PeriodMonths: 1, SHA256: "aaa"}
	parse := func(text string) *receipt.Extracted {
		e := &receipt.Extracted{}
		receipt.ParseText(e, text, loc)
		return e
	}
	good := "Чек по операции\n24.10.2026\nСумма перевода\n399,00 ₽\nНомер документа 1000234567"

	checks, score := Evaluate(exp, parse(good), nil, today)
	if !allOK(checks) || score != 100 {
		t.Fatalf("clean receipt must pass: %+v", checks)
	}
	checks, _ = Evaluate(exp, parse("Чек по операции\n24.10.2026\nСумма перевода\n350,00 ₽"), nil, today)
	if status(checks, "amount") != models.CheckFail {
		t.Errorf("wrong amount must fail: %+v", checks)
	}
	checks, _ = Evaluate(exp, parse("Чек по операции\n24.11.2026\nСумма 399,00 ₽"), nil, today)
	if status(checks, "date") != models.CheckFail {
		t.Errorf("future date must fail: %+v", checks)
	}
	checks, _ = Evaluate(exp, parse("Чек по операции\n01.06.2026\nСумма 399,00 ₽"), nil, today)
	if status(checks, "date") != models.CheckFail {
		t.Errorf("old receipt must fail: %+v", checks)
	}
	checks, _ = Evaluate(exp, parse("Чек по операции\n24.10.2026\nСумма 399,00 $"), nil, today)
	if status(checks, "currency") != models.CheckFail {
		t.Errorf("other currency must fail: %+v", checks)
	}
	// без номера операции и QR автоматически не подтверждаем, даже если сумма и дата подходят
	checks, _ = Evaluate(exp, parse("Чек по операции\n24.10.2026\nСумма перевода\n399,00 ₽\nВыполнено"), nil, today)
	if allOK(checks) || status(checks, "authenticity") != models.CheckWarn {
		t.Errorf("receipt without operation id must go to admin: %+v", checks)
	}
	dup := []models.Receipt{{ID: 7, SHA256: "aaa", PaymentTitle: "Кино", SubmittedBy: "user"}}
	checks, _ = Evaluate(exp, parse(good), dup, today)
	if status(checks, "duplicate") != models.CheckFail || allOK(checks) {
		t.Errorf("duplicate must fail: %+v", checks)
	}
	checks, _ = Evaluate(exp, parse("Фото из отпуска"), nil, today)
	if allOK(checks) || status(checks, "document") != models.CheckWarn {
		t.Errorf("not a receipt must go to admin: %+v", checks)
	}
}

func TestFmtMoney(t *testing.T) {
	if got := fmtMoney(decimal.RequireFromString("85000")); got != "85 000,00" {
		t.Errorf("fmtMoney = %q", got)
	}
}
