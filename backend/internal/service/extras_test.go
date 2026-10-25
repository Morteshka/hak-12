package service

import (
	"testing"
	"time"

	"invoicekeeper/internal/models"
)

func TestPeriodBounds(t *testing.T) {
	at := models.NewDate(2028, time.February, 14)
	from, to := PeriodBounds(models.BudgetMonth, at)
	if from.String() != "2028-02-01" || to.String() != "2028-02-29" {
		t.Fatalf("month: %s..%s", from, to)
	}
	from, to = PeriodBounds(models.BudgetYear, at)
	if from.String() != "2028-01-01" || to.String() != "2028-12-31" {
		t.Fatalf("year: %s..%s", from, to)
	}
}

func TestNormReceiptURL(t *testing.T) {
	if v, err := normReceiptURL("  "); err != nil || v != nil {
		t.Fatal("empty must clear")
	}
	if v, err := normReceiptURL("https://bank.ru/check/1"); err != nil || *v != "https://bank.ru/check/1" {
		t.Fatal("valid url rejected")
	}
	for _, bad := range []string{"ftp://x", "javascript:alert(1)", "not a url", "https://"} {
		if _, err := normReceiptURL(bad); err == nil {
			t.Fatalf("%q must be rejected", bad)
		}
	}
}

func TestNormEmail(t *testing.T) {
	if _, err := normEmail("", true); err == nil {
		t.Fatal("required email")
	}
	if _, err := normEmail("Anna <a@b.ru>", false); err == nil {
		t.Fatal("display name must be rejected")
	}
	if v, err := normEmail("a@b.ru", true); err != nil || *v != "a@b.ru" {
		t.Fatal("valid email rejected")
	}
}

func TestNormTelegramUsername(t *testing.T) {
	for in, want := range map[string]string{"@zeronid": "zeronid", " zeronid ": "zeronid", "https://t.me/ZeroNid": "ZeroNid", "": ""} {
		if got, err := NormTelegramUsername(in); err != nil || got != want {
			t.Errorf("%q: got %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"ab", "1abcde", "a b c d e", "@@zero"} {
		if _, err := NormTelegramUsername(bad); err == nil {
			t.Errorf("%q: expected error", bad)
		}
	}
}
