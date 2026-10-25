package telegram

import (
	"errors"
	"io"
	"log"
	"os"
	"regexp"
)

// Текст сетевых ошибок содержит адрес запроса вместе с токеном бота (…/bot<id>:<секрет>/getMe).
// Токен не должен попадать ни в логи, ни в статус бота на сайте.
var tokenRe = regexp.MustCompile(`bot\d+:[A-Za-z0-9_-]{20,}`)

// Redact скрывает токен бота в тексте.
func Redact(s string) string { return tokenRe.ReplaceAllString(s, "bot***") }

func redactErr(err error) error {
	if err == nil {
		return nil
	}
	return errors.New(Redact(err.Error()))
}

type redactWriter struct{ w io.Writer }

func (r redactWriter) Write(p []byte) (int, error) {
	if _, err := r.w.Write([]byte(Redact(string(p)))); err != nil {
		return 0, err
	}
	return len(p), nil
}

// InstallLogRedaction: все строки журнала проходят через Redact — токен не попадёт в docker logs,
// даже если библиотека или код выведет адрес запроса целиком.
func InstallLogRedaction() { log.SetOutput(redactWriter{os.Stderr}) }
