// Package service — бизнес-логика. Не знает ни про HTTP, ни про Telegram.
package service

import "errors"

type Kind int

const (
	KindInvalid Kind = iota + 1
	KindUnauthorized
	KindForbidden
	KindNotFound
	KindConflict
)

// Error — доменная ошибка; handler переводит Kind в HTTP-статус.
type Error struct {
	Kind Kind
	Msg  string
}

func (e *Error) Error() string { return e.Msg }

func Invalid(msg string) error      { return &Error{KindInvalid, msg} }
func Unauthorized(msg string) error { return &Error{KindUnauthorized, msg} }
func Forbidden(msg string) error    { return &Error{KindForbidden, msg} }
func NotFound(msg string) error     { return &Error{KindNotFound, msg} }
func Conflict(msg string) error     { return &Error{KindConflict, msg} }

// IsUnauthorized — ошибка «неверный логин или пароль / токен».
func IsUnauthorized(err error) bool {
	e, ok := AsError(err)
	return ok && e.Kind == KindUnauthorized
}

func AsError(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}

// Actor — кто выполняет действие (для логов и прав).
type Actor struct {
	UserID *uint
	Login  string
	Role   string
	IP     string
	// UserAgent — браузер/клиент (для журнала).
	UserAgent string
}
