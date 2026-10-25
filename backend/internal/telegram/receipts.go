package telegram

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"invoicekeeper/internal/models"
	"invoicekeeper/internal/notify"
	"invoicekeeper/internal/service"
)

// ---------- состояние: чек к какому платежу ждём ----------

type tgFile struct {
	ID, Name string
	Size     int
}

type pendingState struct {
	mu      sync.Mutex
	waiting map[int64]waitEntry // Telegram ID → платёж, по которому попросили чек
	files   map[string]fileEntry
}

type waitEntry struct {
	occID uint
	at    time.Time
}

type fileEntry struct {
	file  tgFile
	tgID  int64
	note  string
	at    time.Time
	chat  int64
	reply int
}

func newPendingState() *pendingState {
	return &pendingState{waiting: map[int64]waitEntry{}, files: map[string]fileEntry{}}
}

func (p *pendingState) wait(tgID int64, occID uint) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.waiting[tgID] = waitEntry{occID: occID, at: time.Now()}
}

// take — платёж, по которому человек недавно нажал «Оплачено» (сутки).
func (p *pendingState) take(tgID int64) uint {
	p.mu.Lock()
	defer p.mu.Unlock()
	w, ok := p.waiting[tgID]
	delete(p.waiting, tgID)
	if !ok || time.Since(w.at) > 24*time.Hour {
		return 0
	}
	return w.occID
}

func (p *pendingState) putFile(e fileEntry) string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)
	p.mu.Lock()
	defer p.mu.Unlock()
	for k, v := range p.files { // чистим старые
		if time.Since(v.at) > 24*time.Hour {
			delete(p.files, k)
		}
	}
	p.files[token] = e
	return token
}

func (p *pendingState) popFile(token string) (fileEntry, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, ok := p.files[token]
	delete(p.files, token)
	return e, ok
}

// ---------- «Оплачено» → просим чек ----------

// paymentMarker — номер платежа в тексте сообщения-запроса: ответом на него присылают чек.
var paymentMarker = regexp.MustCompile(`платёж #(\d+)`)

func (b *Bot) askForReceipt(ctx context.Context, q *tgbotapi.CallbackQuery, actor service.Actor, id uint) {
	answer := func(text string) { _, _ = b.api.Request(tgbotapi.NewCallback(q.ID, text)) }
	v, err := b.payments.GetOccurrence(ctx, actor, id)
	if err != nil {
		answer("Платёж не найден")
		return
	}
	if v.PaidAt != nil {
		answer("Платёж уже оплачен ✅")
		return
	}
	answer("Пришлите чек об оплате 🧾")
	b.pending.wait(q.From.ID, id)
	text := fmt.Sprintf("🧾 <b>Подтвердите оплату чеком</b>\n«%s» — %s\n\nПришлите <b>ответом на это сообщение</b> фото, скриншот или PDF чека (квитанции из банка). Я сверю сумму, дату и номер операции и отмечу оплату.\n\n<i>платёж #%d</i>",
		html.EscapeString(v.Title), moneyRU(v.Amount.StringFixed(2), v.Currency), v.ID)
	if v.ReceiptStatus != nil && *v.ReceiptStatus == models.ReceiptPending {
		text += "\n\nℹ️ Предыдущий чек ещё на проверке — новый заменит его."
	}
	msg := tgbotapi.NewMessage(q.Message.Chat.ID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true, Selective: true, InputFieldPlaceholder: "Фото или PDF чека"}
	if q.Message.Chat.IsPrivate() {
		msg.ReplyMarkup = nil
	}
	b.send(msg)
}

// ---------- приём файла ----------

func (b *Bot) onReceiptFile(ctx context.Context, m *tgbotapi.Message) {
	if b.receipts == nil || m.From == nil {
		return
	}
	// номер платежа из сообщения, на которое ответили
	var occID uint
	if r := m.ReplyToMessage; r != nil && r.From != nil && r.From.ID == b.api.Self.ID {
		if mm := paymentMarker.FindStringSubmatch(r.Text + r.Caption); mm != nil {
			v, _ := strconv.ParseUint(mm[1], 10, 64)
			occID = uint(v)
		}
	}
	if !m.Chat.IsPrivate() && occID == 0 {
		return // в группе принимаем чек только ответом на запрос бота
	}
	me := b.linkedUser(ctx, m.From)
	if me == nil {
		b.replyTo(m, "Чтобы прислать чек, подключите Telegram на сайте: «Профиль и настройки» → Telegram, затем подтвердите здесь.")
		return
	}
	f, errText := fileOf(m)
	if errText != "" {
		b.replyTo(m, errText)
		return
	}
	if occID == 0 {
		occID = b.pending.take(m.From.ID)
	}
	if occID != 0 {
		b.processReceipt(ctx, *me, occID, f, m.Caption, m.Chat.ID, m.MessageID)
		return
	}
	// не знаем, к какому платежу — спрашиваем
	list, err := b.payments.Upcoming(ctx, *me, "", 6)
	if err != nil || len(list) == 0 {
		b.replyTo(m, "У вас нет неоплаченных платежей — прикладывать чек не к чему 🎉")
		return
	}
	token := b.pending.putFile(fileEntry{file: f, tgID: m.From.ID, note: m.Caption, at: time.Now(), chat: m.Chat.ID, reply: m.MessageID})
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, v := range list {
		label := fmt.Sprintf("%s · %s · до %s", v.Title, moneyRU(v.Amount.StringFixed(2), v.Currency), formatDate(v.DueDate))
		if len([]rune(label)) > 60 {
			label = string([]rune(label)[:59]) + "…"
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(label, "rcf:"+token+":"+strconv.Itoa(int(v.ID)))))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Отмена", "rcf:"+token+":0")))
	msg := tgbotapi.NewMessage(m.Chat.ID, "🧾 К какому платежу этот чек?")
	msg.ReplyToMessageID = m.MessageID
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(msg)
}

// onReceiptChoice — выбран платёж для присланного файла.
func (b *Bot) onReceiptChoice(ctx context.Context, q *tgbotapi.CallbackQuery) {
	answer := func(text string) { _, _ = b.api.Request(tgbotapi.NewCallback(q.ID, text)) }
	parts := strings.Split(q.Data, ":")
	if len(parts) != 3 || q.Message == nil {
		answer("Некорректная кнопка")
		return
	}
	e, ok := b.pending.popFile(parts[1])
	if !ok || e.tgID != q.From.ID {
		answer("Запрос устарел — пришлите чек ещё раз")
		return
	}
	occID, _ := strconv.ParseUint(parts[2], 10, 64)
	if occID == 0 {
		answer("Отменено")
		b.editText(q.Message.Chat.ID, q.Message.MessageID, "Чек не отправлен.")
		return
	}
	me := b.linkedUser(ctx, q.From)
	if me == nil {
		answer("Подключите Telegram на сайте")
		return
	}
	answer("Проверяю чек…")
	_, _ = b.api.Request(tgbotapi.NewDeleteMessage(q.Message.Chat.ID, q.Message.MessageID))
	go b.processReceipt(ctx, *me, uint(occID), e.file, e.note, e.chat, e.reply)
}

// processReceipt скачивает файл, отправляет на проверку и отвечает результатом.
func (b *Bot) processReceipt(ctx context.Context, me service.Actor, occID uint, f tgFile, note string, chat int64, replyTo int) {
	wait := tgbotapi.NewMessage(chat, "⏳ Проверяю чек: распознаю сумму, дату и номер операции…")
	wait.ReplyToMessageID = replyTo
	sent, err := b.api.Send(wait)
	if err != nil {
		log.Printf("telegram: send: %v", err)
	}
	finish := func(text string) {
		if sent.MessageID != 0 {
			b.editText(chat, sent.MessageID, text)
		} else {
			b.reply(chat, text)
		}
	}
	data, err := b.download(ctx, f.ID)
	if err != nil {
		log.Printf("telegram: download receipt: %v", err)
		finish("Не удалось скачать файл из Telegram, попробуйте ещё раз.")
		return
	}
	res, err := b.receipts.Submit(ctx, me, occID, service.Upload{Name: f.Name, Note: note, Source: "telegram", Data: data})
	if err != nil {
		finish("❌ Чек не принят: " + receiptErrorRU(err))
		return
	}
	finish(resultText(res))
}

func resultText(res *service.SubmitResult) string {
	r := res.Receipt
	var checks []models.ReceiptCheck
	_ = json.Unmarshal(r.Checks, &checks)
	title := html.EscapeString(r.PaymentTitle)
	amount := moneyRU(r.PaymentAmount.StringFixed(2), r.PaymentCurrency)
	if r.Status == models.ReceiptApproved {
		paid := ""
		if res.Occurrence != nil && res.Occurrence.PaidAt != nil {
			paid = " от " + formatDate(*res.Occurrence.PaidAt)
		}
		var ok []string
		for _, c := range checks {
			ok = append(ok, "✓ "+html.EscapeString(c.Title))
		}
		return fmt.Sprintf("✅ <b>Оплата подтверждена</b>\n«%s» — %s%s\n\nЧек проверен автоматически:\n%s", title, amount, paid, strings.Join(ok, "\n"))
	}
	var issues []string
	for _, c := range checks {
		switch c.Status {
		case models.CheckFail:
			issues = append(issues, "❌ "+html.EscapeString(c.Title+": "+c.Detail))
		case models.CheckWarn:
			issues = append(issues, "⚠️ "+html.EscapeString(c.Title+": "+c.Detail))
		}
	}
	text := fmt.Sprintf("🔎 <b>Чек отправлен администратору на проверку</b>\n«%s» — %s\n", title, amount)
	if len(issues) > 0 {
		text += "\nЧто смутило автопроверку:\n" + strings.Join(issues, "\n") + "\n"
	}
	return text + "\nЯ напишу, когда администратор примет решение."
}

// ---------- решение администратора ----------

var rejectReasons = []string{
	"Сумма не совпадает с платежом",
	"Чек нечитаемый — пришлите чёткое фото или PDF",
	"Чек от другого платежа или получателя",
	"Чек уже использовался",
	"Похоже на подделку",
}

// onReceiptDecision: rcpt:ok:<id>, rcpt:no:<id> (показать причины), rcpr:<id>:<n> (отклонить с причиной), rcpt:back:<id>.
func (b *Bot) onReceiptDecision(ctx context.Context, q *tgbotapi.CallbackQuery) {
	answer := func(text string) { _, _ = b.api.Request(tgbotapi.NewCallback(q.ID, text)) }
	if q.Message == nil {
		answer("Некорректная кнопка")
		return
	}
	me := b.linkedUser(ctx, q.From)
	if me == nil || me.Role != models.RoleAdmin {
		answer("Решение принимает администратор")
		return
	}
	parts := strings.Split(q.Data, ":")
	var id uint64
	var err error
	switch parts[0] {
	case "rcpt":
		if len(parts) == 3 {
			id, err = strconv.ParseUint(parts[2], 10, 64)
		}
	case "rcpr":
		if len(parts) == 3 {
			id, err = strconv.ParseUint(parts[1], 10, 64)
		}
	}
	if id == 0 || err != nil {
		answer("Некорректная кнопка")
		return
	}
	sid := strconv.FormatUint(id, 10)
	switch {
	case parts[0] == "rcpt" && parts[1] == "no": // выбор причины
		var rows [][]tgbotapi.InlineKeyboardButton
		for i, reason := range rejectReasons {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(reason, "rcpr:"+sid+":"+strconv.Itoa(i))))
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("← Назад", "rcpt:back:"+sid)))
		answer("Выберите причину")
		_, _ = b.api.Send(tgbotapi.NewEditMessageReplyMarkup(q.Message.Chat.ID, q.Message.MessageID, tgbotapi.NewInlineKeyboardMarkup(rows...)))
		return
	case parts[0] == "rcpt" && parts[1] == "back":
		answer("")
		_, _ = b.api.Send(tgbotapi.NewEditMessageReplyMarkup(q.Message.Chat.ID, q.Message.MessageID, decisionKeyboard(sid)))
		return
	}

	var (
		res   *service.SubmitResult
		toast string
	)
	if parts[0] == "rcpt" && parts[1] == "ok" {
		res, err = b.receipts.Approve(ctx, *me, uint(id), nil, "Подтверждено в Telegram")
		toast = "Оплата подтверждена ✅"
	} else {
		n, _ := strconv.Atoi(parts[2])
		if n < 0 || n >= len(rejectReasons) {
			answer("Некорректная кнопка")
			return
		}
		res, err = b.receipts.Reject(ctx, *me, uint(id), rejectReasons[n])
		toast = "Чек отклонён"
	}
	if err != nil {
		if e, ok := service.AsError(err); ok && e.Kind == service.KindConflict {
			answer("Решение по этому чеку уже принято")
		} else {
			log.Printf("telegram: receipt decision #%d: %v", id, err)
			answer("Не удалось сохранить решение")
		}
		return
	}
	answer(toast)
	r := res.Receipt
	result := "\n\n✅ <b>Подтверждено</b> — " + html.EscapeString(me.Login)
	if r.Status == models.ReceiptRejected {
		result = "\n\n❌ <b>Отклонено</b> — " + html.EscapeString(me.Login) + ": " + html.EscapeString(r.DecisionComment)
	}
	caption := notify.ReceiptCaption(r)
	if len([]rune(caption))+len([]rune(result)) > 1020 {
		caption = string([]rune(caption)[:max(0, 1020-len([]rune(result)))])
	}
	edit := tgbotapi.NewEditMessageCaption(q.Message.Chat.ID, q.Message.MessageID, caption+result)
	edit.ParseMode = tgbotapi.ModeHTML
	empty := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}}
	edit.ReplyMarkup = &empty
	if _, err := b.api.Send(edit); err != nil {
		// сообщение без подписи (например, текстовое) — просто убираем кнопки
		_, _ = b.api.Send(tgbotapi.NewEditMessageReplyMarkup(q.Message.Chat.ID, q.Message.MessageID, empty))
	}
}

func decisionKeyboard(sid string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("✅ Подтвердить", "rcpt:ok:"+sid),
		tgbotapi.NewInlineKeyboardButtonData("❌ Отклонить", "rcpt:no:"+sid),
	))
}

// ---------- helpers ----------

// fileOf — файл из сообщения: самое крупное фото или документ (PDF / картинка).
func fileOf(m *tgbotapi.Message) (tgFile, string) {
	if len(m.Photo) > 0 {
		p := m.Photo[len(m.Photo)-1]
		return tgFile{ID: p.FileID, Name: "photo-" + time.Now().Format("20060102-150405") + ".jpg", Size: p.FileSize}, ""
	}
	d := m.Document
	switch {
	case d == nil:
		return tgFile{}, "Пришлите фото или PDF чека."
	case d.FileSize > service.MaxReceiptSize:
		return tgFile{}, "Файл больше 10 МБ — пришлите фото или PDF поменьше."
	case d.MimeType != "application/pdf" && !strings.HasPrefix(d.MimeType, "image/"):
		return tgFile{}, "Нужен чек в виде фото (JPG, PNG) или PDF."
	}
	return tgFile{ID: d.FileID, Name: d.FileName, Size: d.FileSize}, ""
}

func (b *Bot) download(ctx context.Context, fileID string) ([]byte, error) {
	f, err := b.api.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		return nil, err
	}
	url := f.Link(b.opt.Token)
	if b.opt.APIEndpoint != "" { // зеркало Bot API: файлы скачиваются через него же
		url = strings.TrimSuffix(b.opt.APIEndpoint, "/bot%s/%s") + "/file/bot" + b.opt.Token + "/" + f.FilePath
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download failed") // URL содержит токен — в лог не пишем
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download: HTTP %d", res.StatusCode)
	}
	return io.ReadAll(io.LimitReader(res.Body, service.MaxReceiptSize+1))
}

func (b *Bot) replyTo(m *tgbotapi.Message, text string) {
	msg := tgbotapi.NewMessage(m.Chat.ID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyToMessageID = m.MessageID
	b.send(msg)
}

func (b *Bot) editText(chat int64, msgID int, text string) {
	edit := tgbotapi.NewEditMessageText(chat, msgID, text)
	edit.ParseMode = tgbotapi.ModeHTML
	b.send(edit)
}

func moneyRU(amount, cur string) string { return service.MoneyText(amount, cur) }

// receiptErrorRU — понятные причины отказа принять файл.
func receiptErrorRU(err error) string {
	e, ok := service.AsError(err)
	if !ok {
		log.Printf("telegram: receipt: %v", err)
		return "внутренняя ошибка, попробуйте позже."
	}
	switch {
	case strings.Contains(e.Msg, "already marked as paid"):
		return "этот платёж уже оплачен."
	case strings.Contains(e.Msg, "unsupported file type"):
		return "нужен файл JPG, PNG, WebP или PDF."
	case strings.Contains(e.Msg, "too large"):
		return "файл больше 10 МБ."
	case e.Kind == service.KindNotFound:
		return "платёж не найден или недоступен вам."
	}
	return e.Msg
}
