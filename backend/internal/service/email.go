package service

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	"invoicekeeper/internal/store"
)

// normEmail проверяет и нормализует адрес. Пустая строка → nil (если required=false).
func normEmail(v string, required bool) (*string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		if required {
			return nil, Invalid("email is required")
		}
		return nil, nil
	}
	a, err := mail.ParseAddress(v)
	if err != nil || a.Address != v || len(v) > 254 {
		return nil, Invalid("invalid email")
	}
	return &v, nil
}

// checkEmailFree — почта не должна принадлежать другому пользователю.
func checkEmailFree(ctx context.Context, st *store.Store, email *string, selfID uint) error {
	if email == nil {
		return nil
	}
	u, err := st.UserByEmail(ctx, *email)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if u.ID != selfID {
		return Conflict("email already taken")
	}
	return nil
}
