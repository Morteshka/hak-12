package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/shopspring/decimal"

	"invoicekeeper/internal/models"
	"invoicekeeper/internal/service"
)

// Пункты меню: одна и та же команда доступна кнопкой внизу чата, inline-кнопкой и командой /…
const (
	btnUpcoming = "📅 Ближайшие платежи"
	btnOverdue  = "⚠️ Просроченные"
	btnPay      = "✅ Отметить оплату"
	btnReceipt  = "🧾 Прислать чек"
	btnSummary  = "📊 Итоги месяца"
	btnRenew    = "🔄 Автопродление"
	btnSite     = "🌐 Открыть сайт"
	btnHelp     = "❓ Помощь"
)

var menuActions = map[string]string{
	btnUpcoming: "upcoming", btnOverdue: "overdue", btnPay: "pay", btnReceipt: "receipt",
	btnSummary: "summary", btnRenew: "renew", btnSite: "site", btnHelp: "help",
	// команды
	"/upcoming": "upcoming", "/overdue": "overdue", "/pay": "pay", "/receipt": "receipt",
	"/summary": "summary", "/renew": "renew", "/site": "site", "/help": "help", "/menu": "menu",
}

// mainKeyboard — постоянное меню внизу личного чата.
func mainKeyboard() tgbotapi.ReplyKeyboardMarkup {
	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton(btnUpcoming), tgbotapi.NewKeyboardButton(btnOverdue)),
		tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton(btnPay), tgbotapi.NewKeyboardButton(btnReceipt)),
		tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton(btnSummary), tgbotapi.NewKeyboardButton(btnRenew)),
		tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton(btnSite), tgbotapi.NewKeyboardButton(btnHelp)),
	)
	kb.ResizeKeyboard = true
	kb.InputFieldPlaceholder = "Выберите действие"
	return kb
}

// inlineMenu — то же меню кнопками под сообщением (работает и в группе).
func inlineMenu() tgbotapi.InlineKeyboardMarkup {
	b := func(text, action string) tgbotapi.InlineKeyboardButton {
		return tgbotapi.NewInlineKeyboardButtonData(text, "menu:"+action)
	}
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(b(btnUpcoming, "upcoming"), b(btnOverdue, "overdue")),
		tgbotapi.NewInlineKeyboardRow(b(btnPay, "pay"), b(btnReceipt, "receipt")),
		tgbotapi.NewInlineKeyboardRow(b(btnSummary, "summary"), b(btnRenew, "renew")),
		tgbotapi.NewInlineKeyboardRow(b(btnSite, "site"), b(btnHelp, "help")),
	)
}

// setCommands — список команд в меню Telegram (кнопка «/» слева от поля ввода).
func (b *Bot) setCommands() {
	cmds := tgbotapi.NewSetMyCommands(
		tgbotapi.BotCommand{Command: "start", Description: "Начало работы и меню"},
		tgbotapi.BotCommand{Command: "menu", Description: "Меню действий"},
		tgbotapi.BotCommand{Command: "upcoming", Description: "Ближайшие платежи"},
		tgbotapi.BotCommand{Command: "overdue", Description: "Просроченные платежи"},
		tgbotapi.BotCommand{Command: "pay", Description: "Отметить оплату"},
		tgbotapi.BotCommand{Command: "receipt", Description: "Прислать чек"},
		tgbotapi.BotCommand{Command: "summary", Description: "Итоги месяца по валютам"},
		tgbotapi.BotCommand{Command: "renew", Description: "Автопродление подписок"},
		tgbotapi.BotCommand{Command: "site", Description: "Открыть сайт"},
		tgbotapi.BotCommand{Command: "help", Description: "Помощь"},
	)
	if _, err := b.api.Request(cmds); err != nil {
		log.Printf("telegram: set commands: %v", err)
	}
}

// startText — приветствие по /start.
func (b *Bot) startText(ctx context.Context, m *tgbotapi.Message, me *service.Actor, sharedNote string) string {
	var sb strings.Builder
	name := "Invoice Keeper"
	if b.api.Self.UserName != "" {
		name = "@" + b.api.Self.UserName
	}
	fmt.Fprintf(&sb, "👋 <b>Привет! Я %s</b> — слежу за оплатой подписок и регулярных платежей.\n\n", html.EscapeString(name))
	sb.WriteString("Что я умею:\n")
	sb.WriteString("• напоминаю об оплате за неделю, за 3 дня и за сутки до срока, а если срок прошёл — сообщаю о просрочке;\n")
	sb.WriteString("• в каждом напоминании — кнопка «Оплачено» и ссылка на платёж на сайте;\n")
	sb.WriteString("• присылаю подтверждение, когда оплата проведена, со ссылкой на чек;\n")
	sb.WriteString("• показываю ближайшие и просроченные платежи и итоги месяца по каждой валюте.\n\n")
	switch {
	case me != nil:
		fmt.Fprintf(&sb, "✅ Вы вошли как <b>%s</b>.", html.EscapeString(me.Login))
		if b.receipts != nil && b.receipts.RequiresReceipt(ctx, *me) {
			sb.WriteString(" Оплату подтверждают чеком: пришлите сюда фото, скриншот или PDF — я проверю и отмечу оплату.")
		}
		sb.WriteString("\n\n")
	case m.From != nil && m.From.UserName != "":
		fmt.Fprintf(&sb, "🔗 Чтобы получать личные напоминания, откройте на сайте «Профиль и настройки» и нажмите «Открыть бота» — Telegram подключится сразу. Или укажите там <b>@%s</b> и нажмите «Да, это я» в запросе, который я пришлю сюда.\n\n", html.EscapeString(m.From.UserName))
	default:
		sb.WriteString("🔗 Чтобы получать личные напоминания, откройте на сайте «Профиль и настройки» и нажмите «Открыть бота» — Telegram подключится сразу.\n\n")
	}
	sb.WriteString(sharedNote)
	if b.webAppURL() != "" && m.Chat.IsPrivate() {
		sb.WriteString("📱 Кнопка <b>«Приложение»</b> слева от поля ввода открывает Invoice Keeper прямо в Telegram — без логина и пароля.\n\n")
	}
	if me != nil && me.Role == models.RoleAdmin {
		fmt.Fprintf(&sb, "ID этого чата: <code>%d</code>\n\n", m.Chat.ID)
	}
	sb.WriteString("Выберите действие в меню 👇")
	return sb.String()
}

// sendMenu — сообщение с меню: в личке — постоянные кнопки внизу, в группе — кнопки под сообщением.
func (b *Bot) sendMenu(chatID int64, private bool, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true
	if private {
		msg.ReplyMarkup = mainKeyboard()
	} else {
		msg.ReplyMarkup = inlineMenu()
	}
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("telegram: menu: %v", err)
	}
}

// onMenuText — нажата кнопка меню (или команда /upcoming, /pay …). false — текст не из меню.
func (b *Bot) onMenuText(ctx context.Context, m *tgbotapi.Message) bool {
	key := strings.TrimSpace(m.Text)
	if m.IsCommand() {
		key = "/" + m.Command()
	}
	action, ok := menuActions[key]
	if !ok {
		return false
	}
	b.runMenu(ctx, m.Chat.ID, m.Chat.IsPrivate(), m.From, action)
	return true
}

// onMenuCallback — inline-кнопки меню и автопродления.
func (b *Bot) onMenuCallback(ctx context.Context, q *tgbotapi.CallbackQuery) {
	_, _ = b.api.Request(tgbotapi.NewCallback(q.ID, ""))
	if q.Message == nil {
		return
	}
	if rest, ok := strings.CutPrefix(q.Data, "ar:"); ok {
		b.onRenewToggle(ctx, q, rest)
		return
	}
	action := strings.TrimPrefix(q.Data, "menu:")
	b.runMenu(ctx, q.Message.Chat.ID, q.Message.Chat.IsPrivate(), q.From, action)
}

func (b *Bot) runMenu(ctx context.Context, chatID int64, private bool, from *tgbotapi.User, action string) {
	me := b.linkedUser(ctx, from)
	// без подтверждённого Telegram — только общие платежи и только в общем чате
	actor := service.Actor{UserAgent: "telegram"}
	switch {
	case me != nil:
		actor = *me
	case chatID == b.chatID(ctx):
	case action == "site" || action == "help" || action == "menu":
	default:
		b.reply(chatID, "🔗 Сначала подключите Telegram: на сайте «Профиль и настройки» → «Ваш Telegram», затем подтвердите запрос здесь. После этого я покажу ваши платежи.")
		return
	}
	switch action {
	case "menu":
		b.sendMenu(chatID, private, "Выберите действие 👇")
	case "upcoming":
		b.listPayments(ctx, chatID, actor, me, false)
	case "overdue":
		b.listPayments(ctx, chatID, actor, me, true)
	case "pay":
		b.payChooser(ctx, chatID, actor)
	case "receipt":
		if me == nil {
			b.reply(chatID, "Чтобы прислать чек, подключите Telegram на сайте («Профиль и настройки»).")
			return
		}
		b.reply(chatID, "🧾 Пришлите сюда фото, скриншот или PDF чека — я распознаю его, спрошу, к какому платежу он относится, и проверю сумму, дату и номер операции.")
	case "summary":
		b.monthSummary(ctx, chatID, actor)
	case "renew":
		if me == nil {
			b.reply(chatID, "Автопродлением управляет владелец подписки — подключите Telegram на сайте.")
			return
		}
		b.renewList(ctx, chatID, *me, 0)
	case "site":
		b.siteLink(chatID)
	case "help":
		b.reply(chatID, helpText)
	}
}

// Заголовки сообщений-списков: по ним кнопка «Оплачено» понимает, что это список, а не одно напоминание.
const (
	listUpcomingTitle = "📅 Ближайшие платежи"
	listOverdueTitle  = "⚠️ Просроченные платежи"
	payChooserText    = "Какой платёж оплачен?"
)

const helpText = "<b>Как пользоваться</b>\n\n" +
	"📅 <b>Ближайшие платежи</b> — неоплаченные периоды по сроку.\n" +
	"⚠️ <b>Просроченные</b> — срок прошёл, оплаты нет.\n" +
	"✅ <b>Отметить оплату</b> — выберите платёж (если включена проверка чеков, я попрошу чек).\n" +
	"🧾 <b>Прислать чек</b> — фото, скриншот или PDF из банка.\n" +
	"📊 <b>Итоги месяца</b> — оплачено, просрочено и предстоит, отдельно по рублям, долларам и евро.\n" +
	"🔄 <b>Автопродление</b> — включить или выключить продление ваших подписок.\n" +
	"🌐 <b>Открыть сайт</b> — там можно исправить дату оплаты, посмотреть историю и чеки.\n\n" +
	"Напоминания приходят за неделю, за 3 дня и за сутки до срока, а также если оплата просрочена."

func (b *Bot) listPayments(ctx context.Context, chatID int64, actor service.Actor, me *service.Actor, overdueOnly bool) {
	list, err := b.payments.Upcoming(ctx, actor, "", 200)
	if err != nil {
		log.Printf("telegram: upcoming: %v", err)
		b.reply(chatID, "Не удалось загрузить платежи, попробуйте позже.")
		return
	}
	var rows []models.OccurrenceView
	for _, v := range list {
		if overdueOnly == (v.Status == models.StatusOverdue) {
			rows = append(rows, v)
		}
	}
	if len(rows) == 0 {
		if overdueOnly {
			b.reply(chatID, "Просроченных платежей нет 🎉")
		} else {
			b.reply(chatID, "Ближайших неоплаченных платежей нет 🎉")
		}
		return
	}
	more := 0
	if len(rows) > 10 {
		more, rows = len(rows)-10, rows[:10]
	}
	var sb strings.Builder
	if overdueOnly {
		sb.WriteString("⚠️ <b>" + strings.TrimPrefix(listOverdueTitle, "⚠️ ") + "</b>\n\n")
	} else {
		sb.WriteString("📅 <b>" + strings.TrimPrefix(listUpcomingTitle, "📅 ") + "</b>\n\n")
	}
	today := b.payments.Today()
	var kb [][]tgbotapi.InlineKeyboardButton
	for _, v := range rows {
		who := ""
		if v.OwnerLogin != nil && (me == nil || me.Role == models.RoleAdmin) {
			who = " · " + html.EscapeString(*v.OwnerLogin)
		}
		review := ""
		if v.ReceiptStatus != nil && *v.ReceiptStatus == models.ReceiptPending {
			review = " · 🧾 чек на проверке"
		}
		fmt.Fprintf(&sb, "<b>%s</b> — %s\nдо %s · %s%s%s\n\n", html.EscapeString(v.Title), moneyRU(v.Amount.StringFixed(2), v.Currency),
			formatDate(v.DueDate), service.DaysLeftRU(v, today), who, review)
		kb = append(kb, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(
			"✅ "+short(v.Title, 28)+" · "+formatDate(v.DueDate), "paid:"+strconv.Itoa(int(v.ID)))))
	}
	if more > 0 {
		fmt.Fprintf(&sb, "…и ещё %d — все платежи на сайте.", more)
	}
	msg := tgbotapi.NewMessage(chatID, strings.TrimSpace(sb.String()))
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(kb...)
	b.send(msg)
}

// payChooser — выбрать платёж для отметки оплаты.
func (b *Bot) payChooser(ctx context.Context, chatID int64, actor service.Actor) {
	list, err := b.payments.Upcoming(ctx, actor, "", 8)
	if err != nil || len(list) == 0 {
		b.reply(chatID, "Неоплаченных платежей нет 🎉")
		return
	}
	var kb [][]tgbotapi.InlineKeyboardButton
	for _, v := range list {
		mark := "•"
		if v.Status == models.StatusOverdue {
			mark = "⚠️"
		}
		kb = append(kb, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%s %s · %s · %s", mark, short(v.Title, 22), moneyRU(v.Amount.StringFixed(2), v.Currency), formatDate(v.DueDate)),
			"paid:"+strconv.Itoa(int(v.ID)))))
	}
	msg := tgbotapi.NewMessage(chatID, payChooserText+" Дата оплаты — сегодня; исправить её можно на сайте.")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(kb...)
	b.send(msg)
}

// monthSummary — итоги текущего и следующего месяца по каждой валюте (без конвертации).
func (b *Bot) monthSummary(ctx context.Context, chatID int64, actor service.Actor) {
	rows, err := b.payments.MonthlySummary(ctx, actor, "", 1, 1)
	if err != nil {
		b.reply(chatID, "Не удалось посчитать итоги, попробуйте позже.")
		return
	}
	today := b.payments.Today()
	next := models.DateOf(models.NewDate(today.Year(), today.Month(), 1).Time.AddDate(0, 1, 0))
	names := map[string]string{"RUB": "₽ Рубли", "USD": "$ Доллары", "EUR": "€ Евро"}
	var sb strings.Builder
	write := func(month models.Date, title string) {
		fmt.Fprintf(&sb, "<b>%s</b>\n", title)
		found := false
		for _, r := range rows {
			if r.Month != month.Format("2006-01") || r.Count == 0 {
				continue
			}
			found = true
			m := func(d decimal.Decimal) string { return moneyRU(d.StringFixed(2), r.Currency) }
			fmt.Fprintf(&sb, "\n%s — <b>%s</b>\n", names[r.Currency], m(r.Total))
			parts := []string{fmt.Sprintf("✅ оплачено %s", m(r.Paid))}
			if r.Overdue.IsPositive() {
				parts = append(parts, fmt.Sprintf("⚠️ просрочено %s", m(r.Overdue)))
			}
			if r.Upcoming.IsPositive() {
				parts = append(parts, fmt.Sprintf("⏳ предстоит %s", m(r.Upcoming)))
			}
			sb.WriteString(strings.Join(parts, " · ") + "\n")
		}
		if !found {
			sb.WriteString("платежей нет\n")
		}
		sb.WriteString("\n")
	}
	write(today, "📊 "+capitalize(monthRU(today)))
	write(next, "🗓 "+capitalize(monthRU(next))+" — прогноз")
	sb.WriteString("<i>Суммы в разных валютах считаются отдельно, без конвертации.</i>")
	b.reply(chatID, sb.String())
}

func capitalize(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return strings.ToUpper(string(r[:1])) + string(r[1:])
}

// renewList — подписки с переключателем автопродления. edit — id сообщения для обновления (0 — новое).
func (b *Bot) renewList(ctx context.Context, chatID int64, me service.Actor, edit int) {
	list, err := b.payments.ListRecurring(ctx, me, "")
	if err != nil {
		b.reply(chatID, "Не удалось загрузить подписки.")
		return
	}
	var kb [][]tgbotapi.InlineKeyboardButton
	var sb strings.Builder
	sb.WriteString("🔄 <b>Автопродление</b>\nНажмите на платёж, чтобы включить или выключить продление. Выключенный платёж не создаёт новые периоды — текущий остаётся к оплате.\n")
	n := 0
	for _, r := range list {
		mine := r.UserID != nil && me.UserID != nil && *r.UserID == *me.UserID
		if r.Period == models.PeriodOnce || !(mine || (r.UserID == nil && me.Role == models.RoleAdmin)) {
			continue
		}
		n++
		state, next := "✅ вкл", "0"
		if !r.AutoRenew {
			state, next = "⏸ выкл", "1"
		}
		kb = append(kb, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%s · %s", short(r.Title, 30), state), "ar:"+strconv.Itoa(int(r.ID))+":"+next)))
	}
	if n == 0 {
		b.reply(chatID, "У вас нет подписок с продлением. Подключить услугу можно на сайте в разделе «Услуги».")
		return
	}
	if edit != 0 {
		e := tgbotapi.NewEditMessageTextAndMarkup(chatID, edit, sb.String(), tgbotapi.NewInlineKeyboardMarkup(kb...))
		e.ParseMode = tgbotapi.ModeHTML
		b.send(e)
		return
	}
	msg := tgbotapi.NewMessage(chatID, sb.String())
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(kb...)
	b.send(msg)
}

func (b *Bot) onRenewToggle(ctx context.Context, q *tgbotapi.CallbackQuery, rest string) {
	me := b.linkedUser(ctx, q.From)
	parts := strings.Split(rest, ":")
	if me == nil || len(parts) != 2 {
		return
	}
	id, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return
	}
	on := parts[1] == "1"
	if _, err := b.payments.UpdateRecurring(ctx, *me, uint(id), service.UpdateRecurringInput{AutoRenew: &on}); err != nil {
		log.Printf("telegram: auto renew #%d: %v", id, err)
		_, _ = b.api.Request(tgbotapi.NewCallback(q.ID, "Не удалось изменить"))
		return
	}
	b.renewList(ctx, q.Message.Chat.ID, *me, q.Message.MessageID)
}

func (b *Bot) siteLink(chatID int64) {
	u := strings.TrimRight(b.opt.PublicURL, "/")
	if u == "" {
		b.reply(chatID, "Адрес сайта не настроен (PUBLIC_URL).")
		return
	}
	if app := b.webAppURL(); app != "" && chatID > 0 { // личный чат: приложение открывается прямо в Telegram
		b.sendRaw(chatID, "📱 <b>Invoice Keeper в Telegram</b>\nВсе платежи, чеки, бюджет и импорт выписки — прямо здесь, без входа: Telegram уже подтвердил, что это вы.\n\nТа же кнопка «Приложение» есть слева от поля ввода.",
			map[string]any{"inline_keyboard": [][]map[string]any{
				{{"text": "📱 Открыть приложение", "web_app": map[string]string{"url": app + "/"}}},
				{{"text": "🌐 Открыть в браузере", "url": u + "/payments"}},
			}})
		return
	}
	msg := tgbotapi.NewMessage(chatID, "🌐 Сайт Invoice Keeper: "+u+"\nТам — все платежи, история, чеки, итоги по месяцам и настройки напоминаний.")
	if public(u) {
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonURL("Открыть сайт", u+"/payments")))
	}
	b.send(msg)
}

// webAppURL — адрес Mini App: Telegram открывает приложения только по HTTPS и не с localhost.
func (b *Bot) webAppURL() string {
	u := strings.TrimRight(b.opt.PublicURL, "/")
	if strings.HasPrefix(u, "https://") && public(u) {
		return u
	}
	return ""
}

// setMenuButton — кнопка «Приложение» слева от поля ввода во всех личных чатах (или список команд без HTTPS).
func (b *Bot) setMenuButton() {
	btn := map[string]any{"type": "commands"}
	if u := b.webAppURL(); u != "" {
		btn = map[string]any{"type": "web_app", "text": "Приложение", "web_app": map[string]string{"url": u + "/"}}
	}
	j, _ := json.Marshal(btn)
	if _, err := b.api.MakeRequest("setChatMenuButton", tgbotapi.Params{"menu_button": string(j)}); err != nil {
		log.Printf("telegram: menu button: %v", redactErr(err))
	}
}

// sendRaw — сообщение с произвольной клавиатурой (web_app-кнопки библиотека не поддерживает).
func (b *Bot) sendRaw(chatID int64, html string, markup any) {
	p := tgbotapi.Params{"chat_id": strconv.FormatInt(chatID, 10), "text": html, "parse_mode": "HTML", "disable_web_page_preview": "true"}
	if markup != nil {
		j, _ := json.Marshal(markup)
		p["reply_markup"] = string(j)
	}
	if _, err := b.api.MakeRequest("sendMessage", p); err != nil {
		log.Printf("telegram: send: %v", redactErr(err))
	}
}

// public — Telegram не принимает кнопки-ссылки на localhost.
func public(u string) bool {
	return u != "" && !strings.Contains(u, "localhost") && !strings.Contains(u, "127.0.0.1")
}

// paymentLinkButton — кнопка «Открыть платёж» под напоминанием (если сайт доступен из интернета).
func (b *Bot) paymentLinkButton(id uint) []tgbotapi.InlineKeyboardButton {
	u := strings.TrimRight(b.opt.PublicURL, "/")
	if !public(u) {
		return nil
	}
	return []tgbotapi.InlineKeyboardButton{tgbotapi.NewInlineKeyboardButtonURL("🔗 Открыть платёж", u+"/payments?open="+strconv.Itoa(int(id)))}
}

func short(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

var monthsRU = []string{"январь", "февраль", "март", "апрель", "май", "июнь", "июль", "август", "сентябрь", "октябрь", "ноябрь", "декабрь"}

func monthRU(d models.Date) string { return fmt.Sprintf("%s %d", monthsRU[d.Month()-1], d.Year()) }
