// Команда bot — Telegram-бот напоминаний. Отдельный процесс/контейнер, общая БД и сервисный слой с API.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"invoicekeeper/internal/config"
	"invoicekeeper/internal/db"
	"invoicekeeper/internal/notify"
	"invoicekeeper/internal/service"
	"invoicekeeper/internal/store"
	"invoicekeeper/internal/telegram"
)

func main() {
	telegram.InstallLogRedaction()
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.TelegramToken == "" {
		log.Println("bot: TELEGRAM_BOT_TOKEN is empty — bot is disabled (idle)")
		<-ctx.Done()
		return
	}

	gdb, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	st := store.New(gdb) // миграции применяет API-сервер
	audit := service.NewAudit(st)
	payments := service.NewPaymentService(st, audit, cfg.Loc, cfg.ReminderDays)
	payments.EnableTimeTravel(cfg.TimeTravel) // та же «сегодняшняя» дата, что у API
	// оплата кнопкой в Telegram тоже уходит в webhook; чеки из бота — администратору на проверку
	hub := notify.New(st, audit, notify.Config{WebhookSecret: cfg.WebhookSecret, Env: cfg.ChannelEnv(), TelegramToken: cfg.TelegramToken, TelegramAPI: cfg.TelegramAPI, PublicURL: cfg.PublicURL, InternalURL: cfg.InternalURL})
	payments.SetEvents(hub)
	receipts := service.NewReceiptService(st, audit, payments, cfg.Analyzer())
	receipts.SetNotifier(hub)
	opt := telegram.Options{Token: cfg.TelegramToken, Env: cfg.ChannelEnv(), Interval: cfg.CheckInterval, RequireLink: cfg.TelegramRequireLink, PublicURL: cfg.PublicURL, APIEndpoint: cfg.TelegramAPI}

	for {
		bot, err := telegram.New(opt, st, payments, receipts)
		if err != nil {
			log.Printf("bot: %v (retry in 15s)", err)
			telegram.ReportHealth(ctx, st, telegram.DescribeError(err))
			select {
			case <-ctx.Done():
				return
			case <-time.After(15 * time.Second):
				continue
			}
		}
		if cfg.TelegramChatID == 0 {
			log.Println("bot: TELEGRAM_CHAT_ID not set — send /start to the bot: the chat will be linked automatically")
		}
		bot.Run(ctx)
		return
	}
}
