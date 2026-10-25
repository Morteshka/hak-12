package service

import (
	"time"

	"invoicekeeper/internal/models"
)

// DueDateForIndex возвращает срок n-го периода (n=0 — первый) для ежемесячного платежа.
// day — «число месяца», заданное при создании. Если в месяце нет такого числа
// (например 31 в апреле или 30 в феврале) — берётся последний день месяца.
// Считаем от якоря (а не от предыдущей даты), поэтому 31 → 28 → 31 не «дрейфует».
func DueDateForIndex(first models.Date, day, n int) models.Date {
	return DueDateForStep(first, day, 1, n)
}

// DueDateForStep — то же для любой периодичности: step месяцев между оплатами
// (1 — ежемесячно, 3 — раз в квартал, 12 — раз в год).
func DueDateForStep(first models.Date, day, step, n int) models.Date {
	if step < 1 {
		step = 1
	}
	firstOfMonth := time.Date(first.Year(), first.Month()+time.Month(n*step), 1, 0, 0, 0, 0, time.UTC)
	last := firstOfMonth.AddDate(0, 1, -1).Day()
	if day > last {
		day = last
	}
	return models.NewDate(firstOfMonth.Year(), firstOfMonth.Month(), day)
}

// DueDateOf — срок n-го периода регулярного платежа с учётом его периодичности.
func DueDateOf(rp *models.RecurringPayment, n int) models.Date {
	if rp.Period == models.PeriodOnce {
		if n == 0 {
			return rp.FirstDueDate
		}
		return models.NewDate(9999, 12, 31) // у разового платежа второго периода нет
	}
	return DueDateForStep(rp.FirstDueDate, rp.DayOfMonth, models.PeriodMonths(rp.Period), n)
}
