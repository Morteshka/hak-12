// Package telegram — Telegram-бот: напоминания об оплате и кнопка «Оплачено».
package telegram

import (
	"context"
	"fmt"
	"html"
	"log"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"invoicekeeper/internal/models"
	"invoicekeeper/internal/service"
	"invoicekeeper/internal/store"
)

type Options struct {
	Token    string
	Env      service.ChannelEnv // TELEGRAM_CHAT_ID из .env (если пусто — чат привязывается командой /start)
	Interval time.Duration
	// RequireLink (по умолчанию) — «Оплачено» нажимают только пользователи сайта, подтвердившие свой @username.
	// false — также любой участник чата напоминаний.
	RequireLink bool
	PublicURL   string // адрес сайта для ссылок
	APIEndpoint string // пусто — api.telegram.org; иначе формат https://host/bot%s/%s
}

type Bot struct {
	api      *tgbotapi.BotAPI
	st       *store.Store
	payments *service.PaymentService
	receipts *service.ReceiptService
	opt      Options
	pending  *pendingState // кто какой чек сейчас присылает
	health   *health       // состояние для сайта: на связи ли бот и последняя ошибка
}

func New(opt Options, st *store.Store, payments *service.PaymentService, receipts *service.ReceiptService) (*Bot, error) {
	endpoint := tgbotapi.APIEndpoint
	if opt.APIEndpoint != "" {
		endpoint = opt.APIEndpoint
	}
	api, err := tgbotapi.NewBotAPIWithAPIEndpoint(opt.Token, endpoint)
	if err != nil {
		return nil, fmt.Errorf("telegram auth: %w", redactErr(err))
	}
	log.Printf("telegram: authorized as @%s", api.Self.UserName)
	// сайт показывает ссылку на бота в настройках
	if err := st.SetSetting(context.Background(), service.SettingTelegramBot, api.Self.UserName); err != nil {
		log.Printf("telegram: save bot name: %v", err)
	}
	// если у бота когда-то был задан webhook, Telegram не отдаёт сообщения через getUpdates
	// и бот «молчит» — снимаем его (накопившиеся сообщения сохраняются)
	if _, err := api.Request(tgbotapi.DeleteWebhookConfig{}); err != nil {
		log.Printf("telegram: delete webhook: %v", redactErr(err))
	}
	return &Bot{api: api, st: st, payments: payments, receipts: receipts, opt: opt, pending: newPendingState(), health: &health{st: st}}, nil
}

// chatID — чат для напоминаний: из .env или привязанный через /start (хранится в БД).
func (b *Bot) chatID(ctx context.Context) int64 {
	eff, _, err := service.EffectiveChannels(ctx, b.st, b.opt.Env)
	if err != nil {
		log.Printf("telegram: settings: %v", err)
		return b.opt.Env.TelegramChatID
	}
	return eff.TelegramChatID
}

// Run блокирует до отмены ctx: слушает обновления и периодически проверяет сроки.
func (b *Bot) Run(ctx context.Context) {
	b.setCommands()
	b.setMenuButton()
	go b.listen(ctx)
	go b.linkLoop(ctx)

	b.check(ctx)
	t := time.NewTicker(b.opt.Interval)
	defer t.Stop()
	// машина времени: дату сдвинули — напоминания в общий чат уходят через пару секунд, а не через минуту
	watch := time.NewTicker(3 * time.Second)
	defer watch.Stop()
	today := b.payments.Today()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			today = b.payments.Today()
			b.check(ctx)
		case <-watch.C:
			if d := b.payments.Today(); d.String() != today.String() {
				today = d
				b.check(ctx)
			}
		}
	}
}

func (b *Bot) check(ctx context.Context) {
	chat := b.chatID(ctx)
	if chat == 0 {
		return // чат ещё не задан: напишите боту /start
	}
	err := b.payments.ProcessReminders(ctx, func(v models.OccurrenceView) error { return b.sendReminder(chat, v) })
	if err != nil {
		log.Printf("telegram: reminders: %v", err)
	}
}

func (b *Bot) sendReminder(chat int64, v models.OccurrenceView) error {
	text := service.ReminderHTML(v, b.payments.Today(), false)
	if u := strings.TrimRight(b.opt.PublicURL, "/"); u != "" && !public(u) { // при публичном адресе ссылка — кнопкой
		text += "\n\n🔗 Оплатить и отметить: " + u + "/payments?open=" + strconv.Itoa(int(v.ID))
	}
	msg := tgbotapi.NewMessage(chat, text)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true
	row := tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("✅ Оплачено", "paid:"+strconv.Itoa(int(v.ID))))
	row = append(row, b.paymentLinkButton(v.ID)...)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(row)
	_, err := b.api.Send(msg)
	return err
}

func formatDate(d models.Date) string { return d.Format("02.01.2006") }

func (b *Bot) listen(ctx context.Context) {
	// Свой цикл вместо GetUpdatesChan: ошибки Telegram (неверный токен, второй запущенный бот
	// с тем же токеном) видны в логах и на сайте, а не теряются внутри библиотеки.
	offset := 0
	for ctx.Err() == nil {
		ups, err := b.api.GetUpdates(tgbotapi.UpdateConfig{Offset: offset, Timeout: 30, AllowedUpdates: []string{"message", "callback_query"}})
		if err != nil {
			b.health.report(ctx, DescribeError(err))
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
			}
			continue
		}
		b.health.report(ctx, "")
		for _, u := range ups {
			if u.UpdateID >= offset {
				offset = u.UpdateID + 1
			}
			b.handle(ctx, u)
		}
	}
}

// DescribeError — понятная причина, почему бот не получает сообщения.
func DescribeError(err error) string {
	t := Redact(err.Error())
	switch {
	case strings.Contains(t, "Conflict"):
		return "conflict: этот же токен бота запущен в другом месте (например, на вашем компьютере) — работать может только одна копия"
	case strings.Contains(t, "Unauthorized"), strings.Contains(t, "Not Found"):
		return "unauthorized: Telegram не принимает токен бота — проверьте TELEGRAM_BOT_TOKEN в .env"
	}
	return "network: нет связи с Telegram (" + t + ")"
}

func (b *Bot) handle(ctx context.Context, u tgbotapi.Update) {
	defer func() {
		if r := recover(); r != nil { // одна сломанная команда не должна останавливать бота
			log.Printf("telegram: panic while handling update %d: %v", u.UpdateID, r)
		}
	}()
	switch {
	case u.CallbackQuery != nil:
		b.onCallback(ctx, u.CallbackQuery)
	case u.Message != nil:
		if u.Message.From != nil {
			log.Printf("telegram: message from @%s (id %d) in chat %d: %q", u.Message.From.UserName, u.Message.From.ID, u.Message.Chat.ID, truncate(u.Message.Text, 40))
		}
		b.rememberContact(ctx, u.Message)
		switch {
		case u.Message.IsCommand():
			b.onCommand(ctx, u.Message)
		case len(u.Message.Photo) > 0 || u.Message.Document != nil:
			go b.onReceiptFile(ctx, u.Message) // распознавание занимает несколько секунд — не держим остальные сообщения
		case u.Message.Text != "":
			if !b.onMenuText(ctx, u.Message) { // кнопки меню внизу чата
				b.notUnderstood(u.Message)
			}
		}
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// linkedUser — пользователь сайта, подтвердивший этот Telegram.
func (b *Bot) linkedUser(ctx context.Context, from *tgbotapi.User) *service.Actor {
	if from == nil {
		return nil
	}
	u, err := b.st.UserByTelegramID(ctx, from.ID)
	if err != nil {
		return nil
	}
	return &service.Actor{UserID: &u.ID, Login: u.Login, Role: u.Role, UserAgent: "telegram"}
}

func (b *Bot) onCommand(ctx context.Context, m *tgbotapi.Message) {
	chat := b.chatID(ctx)
	me := b.linkedUser(ctx, m.From)
	switch m.Command() {
	case "start", "chatid":
		if m.Command() == "start" && b.onStartLink(ctx, m) {
			return
		}
		// общий чат назначает только администратор: /start в группе или в личке
		shared := ""
		if me != nil && me.Role == models.RoleAdmin {
			switch linked, err := service.LinkTelegramChat(ctx, b.st, b.opt.Env, m.Chat.ID); {
			case err != nil:
				log.Printf("telegram: link chat: %v", err)
			case linked:
				shared = "📣 Этот чат назначен общим: сюда приходят напоминания об общих платежах.\n\n"
			case chat == m.Chat.ID:
				shared = "📣 Это общий чат: сюда приходят напоминания об общих платежах.\n\n"
			}
		} else if chat == m.Chat.ID {
			shared = "📣 Это общий чат: сюда приходят напоминания об общих платежах.\n\n"
		}
		b.sendMenu(m.Chat.ID, m.Chat.IsPrivate(), b.startText(ctx, m, me, shared))
	default:
		if !b.onMenuText(ctx, m) {
			b.notUnderstood(m)
		}
	}
}

// notUnderstood — подсказка на произвольный текст. В группе молчим, чтобы не мешать переписке.
func (b *Bot) notUnderstood(m *tgbotapi.Message) {
	if !m.Chat.IsPrivate() {
		return
	}
	b.sendMenu(m.Chat.ID, true, "🤔 Не понял сообщение. Выберите действие в меню ниже или отправьте /help.\n\nЧек об оплате можно прислать сюда фото, скриншотом или PDF.")
}

// actorFor — кто нажал кнопку. Пользователь с подтверждённым Telegram действует от своего имени;
// без привязки — только участник чата напоминаний и только при TELEGRAM_REQUIRE_LINK=false.
func (b *Bot) actorFor(ctx context.Context, q *tgbotapi.CallbackQuery) (service.Actor, bool) {
	if u, err := b.st.UserByTelegramID(ctx, q.From.ID); err == nil {
		return service.Actor{UserID: &u.ID, Login: u.Login, Role: u.Role, UserAgent: "telegram"}, true
	}
	if b.opt.RequireLink || q.Message == nil || q.Message.Chat.ID != b.chatID(ctx) {
		return service.Actor{}, false
	}
	who := q.From.UserName
	if who == "" {
		who = strconv.FormatInt(q.From.ID, 10)
	}
	return service.Actor{Login: "telegram:" + who, UserAgent: "telegram"}, true
}

// onCallback — нажатие «✅ Оплачено» прямо в сообщении-напоминании.
func (b *Bot) onCallback(ctx context.Context, q *tgbotapi.CallbackQuery) {
	answer := func(text string) { _, _ = b.api.Request(tgbotapi.NewCallback(q.ID, text)) }

	switch {
	case strings.HasPrefix(q.Data, "tglink:"):
		b.onLinkAnswer(ctx, q)
		return
	case strings.HasPrefix(q.Data, "rcpt:"), strings.HasPrefix(q.Data, "rcpr:"):
		b.onReceiptDecision(ctx, q)
		return
	case strings.HasPrefix(q.Data, "rcf:"):
		b.onReceiptChoice(ctx, q)
		return
	case strings.HasPrefix(q.Data, "menu:"), strings.HasPrefix(q.Data, "ar:"):
		b.onMenuCallback(ctx, q)
		return
	}
	idStr, ok := strings.CutPrefix(q.Data, "paid:")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if !ok || err != nil || q.Message == nil {
		answer("Некорректная кнопка")
		return
	}
	actor, allowed := b.actorFor(ctx, q)
	if !allowed {
		answer("У вас нет доступа к этому платежу")
		return
	}
	if b.receipts != nil && b.receipts.RequiresReceipt(ctx, actor) {
		b.askForReceipt(ctx, q, actor, uint(id))
		return
	}
	v, already, err := b.payments.MarkPaidFromBot(ctx, actor, uint(id))
	if err != nil {
		log.Printf("telegram: mark paid #%d: %v", id, err)
		answer("Не удалось отметить оплату")
		return
	}
	if already {
		answer("Платёж уже был отмечен как оплаченный ✅")
	} else {
		answer("Платёж отмечен как оплаченный ✅")
	}

	by := ""
	if v.PaidBy != nil {
		by = " (" + html.EscapeString(*v.PaidBy) + ")"
	}
	// кнопка из списка («Ближайшие», «Отметить оплату»): список не затираем — убираем оплаченную строку
	if rest, isList := withoutButton(q.Message.ReplyMarkup, q.Data, q.Message.Text); isList {
		if !already {
			b.reply(q.Message.Chat.ID, fmt.Sprintf("✅ <b>%s</b> — %s\nОплата отмечена %s%s", html.EscapeString(v.Title),
				moneyRU(v.Amount.StringFixed(2), v.Currency), formatDate(*v.PaidAt), by))
		}
		b.send(tgbotapi.NewEditMessageReplyMarkup(q.Message.Chat.ID, q.Message.MessageID, rest))
		return
	}
	text := service.ReminderHTML(*v, b.payments.Today(), false) + fmt.Sprintf("\n\n✅ <b>Оплачено</b> %s%s", formatDate(*v.PaidAt), by)
	edit := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, text)
	edit.ParseMode = tgbotapi.ModeHTML
	empty := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}} // убираем кнопку
	edit.ReplyMarkup = &empty
	b.send(edit)
}

// withoutButton — клавиатура без нажатой кнопки «Оплачено». isList — сообщение со списком платежей
// (несколько кнопок «paid:»), а не одиночное напоминание.
func withoutButton(kb *tgbotapi.InlineKeyboardMarkup, data, text string) (tgbotapi.InlineKeyboardMarkup, bool) {
	out := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}}
	if kb == nil {
		return out, false
	}
	paidRows := 0
	for _, row := range kb.InlineKeyboard {
		keep := true
		for _, btn := range row {
			if btn.CallbackData != nil && strings.HasPrefix(*btn.CallbackData, "paid:") {
				paidRows++
				if *btn.CallbackData == data {
					keep = false
				}
			}
		}
		if keep {
			out.InlineKeyboard = append(out.InlineKeyboard, row)
		}
	}
	listText := strings.HasPrefix(text, listUpcomingTitle) || strings.HasPrefix(text, listOverdueTitle) || strings.HasPrefix(text, payChooserText)
	return out, paidRows > 1 || listText
}

func (b *Bot) reply(chatID int64, text string) {
	m := tgbotapi.NewMessage(chatID, text)
	m.ParseMode = tgbotapi.ModeHTML
	b.send(m)
}

// ---------- подтверждение привязки аккаунта ----------

// rememberContact запоминает личный чат по @имени: туда уйдёт запрос на подтверждение.
func (b *Bot) rememberContact(ctx context.Context, m *tgbotapi.Message) {
	if m.From == nil || m.From.IsBot || m.From.UserName == "" || !m.Chat.IsPrivate() {
		return
	}
	if err := b.st.UpsertTelegramContact(ctx, m.From.UserName, m.From.ID, m.Chat.ID); err != nil {
		log.Printf("telegram: remember contact: %v", err)
		return
	}
	if m.IsCommand() && m.Command() == "start" {
		if strings.HasPrefix(m.CommandArguments(), "L") {
			return // /start по ссылке с сайта подключает сразу (onStartLink) — отдельный запрос не нужен
		}
		// повторный /start: если прошлый запрос не дошёл (человек ещё не писал боту) — отправим заново
		if err := b.st.ResetTelegramPrompt(ctx, m.From.UserName); err != nil {
			log.Printf("telegram: reset prompt: %v", err)
		}
	}
	b.sendLinkRequests(ctx) // человек только что написал боту — сразу проверяем, не ждёт ли он подтверждения
}

// onStartLink — /start с кодом из кнопки «Открыть бота» на сайте: Telegram подключается сразу.
// Возвращает true, если код был (даже недействительный) и ответ уже отправлен.
func (b *Bot) onStartLink(ctx context.Context, m *tgbotapi.Message) bool {
	arg := strings.TrimSpace(m.CommandArguments())
	if !strings.HasPrefix(arg, "L") || m.From == nil {
		return false
	}
	if !m.Chat.IsPrivate() {
		b.reply(m.Chat.ID, "Чтобы подключить личные напоминания, откройте бота в личном чате по кнопке на сайте.")
		return true
	}
	id, ok := service.ParseTelegramLinkToken(b.opt.Env.LinkSecret, arg, time.Now())
	if !ok {
		b.reply(m.Chat.ID, "⚠️ Ссылка для подключения устарела или неверна. Откройте сайт → «Профиль и настройки» и нажмите «Открыть бота» ещё раз.")
		return true
	}
	u, err := b.st.UserByID(ctx, id)
	if err != nil {
		b.reply(m.Chat.ID, "⚠️ Аккаунт на сайте не найден. Откройте сайт и нажмите «Открыть бота» ещё раз.")
		return true
	}
	if err := b.st.LinkTelegramDirect(ctx, u.ID, m.From.ID, m.From.UserName); err != nil {
		log.Printf("telegram: link by start: %v", err)
		b.reply(m.Chat.ID, "Не удалось подключить, попробуйте ещё раз через минуту.")
		return true
	}
	text := fmt.Sprintf("✅ <b>Telegram подключён</b> к аккаунту <b>%s</b>.\nНапоминания о ваших платежах будут приходить сюда за неделю, за 3 дня и за сутки до срока, а кнопка «Оплачено» отмечает оплату от вашего имени.", html.EscapeString(u.Login))
	if u.Role == models.RoleAdmin {
		if linked, err := service.LinkTelegramChat(ctx, b.st, b.opt.Env, m.Chat.ID); err == nil && linked {
			text += "\n\n📣 Этот чат назначен общим: сюда будут приходить и напоминания об общих платежах."
		}
	}
	b.sendMenu(m.Chat.ID, true, text+"\n\nВыберите действие в меню 👇")
	return true
}

// linkLoop раз в несколько секунд ищет сохранённые на сайте @имена и пишет этим людям в Telegram.
func (b *Bot) linkLoop(ctx context.Context) {
	t := time.NewTicker(4 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			b.sendLinkRequests(ctx)
		}
	}
}

func (b *Bot) sendLinkRequests(ctx context.Context) {
	users, err := b.st.PendingTelegramLinks(ctx)
	if err != nil {
		log.Printf("telegram: pending links: %v", err)
		return
	}
	for _, u := range users {
		if u.TelegramUsername == nil {
			continue
		}
		c, err := b.st.TelegramContact(ctx, *u.TelegramUsername)
		if err != nil {
			continue // этот человек ещё не писал боту — запрос уйдёт сразу после его /start
		}
		text := fmt.Sprintf("🔐 <b>Подтверждение подключения</b>\n\nВы пытаетесь подключить этот Telegram (@%s) к аккаунту <b>%s</b> в Invoice Keeper.\n\nПосле подтверждения кнопка «✅ Оплачено» будет отмечать платежи от вашего имени.\n\n<b>Это вы?</b>",
			html.EscapeString(*u.TelegramUsername), html.EscapeString(u.Login))
		msg := tgbotapi.NewMessage(c.ChatID, text)
		msg.ParseMode = tgbotapi.ModeHTML
		id := strconv.Itoa(int(u.ID))
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Да, это я", "tglink:ok:"+id),
			tgbotapi.NewInlineKeyboardButtonData("❌ Это не я", "tglink:no:"+id),
		))
		if _, err := b.api.Send(msg); err != nil {
			log.Printf("telegram: link request for @%s: %v", *u.TelegramUsername, err)
		}
		// помечаем в любом случае, иначе при ошибке запрос будет лететь каждые 4 секунды;
		// повторить можно, пересохранив @имя на сайте
		_ = b.st.MarkTelegramPrompted(ctx, u.ID)
	}
}

// onLinkAnswer — нажатие «Да, это я» / «Это не я».
func (b *Bot) onLinkAnswer(ctx context.Context, q *tgbotapi.CallbackQuery) {
	answer := func(text string) { _, _ = b.api.Request(tgbotapi.NewCallback(q.ID, text)) }
	parts := strings.Split(q.Data, ":")
	id, err := strconv.ParseUint(parts[len(parts)-1], 10, 64)
	if len(parts) != 3 || err != nil || q.Message == nil {
		answer("Некорректная кнопка")
		return
	}
	u, err := b.st.UserByID(ctx, uint(id))
	// отвечать может только тот, чей @имя указан на сайте: пересланное сообщение чужим людям не поможет
	if err != nil || u.TelegramUsername == nil || u.TelegramVerifiedAt != nil ||
		!strings.EqualFold(*u.TelegramUsername, q.From.UserName) {
		answer("Этот запрос уже недействителен")
		return
	}
	var text, toast string
	if parts[1] == "ok" {
		if err := b.st.VerifyTelegramLink(ctx, u.ID, q.From.ID); err != nil {
			log.Printf("telegram: verify link: %v", err)
			answer("Не удалось подтвердить, попробуйте ещё раз")
			return
		}
		text = fmt.Sprintf("✅ <b>Telegram подключён</b> к аккаунту <b>%s</b>.\nНапоминания о ваших платежах будут приходить сюда за неделю, за 3 дня и за сутки до срока, а кнопка «Оплачено» отмечает оплату от вашего имени. Меню действий — /menu.\n\nПроверить можно на сайте: «Профиль и настройки» → «Прислать тестовое напоминание».", html.EscapeString(u.Login))
		// администратор без общего чата: его личный чат становится общим (напоминания об общих платежах)
		if u.Role == models.RoleAdmin {
			if linked, err := service.LinkTelegramChat(ctx, b.st, b.opt.Env, q.Message.Chat.ID); err == nil && linked {
				text += "\n\n📣 Этот чат назначен общим: сюда будут приходить и напоминания об общих платежах."
			}
		}
		toast = "Telegram подключён ✅"
	} else {
		if err := b.st.RejectTelegramLink(ctx, u.ID); err != nil {
			log.Printf("telegram: reject link: %v", err)
			answer("Не удалось отклонить, попробуйте ещё раз")
			return
		}
		text = "❌ Подключение отклонено. Аккаунт сайта не получил доступа к вашему Telegram."
		toast = "Подключение отклонено"
	}
	answer(toast)
	edit := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, text)
	edit.ParseMode = tgbotapi.ModeHTML
	empty := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}}
	edit.ReplyMarkup = &empty
	b.send(edit)
}
