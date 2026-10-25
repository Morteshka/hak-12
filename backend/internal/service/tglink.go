package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"time"
)

// Ссылка «Открыть бота»: t.me/<бот>?start=<код>. Telegram передаёт код боту вместе с /start,
// поэтому подключение работает с первого нажатия Start — без @имени и без ожидания запроса.
// Код подписан ключом сервера и действует сутки: подделать его или подключить чужой аккаунт нельзя.
// Формат (символы A-Z a-z 0-9 _ -, до 64 знаков, как требует Telegram): L<id>_<срок base36>_<подпись>.

const tgLinkTTL = 24 * time.Hour

func tgLinkSig(secret, payload string) string {
	m := hmac.New(sha256.New, []byte("tglink:"+secret))
	m.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))[:22]
}

// TelegramLinkToken — код для ссылки на бота; пустая строка, если ключ не задан.
func TelegramLinkToken(secret string, userID uint, now time.Time) string {
	if secret == "" {
		return ""
	}
	payload := "L" + strconv.FormatUint(uint64(userID), 10) + "_" + strconv.FormatInt(now.Add(tgLinkTTL).Unix(), 36)
	return payload + "_" + tgLinkSig(secret, payload)
}

// ParseTelegramLinkToken проверяет подпись и срок; возвращает id пользователя сайта.
func ParseTelegramLinkToken(secret, tok string, now time.Time) (uint, bool) {
	if secret == "" || !strings.HasPrefix(tok, "L") {
		return 0, false
	}
	// подпись (base64url) сама может содержать «_», поэтому режем максимум на 3 части
	parts := strings.SplitN(tok, "_", 3)
	if len(parts) != 3 {
		return 0, false
	}
	payload := parts[0] + "_" + parts[1]
	if !hmac.Equal([]byte(parts[2]), []byte(tgLinkSig(secret, payload))) {
		return 0, false
	}
	exp, err := strconv.ParseInt(parts[1], 36, 64)
	if err != nil || now.Unix() > exp {
		return 0, false
	}
	id, err := strconv.ParseUint(parts[0][1:], 10, 64)
	if err != nil || id == 0 {
		return 0, false
	}
	return uint(id), true
}
