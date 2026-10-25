package service

import (
	"context"
	"errors"
	"log"

	"github.com/shopspring/decimal"
	"golang.org/x/crypto/bcrypt"

	"invoicekeeper/internal/models"
	"invoicekeeper/internal/store"
)

// SeedDemo наполняет пустую БД тестовыми данными (SEED_DEMO=true):
// общие платежи — аренда в рублях, сервер в долларах, подписка в евро + обычный пользователь user/user123;
// каталог услуг — несколько услуг с пробным периодом и без, user подключён к одной из них.
// Даты считаются от «сегодня», чтобы сразу показать напоминание и просрочку.
func SeedDemo(ctx context.Context, st *store.Store, pay *PaymentService, cat *CatalogService, admin Actor) error {
	if err := seedPayments(ctx, st, pay, admin); err != nil {
		return err
	}
	return seedCatalog(ctx, st, cat, admin)
}

func seedPayments(ctx context.Context, st *store.Store, pay *PaymentService, admin Actor) error {
	n, err := st.CountRecurring(ctx)
	if err != nil || n > 0 {
		return err
	}
	if _, err := st.UserByLogin(ctx, "user"); errors.Is(err, store.ErrNotFound) {
		hash, _ := bcrypt.GenerateFromPassword([]byte("user123"), bcrypt.DefaultCost)
		if err := st.CreateUser(ctx, &models.User{Login: "user", PasswordHash: string(hash), Role: models.RoleUser}); err != nil {
			return err
		}
	}
	today := pay.Today()
	demo := []CreateRecurringInput{
		{Title: "Аренда офиса", Amount: decimal.RequireFromString("85000.00"), Currency: "RUB", DueDate: today.AddDays(2)},
		{Title: "Сервер (VPS)", Amount: decimal.RequireFromString("25.00"), Currency: "USD", DueDate: today.AddDays(10)},
		{Title: "Подписка на сервис", Amount: decimal.RequireFromString("12.99"), Currency: "EUR", DueDate: today.AddDays(-3)},
	}
	for _, d := range demo {
		if _, err := pay.CreateRecurring(ctx, admin, d); err != nil {
			return err
		}
	}
	log.Println("seed: demo data created (user / user123)")
	return nil
}

// seedCatalog — демо-каталог (создаётся один раз, если услуг ещё не было).
func seedCatalog(ctx context.Context, st *store.Store, cat *CatalogService, admin Actor) error {
	n, err := st.CountServices(ctx)
	if err != nil || n > 0 {
		return err
	}
	str := func(v string) *string { return &v }
	num := func(v string) *decimal.Decimal { d := decimal.RequireFromString(v); return &d }
	days := func(v int) *int { return &v }
	demo := []ServiceInput{
		{Title: str("VPS-сервер 2 vCPU / 4 ГБ"), Description: str("Виртуальный сервер для сайта и Telegram-бота. Первый месяц оплачивается при подключении."),
			Amount: num("15.00"), Currency: str("USD"), Period: str(models.PeriodMonthly), TrialDays: days(0)},
		{Title: str("Облачное хранилище 1 ТБ"), Description: str("Резервные копии файлов и фото. 14 дней бесплатно, затем помесячно."),
			Amount: num("299.00"), Currency: str("RUB"), Period: str(models.PeriodMonthly), TrialDays: days(14)},
		{Title: str("Онлайн-кинотеатр"), Description: str("Фильмы и сериалы. Пробный период 3 дня — потом обычная ежемесячная оплата."),
			Amount: num("399.00"), Currency: str("RUB"), Period: str(models.PeriodMonthly), TrialDays: days(3)},
		{Title: str("Дизайн-редактор Pro"), Description: str("Макеты, презентации и совместная работа. 7 дней бесплатно."),
			Amount: num("12.99"), Currency: str("EUR"), Period: str(models.PeriodMonthly), TrialDays: days(7)},
		{Title: str("Антивирус для офиса"), Description: str("Защита до 5 устройств. Оплата раз в квартал."),
			Amount: num("29.00"), Currency: str("EUR"), Period: str(models.PeriodQuarterly), TrialDays: days(0)},
		{Title: str("Домен .ru"), Description: str("Продление доменного имени раз в год."),
			Amount: num("990.00"), Currency: str("RUB"), Period: str(models.PeriodYearly), TrialDays: days(0)},
	}
	var trial3 uint
	for _, in := range demo {
		sv, err := cat.Create(ctx, admin, in)
		if err != nil {
			return err
		}
		if sv.TrialDays == 3 {
			trial3 = sv.ID
		}
	}
	// демо-пользователь подключил онлайн-кинотеатр: пробный период 3 дня → напоминание «Осталось 3 дня»
	if u, err := st.UserByLogin(ctx, "user"); err == nil && trial3 != 0 {
		if _, err := cat.Subscribe(ctx, Actor{UserID: &u.ID, Login: u.Login, Role: u.Role, UserAgent: "seed"}, trial3); err != nil {
			return err
		}
	}
	log.Println("seed: demo catalog created")
	return nil
}
