package statement

import (
	"os"
	"strings"
	"testing"
	"time"
)

func find(cs []Candidate, sub string) *Candidate {
	for i := range cs {
		if strings.Contains(strings.ToLower(cs[i].Title), strings.ToLower(sub)) {
			return &cs[i]
		}
	}
	return nil
}

func TestTinkoffCP1251(t *testing.T) {
	data, err := os.ReadFile("testdata/tinkoff_cp1251.csv")
	if err != nil {
		t.Fatal(err)
	}
	txns, err := ParseCSV(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range txns {
		if strings.Contains(x.Description, "Зарплата") || strings.Contains(x.Description, "Снятие") {
			t.Fatalf("income or cash withdrawal parsed as expense: %+v", x)
		}
	}
	today := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	cs := Detect(txns, today)
	want := map[string]struct{ period, amount, cur, next string }{
		"Смирнов":     {"monthly", "85000", "RUB", "2026-10-26"},
		"Timeweb":     {"monthly", "1290", "RUB", "2026-11-03"},
		"Яндекс Плюс": {"monthly", "399", "RUB", "2026-11-05"},
		"МТС":         {"monthly", "650", "RUB", "2026-11-12"},
		"Apple":       {"monthly", "149", "RUB", "2026-11-17"},
		"OpenAI":      {"monthly", "20", "USD", "2026-11-09"},
		"Notion":      {"monthly", "10", "EUR", "2026-11-14"},
		"Kaspersky":   {"quarterly", "2990", "RUB", "2026-11-20"},
		"Reg":         {"yearly", "1190", "RUB", "2027-10-15"},
	}
	for name, w := range want {
		c := find(cs, name)
		if c == nil {
			t.Errorf("not found: %s; got %v", name, titles(cs))
			continue
		}
		if c.Period != w.period || c.Amount.String() != w.amount || c.Currency != w.cur || c.NextDue != w.next {
			t.Errorf("%s: got %s %s %s next %s", name, c.Period, c.Amount, c.Currency, c.NextDue)
		}
	}
	for _, noise := range []string{"Пятёрочка", "Такси", "Кофемания", "Зарплата", "Магнит"} {
		if find(cs, noise) != nil {
			t.Errorf("noise detected as recurring: %s", noise)
		}
	}
	if len(cs) != len(want) {
		t.Errorf("want %d candidates, got %d: %v", len(want), len(cs), titles(cs))
	}
}

func TestSberText(t *testing.T) {
	data, _ := os.ReadFile("testdata/sber.txt")
	txns, err := ParseText(string(data))
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range txns {
		if x.Amount.String() == "95000" {
			t.Fatal("income with + parsed as expense")
		}
	}
	cs := Detect(txns, time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC))
	if c := find(cs, "МТС"); c == nil || c.Amount.String() != "650" || c.NextDue != "2026-11-03" {
		t.Fatalf("МТС: %+v (all %v)", c, titles(cs))
	}
	if c := find(cs, "Смирнов"); c == nil || c.Amount.String() != "85000" {
		t.Fatalf("аренда: %+v (all %v)", c, titles(cs))
	}
}

func TestParseAmount(t *testing.T) {
	for in, want := range map[string]string{"-1 234,56": "-1234.56", "−299.00 ₽": "-299", "15,00 $": "15", "1.234.567,89": "1234567.89", "+95 000,00": "95000"} {
		got, ok := parseAmount(in)
		if !ok || got.String() != want {
			t.Errorf("%q: got %s %v, want %s", in, got, ok, want)
		}
	}
}

func titles(cs []Candidate) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.Title+"/"+c.Period+"/"+c.Amount.String())
	}
	return out
}
