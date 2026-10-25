package service

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"invoicekeeper/internal/models"
	"invoicekeeper/internal/store"
)

// UserService — управление обычными пользователями. Только для администратора.
type UserService struct {
	st    *store.Store
	audit *Audit
}

func NewUserService(st *store.Store, audit *Audit) *UserService {
	return &UserService{st: st, audit: audit}
}

func requireAdmin(a Actor) error {
	if a.Role != models.RoleAdmin {
		return Forbidden("admin role required")
	}
	return nil
}

func (s *UserService) List(ctx context.Context, a Actor) ([]models.User, error) {
	if err := requireAdmin(a); err != nil {
		return nil, err
	}
	return s.st.ListUsers(ctx)
}

func validateCreds(login, password string, needLogin, needPass bool) error {
	if n := utf8.RuneCountInString(login); needLogin && (n < 3 || n > 64 || strings.ContainsAny(login, " \t\n")) {
		return Invalid("login must be 3-64 characters without spaces")
	}
	if needPass && len(password) < 6 {
		return Invalid("password must be at least 6 characters")
	}
	return nil
}

// Create — новый пользователь (роль всегда user). Почта обязательна: на неё приходят email-напоминания.
func (s *UserService) Create(ctx context.Context, a Actor, login, email, password string) (*models.User, error) {
	if err := requireAdmin(a); err != nil {
		return nil, err
	}
	login = strings.TrimSpace(login)
	if err := validateCreds(login, password, true, true); err != nil {
		return nil, err
	}
	em, err := normEmail(email, true)
	if err != nil {
		return nil, err
	}
	if err := checkEmailFree(ctx, s.st, em, 0); err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u := &models.User{Login: login, PasswordHash: string(hash), Role: models.RoleUser, Email: em, EmailNotifications: true} // роль всегда user
	if err := s.st.CreateUser(ctx, u); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return nil, Conflict("login already taken")
		}
		return nil, err
	}
	s.audit.Log(ctx, a, ActCreateUser, "user", u.ID, D{"login": login, "email": *em})
	return u, nil
}

func (s *UserService) Update(ctx context.Context, a Actor, id uint, login, email, password *string) (*models.User, error) {
	if err := requireAdmin(a); err != nil {
		return nil, err
	}
	u, err := s.target(ctx, id)
	if err != nil {
		return nil, err
	}
	if login == nil && password == nil && email == nil {
		return nil, Invalid("nothing to update")
	}
	changed := []string{}
	if email != nil {
		em, err := normEmail(*email, true)
		if err != nil {
			return nil, err
		}
		if err := checkEmailFree(ctx, s.st, em, u.ID); err != nil {
			return nil, err
		}
		u.Email = em
		changed = append(changed, "email")
	}
	if login != nil {
		if err := validateCreds(*login, "", true, false); err != nil {
			return nil, err
		}
		u.Login = *login
		changed = append(changed, "login")
	}
	if password != nil {
		if err := validateCreds("", *password, false, true); err != nil {
			return nil, err
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		u.PasswordHash = string(hash)
		changed = append(changed, "password")
	}
	if err := s.st.SaveUser(ctx, u); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return nil, Conflict("login already taken")
		}
		return nil, err
	}
	if password != nil { // новый пароль от администратора — все старые входы пользователя завершаются
		if err := s.st.RevokeUserSessions(ctx, u.ID, ""); err != nil {
			return nil, err
		}
	}
	s.audit.Log(ctx, a, ActUpdateUser, "user", u.ID, D{"changed": changed})
	return u, nil
}

func (s *UserService) Delete(ctx context.Context, a Actor, id uint) error {
	if err := requireAdmin(a); err != nil {
		return err
	}
	u, err := s.target(ctx, id)
	if err != nil {
		return err
	}
	if err := s.st.DeleteUser(ctx, u.ID); err != nil {
		return err
	}
	s.audit.Log(ctx, a, ActDeleteUser, "user", u.ID, D{"login": u.Login})
	return nil
}

// target возвращает обычного пользователя; учётка админа из .env недоступна для изменения.
func (s *UserService) target(ctx context.Context, id uint) (*models.User, error) {
	u, err := s.st.UserByID(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, NotFound("user not found")
	}
	if err != nil {
		return nil, err
	}
	if u.IsSystem || u.Role == models.RoleAdmin {
		return nil, Forbidden("administrator account is managed via .env")
	}
	return u, nil
}
