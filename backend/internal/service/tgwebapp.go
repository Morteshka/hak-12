package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"invoicekeeper/internal/store"
)

// ---------- Telegram Mini App ----------
//
// Сайт открывается внутри Telegram (кнопка «Приложение» в боте). Telegram передаёт странице initData —
// подписанные ключом бота данные пользователя. Сервер проверяет подпись (HMAC-SHA256, алгоритм из документации
// Telegram), и если этот Telegram уже подключён к аккаунту — сразу выдаёт токены, без логина и пароля.

// TelegramWebAppUser — пользователь Telegram из проверенного initData.
type TelegramWebAppUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

var errBadInitData = Unauthorized("telegram data is invalid or expired, reopen the app from the bot")

// VerifyTelegramInitData проверяет подпись initData и срок (не старше maxAge).
func VerifyTelegramInitData(botToken, initData string, maxAge time.Duration, now time.Time) (*TelegramWebAppUser, error) {
	if botToken == "" || initData == "" {
		return nil, errBadInitData
	}
	vals, err := url.ParseQuery(initData)
	if err != nil {
		return nil, errBadInitData
	}
	hash := vals.Get("hash")
	if hash == "" {
		return nil, errBadInitData
	}
	keys := make([]string, 0, len(vals))
	for k := range vals {
		if k != "hash" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	lines := make([]string, len(keys))
	for i, k := range keys {
		lines[i] = k + "=" + vals.Get(k)
	}
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(botToken))
	m := hmac.New(sha256.New, secret.Sum(nil))
	m.Write([]byte(strings.Join(lines, "\n")))
	want := hex.EncodeToString(m.Sum(nil))
	if !hmac.Equal([]byte(want), []byte(strings.ToLower(hash))) {
		return nil, errBadInitData
	}
	ts, err := strconv.ParseInt(vals.Get("auth_date"), 10, 64)
	if err != nil {
		return nil, errBadInitData
	}
	at := time.Unix(ts, 0)
	if now.Sub(at) > maxAge || at.Sub(now) > 5*time.Minute {
		return nil, errBadInitData
	}
	var u TelegramWebAppUser
	if err := json.Unmarshal([]byte(vals.Get("user")), &u); err != nil || u.ID == 0 {
		return nil, errBadInitData
	}
	return &u, nil
}

const webAppMaxAge = 24 * time.Hour

// SetTelegramToken — токен бота для проверки initData (пустой — вход через Telegram выключен).
func (s *AuthService) SetTelegramToken(token string) { s.tgToken = token }

// LoginTelegram — вход из Mini App. ErrTelegramNotLinked — Telegram ещё не подключён к аккаунту.
func (s *AuthService) LoginTelegram(ctx context.Context, initData string, cl Client) (*TokenPair, error) {
	tu, err := VerifyTelegramInitData(s.tgToken, initData, webAppMaxAge, time.Now())
	if err != nil {
		return nil, err
	}
	u, err := s.st.UserByTelegramID(ctx, tu.ID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrTelegramNotLinked
	}
	if err != nil {
		return nil, err
	}
	return s.startSession(ctx, u, cl, "telegram")
}

// ErrTelegramNotLinked — нужно один раз войти логином и паролем: Telegram подключится сам.
var ErrTelegramNotLinked = Forbidden("telegram is not linked: sign in once and it will be linked automatically")

// LinkTelegramWebApp — пользователь вошёл паролем внутри Mini App: подключаем этот Telegram к аккаунту.
// Подпись initData доказывает, что это его Telegram, поэтому подтверждение в боте не нужно.
func (s *AuthService) LinkTelegramWebApp(ctx context.Context, a Actor, initData string) error {
	tu, err := VerifyTelegramInitData(s.tgToken, initData, webAppMaxAge, time.Now())
	if err != nil {
		return err
	}
	if a.UserID == nil {
		return Unauthorized("login required")
	}
	if err := s.st.LinkTelegramDirect(ctx, *a.UserID, tu.ID, tu.Username); err != nil {
		return err
	}
	s.audit.Log(ctx, a, ActSettings, "user", *a.UserID, D{"telegram": "linked via mini app", "telegram_id": tu.ID})
	return nil
}
