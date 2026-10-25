package service

import (
	"testing"
	"time"

	"invoicekeeper/internal/models"
)

func TestDueDateForStep(t *testing.T) {
	first := models.NewDate(2026, time.January, 31)
	cases := []struct {
		step, n int
		want    string
	}{
		{3, 1, "2026-04-30"}, // квартал: 31 апреля нет → последний день
		{3, 2, "2026-07-31"},
		{12, 1, "2027-01-31"},
		{1, 1, "2026-02-28"},
	}
	for _, c := range cases {
		if got := DueDateForStep(first, 31, c.step, c.n).String(); got != c.want {
			t.Errorf("step=%d n=%d: got %s want %s", c.step, c.n, got, c.want)
		}
	}
}

func TestFirstDue(t *testing.T) {
	today := models.NewDate(2026, time.October, 24)
	if d, tr := FirstDue(today, 0); d.String() != "2026-10-24" || tr != nil {
		t.Fatalf("no trial: %s %v", d, tr)
	}
	if d, tr := FirstDue(today, 14); d.String() != "2026-11-07" || tr == nil || tr.String() != "2026-11-07" {
		t.Fatalf("trial 14: %s %v", d, tr)
	}
}

func TestDaysLeftRU(t *testing.T) {
	today := models.NewDate(2026, time.October, 24)
	cases := map[int]string{
		0: "Срок оплаты — сегодня", 1: "Остался 1 день", 3: "Осталось 3 дня", 5: "Осталось 5 дней",
		21: "Остался 21 день", -1: "Просрочено на 1 день", -2: "Просрочено на 2 дня", -11: "Просрочено на 11 дней",
	}
	for n, want := range cases {
		v := models.OccurrenceView{}
		v.DueDate = today.AddDays(n)
		if got := DaysLeftRU(v, today); got != want {
			t.Errorf("%d: got %q want %q", n, got, want)
		}
	}
}
