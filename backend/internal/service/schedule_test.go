package service

import (
	"testing"

	"invoicekeeper/internal/models"
)

func TestDueDateForIndex_ClampsToLastDayOfMonth(t *testing.T) {
	first, _ := models.ParseDate("2026-01-31")
	cases := map[int]string{
		0:  "2026-01-31",
		1:  "2026-02-28", // нет 31 февраля → последний день
		2:  "2026-03-31", // и не «дрейфует» к 28
		3:  "2026-04-30",
		11: "2026-12-31",
		13: "2027-02-28",
		25: "2028-02-29", // високосный год
	}
	for n, want := range cases {
		if got := DueDateForIndex(first, 31, n).String(); got != want {
			t.Errorf("n=%d: got %s, want %s", n, got, want)
		}
	}
}

func TestDaysUntil(t *testing.T) {
	a, _ := models.ParseDate("2026-10-23")
	b, _ := models.ParseDate("2026-10-26")
	if a.DaysUntil(b) != 3 || b.DaysUntil(a) != -3 {
		t.Fatal("DaysUntil is wrong")
	}
}
