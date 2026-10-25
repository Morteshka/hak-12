package service

import (
	"context"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"invoicekeeper/internal/models"
	"invoicekeeper/internal/store"
)

// Register создаёт обычного пользователя (роль user) и сразу выполняет вход.
// Если логин похож на почту и email не передан — почтой считается логин.
// Если включено подтверждение почты — аккаунт ещё не создаётся: на почту уходит код (RegisterPending),
// а создаёт аккаунт ConfirmRegistration.
func (s *AuthService) Register(ctx context.Context, login, email, password string, cl Client) (*TokenPair, *RegisterPending, error) {
	pair, pend, err := s.register(ctx, login, email, password, cl)
	return pair, pend, err
}

func (s *AuthService) register(ctx context.Context, login, email, password string, cl Client) (*TokenPair, *RegisterPending, error) {
	login = strings.TrimSpace(login)
	email = strings.TrimSpace(email)
	if email == "" && strings.Contains(login, "@") {
		email = login
	}
	if login == "" {
		login = email
	}
	if err := validateCreds(login, password, true, true); err != nil {
		return nil, nil, err
	}
	em, err := normEmail(email, false)
	if err != nil {
		return nil, nil, err
	}
	if err := s.emailFree(ctx, em, 0); err != nil {
		return nil, nil, err
	}
	verify := s.EmailVerificationOn(ctx)
	if verify && em == nil {
		return nil, nil, Invalid("email is required")
	}
	if verify {
		if _, err := s.st.UserByLogin(ctx, login); err == nil {
			return nil, nil, Conflict("login already taken")
		} else if !errors.Is(err, store.ErrNotFound) {
			return nil, nil, err
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, err
	}
	if verify {
		pend, err := s.startRegistration(ctx, login, *em, string(hash))
		return nil, pend, err
	}
	u := &models.User{Login: login, PasswordHash: string(hash), Role: models.RoleUser, Email: em}
	if err := s.st.CreateUser(ctx, u); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return nil, nil, Conflict("login already taken")
		}
		return nil, nil, err
	}
	s.audit.Log(ctx, Actor{UserID: &u.ID, Login: u.Login, Role: u.Role, IP: cl.IP, UserAgent: cl.UserAgent}, ActRegister, "user", u.ID, "")
	pair, err := s.Login(ctx, login, password, cl)
	return pair, nil, err
}

func (s *AuthService) emailFree(ctx context.Context, email *string, selfID uint) error {
	return checkEmailFree(ctx, s.st, email, selfID)
}

// ChangePassword — смена своего пароля. Пароль администратора задаётся только в .env.
// Остальные сессии пользователя завершаются, текущая (keepSID) остаётся.
func (s *AuthService) ChangePassword(ctx context.Context, a Actor, u *models.User, keepSID, oldPw, newPw string) error {
	if u.IsSystem {
		return Forbidden("administrator password is set via .env")
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(oldPw)) != nil {
		return Invalid("current password is incorrect")
	}
	if err := validateCreds("", newPw, false, true); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPw), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	if err := s.st.SaveUser(ctx, u); err != nil {
		return err
	}
	if err := s.st.RevokeUserSessions(ctx, u.ID, keepSID); err != nil {
		return err
	}
	s.audit.Log(ctx, a, ActPasswordChange, "user", u.ID, "")
	return nil
}

// ChangeLogin — смена своего имени (логина). Учётная запись админа задаётся только в .env.
func (s *AuthService) ChangeLogin(ctx context.Context, a Actor, u *models.User, login string) (*models.User, error) {
	if u.IsSystem {
		return nil, Forbidden("administrator account is managed via .env")
	}
	login = strings.TrimSpace(login)
	if err := validateCreds(login, "", true, false); err != nil {
		return nil, err
	}
	old := u.Login
	u.Login = login
	if err := s.st.SaveUser(ctx, u); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return nil, Conflict("login already taken")
		}
		return nil, err
	}
	a.Login = login
	s.audit.Log(ctx, a, ActUpdateUser, "user", u.ID, D{"from": old, "to": login})
	return u, nil
}
