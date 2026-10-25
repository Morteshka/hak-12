// Команда server — REST API (+ фоновое создание периодов платежей).
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"invoicekeeper/internal/config"
	"invoicekeeper/internal/db"
	"invoicekeeper/internal/handler"
	"invoicekeeper/internal/notify"
	"invoicekeeper/internal/service"
	"invoicekeeper/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if err := cfg.ValidateServer(); err != nil {
		log.Fatalf("config: %v", err)
	}
	for _, w := range cfg.SecurityWarnings() {
		log.Printf("ВНИМАНИЕ (безопасность): %s", w)
	}

	gdb, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Migrate(gdb); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	st := store.New(gdb)
	audit := service.NewAudit(st)
	auth := service.NewAuthService(st, audit, cfg.JWTSecret, cfg.AccessTTL, cfg.RefreshTTL)
	auth.SetTelegramToken(cfg.TelegramToken) // вход из Telegram Mini App
	users := service.NewUserService(st, audit)
	payments := service.NewPaymentService(st, audit, cfg.Loc, cfg.ReminderDays)
	payments.EnableTimeTravel(cfg.TimeTravel)
	hub := notify.New(st, audit, notify.Config{
		WebhookSecret: cfg.WebhookSecret, PublicURL: cfg.PublicURL, Env: cfg.ChannelEnv(), TelegramToken: cfg.TelegramToken, TelegramAPI: cfg.TelegramAPI,
		InternalURL: cfg.InternalURL,
	})
	payments.SetEvents(hub) // webhook при создании/оплате
	auth.SetMailer(hub)     // код подтверждения почты при регистрации
	auth.SetEmailVerify(cfg.EmailVerify)
	hub.UsePayments(payments)
	catalog := service.NewCatalogService(st, audit, payments)
	// новый платёж или подписка — рассылка напоминаний запускается сразу, не дожидаясь минутного тика
	wake := make(chan struct{}, 1)
	payments.OnChange(func() {
		select {
		case wake <- struct{}{}:
		default:
		}
	})
	budgets := service.NewBudgetService(st, audit, payments)
	settings := service.NewSettingsService(st, audit, cfg.ChannelEnv())
	receipts := service.NewReceiptService(st, audit, payments, cfg.Analyzer())
	receipts.SetNotifier(hub) // администратору — чек на проверку, пользователю — решение
	settings.UseReceipts(receipts)
	analyzer := cfg.Analyzer()
	imports := service.NewImportService(payments, audit, analyzer.PDFText)
	log.Printf("receipts: recognition %+v", receipts.Capabilities())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := auth.EnsureAdmin(ctx, cfg.AdminLogin, cfg.AdminPassword, cfg.AdminEmail); err != nil {
		log.Fatalf("admin bootstrap: %v", err)
	}
	if cfg.SeedDemo {
		if err := service.SeedDemo(ctx, st, payments, catalog, service.Actor{Login: cfg.AdminLogin, Role: "admin"}); err != nil {
			log.Fatalf("seed: %v", err)
		}
	}

	// Фоновая задача: раз в минуту создаёт периоды вперёд (следующий месяц виден заранее)
	// и рассылает напоминания по email / webhook. Telegram — в отдельном процессе bot.
	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for {
			if err := payments.EnsureAll(ctx); err != nil {
				log.Printf("scheduler: %v", err)
			}
			hub.RunReminders(ctx, payments)
			_ = st.PruneNotifications(ctx, 60*24*time.Hour) // прочитанные уведомления старше 60 дней
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			case <-wake:
			}
		}
	}()

	srv := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: handler.NewRouter(handler.Deps{
			Auth: auth, Users: users, Payments: payments, Audit: audit, Budgets: budgets, Settings: settings, Catalog: catalog, Receipts: receipts, Imports: imports, Notifier: hub,
			Notes: service.NewNotificationService(st),
			Loc:   cfg.Loc, Origins: cfg.AllowedOrigins, AllowRegistration: cfg.AllowRegistration,
			WebhookSecret: cfg.WebhookSecret,
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		log.Printf("api: listening on :%s (tz=%s)", cfg.Port, cfg.Timezone)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
}
