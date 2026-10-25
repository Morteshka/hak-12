package telegram

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestWithoutButton(t *testing.T) {
	row := func(data string) []tgbotapi.InlineKeyboardButton {
		return tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("x", data))
	}
	list := tgbotapi.NewInlineKeyboardMarkup(row("paid:1"), row("paid:2"))
	rest, isList := withoutButton(&list, "paid:1", "📅 Ближайшие платежи…")
	if !isList || len(rest.InlineKeyboard) != 1 || *rest.InlineKeyboard[0][0].CallbackData != "paid:2" {
		t.Fatalf("list: %v %+v", isList, rest)
	}
	// последняя кнопка списка: всё ещё список (по заголовку), клавиатура пустая
	one := tgbotapi.NewInlineKeyboardMarkup(row("paid:2"))
	if rest, isList := withoutButton(&one, "paid:2", "Какой платёж оплачен? …"); !isList || len(rest.InlineKeyboard) != 0 {
		t.Fatalf("last in list: %v %+v", isList, rest)
	}
	// одиночное напоминание — не список
	rem := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("✅ Оплачено", "paid:5"), tgbotapi.NewInlineKeyboardButtonURL("🔗", "https://e.x/p")))
	if _, isList := withoutButton(&rem, "paid:5", "🔔 Напоминание об оплате"); isList {
		t.Fatal("reminder treated as list")
	}
}
