package telegram

import (
	"context"
	"log"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"invoicekeeper/internal/service"
	"invoicekeeper/internal/store"
)

// Состояние бота для сайта: когда бот последний раз был на связи с Telegram и последняя ошибка.
// По ним «Профиль и настройки» объясняет, почему бот молчит (не запущен, неверный токен,
// тот же токен запущен в другом месте), вместо безмолвного «ждём подтверждения».

type health struct {
	mu      sync.Mutex
	lastErr string
	saved   time.Time
	st      *store.Store
}

// report сохраняет состояние: ошибку — сразу, «всё хорошо» — не чаще раза в 20 секунд.
func (h *health) report(ctx context.Context, errText string) {
	h.mu.Lock()
	changed := errText != h.lastErr
	if !changed && time.Since(h.saved) < 20*time.Second {
		h.mu.Unlock()
		return
	}
	h.lastErr, h.saved = errText, time.Now()
	h.mu.Unlock()
	if changed && errText != "" {
		log.Printf("telegram: %s", errText)
	}
	ReportHealth(ctx, h.st, errText)
}

// ReportHealth записывает отметку «бот на связи» и текст ошибки (пустой — ошибок нет).
func ReportHealth(ctx context.Context, st *store.Store, errText string) {
	if err := st.SetSetting(ctx, service.SettingTelegramBotSeen, time.Now().UTC().Format(time.RFC3339)); err != nil {
		log.Printf("telegram: save health: %v", err)
		return
	}
	_ = st.SetSetting(ctx, service.SettingTelegramBotError, errText)
}

// send отправляет сообщение и пишет ошибку в лог (раньше ошибки отправки молча терялись).
func (b *Bot) send(c tgbotapi.Chattable) {
	if _, err := b.api.Send(c); err != nil {
		log.Printf("telegram: send: %v", redactErr(err))
	}
}
