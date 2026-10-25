package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"math/big"
	"strings"
	"time"

	"invoicekeeper/internal/models"
	"invoicekeeper/internal/store"
)

// Mailer — отправка писем (реализует notify.Hub). Нужен для кода подтверждения почты.
type Mailer interface {
	MailConfigured(ctx context.Context) bool
	SendMail(ctx context.Context, to, subject, html string) error
}

const (
	codeTTL        = 15 * time.Minute
	codeAttempts   = 5
	resendCooldown = 60 * time.Second
	maxResends     = 5
)

// RegisterPending — регистрация ждёт кода из письма.
type RegisterPending struct {
	Status    string `json:"status"` // "verify"
	Email     string `json:"email"`
	ExpiresIn int    `json:"expires_in"` // секунд до истечения кода
	ResendIn  int    `json:"resend_in"`  // через сколько секунд можно запросить новый код
}

// SetMailer и SetEmailVerify — подтверждение почты при регистрации.
// mode: auto — если почта подключена; true — всегда (без почты регистрация закрыта); false — без кода.
func (s *AuthService) SetMailer(m Mailer) { s.mailer = m }
func (s *AuthService) SetEmailVerify(mode string) {
	s.verifyMode = strings.ToLower(strings.TrimSpace(mode))
}

// EmailVerificationOn — нужен ли код из письма при регистрации прямо сейчас.
func (s *AuthService) EmailVerificationOn(ctx context.Context) bool {
	switch s.verifyMode {
	case "false", "off", "0", "no":
		return false
	case "true", "on", "1", "yes":
		return true
	}
	return s.mailer != nil && s.mailer.MailConfigured(ctx)
}

// startRegistration сохраняет заявку и отправляет 6-значный код. Пароль хранится только как bcrypt-хеш, код — как HMAC.
func (s *AuthService) startRegistration(ctx context.Context, login, email, hash string) (*RegisterPending, error) {
	if s.mailer == nil || !s.mailer.MailConfigured(ctx) {
		return nil, Invalid("email verification is required but mail is not configured")
	}
	now := time.Now()
	if p, err := s.st.PendingRegistration(ctx, email); err == nil && now.Sub(p.SentAt) < resendCooldown && now.Before(p.ExpiresAt) {
		// код только что ушёл — второй не шлём, обновляем пароль и логин
		p.Login, p.PasswordHash = login, hash
		if err := s.st.SavePendingRegistration(ctx, p); err != nil {
			return nil, err
		}
		return pendingInfo(p, now), nil
	} else if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	p := &models.PendingRegistration{Email: email, Login: login, PasswordHash: hash}
	if err := s.sendCode(ctx, p, now); err != nil {
		return nil, err
	}
	return pendingInfo(p, now), nil
}

func pendingInfo(p *models.PendingRegistration, now time.Time) *RegisterPending {
	left := func(d time.Duration) int {
		if d < 0 {
			return 0
		}
		return int(d.Round(time.Second).Seconds())
	}
	return &RegisterPending{Status: "verify", Email: p.Email, ExpiresIn: left(p.ExpiresAt.Sub(now)), ResendIn: left(p.SentAt.Add(resendCooldown).Sub(now))}
}

// sendCode — новый код: сохраняет хеш и отправляет письмо. Если письмо не ушло — заявка не сохраняется.
func (s *AuthService) sendCode(ctx context.Context, p *models.PendingRegistration, now time.Time) error {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return err
	}
	code := fmt.Sprintf("%06d", n.Int64())
	p.CodeHash = s.codeHash(p.Email, code)
	p.Attempts, p.SentAt, p.ExpiresAt = 0, now, now.Add(codeTTL)
	if err := s.mailer.SendMail(ctx, p.Email, "Код подтверждения: "+code, codeEmail(code)); err != nil {
		return Invalid("email failed: " + err.Error())
	}
	return s.st.SavePendingRegistration(ctx, p)
}

func (s *AuthService) codeHash(email, code string) string {
	m := hmac.New(sha256.New, s.secret)
	m.Write([]byte(strings.ToLower(email) + "|" + code))
	return hex.EncodeToString(m.Sum(nil))
}

// ConfirmRegistration — проверка кода. Верный код создаёт аккаунт (почта подтверждена) и сразу выполняет вход.
func (s *AuthService) ConfirmRegistration(ctx context.Context, email, code string, cl Client) (*TokenPair, error) {
	email, code = strings.TrimSpace(email), strings.Join(strings.Fields(code), "")
	p, err := s.st.PendingRegistration(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		return nil, Invalid("code expired, register again")
	} else if err != nil {
		return nil, err
	}
	if time.Now().After(p.ExpiresAt) {
		_ = s.st.DeletePendingRegistration(ctx, p.Email)
		return nil, Invalid("code expired, register again")
	}
	if p.Attempts >= codeAttempts {
		_ = s.st.DeletePendingRegistration(ctx, p.Email)
		return nil, Invalid("too many wrong codes, register again")
	}
	if !hmac.Equal([]byte(s.codeHash(p.Email, code)), []byte(p.CodeHash)) {
		p.Attempts++
		if err := s.st.SavePendingRegistration(ctx, p); err != nil {
			return nil, err
		}
		if left := codeAttempts - p.Attempts; left > 0 {
			return nil, Invalid(fmt.Sprintf("invalid code: %d attempts left", left))
		}
		_ = s.st.DeletePendingRegistration(ctx, p.Email)
		return nil, Invalid("too many wrong codes, register again")
	}
	// за 15 минут почту или логин могли занять — проверяем ещё раз
	em := p.Email
	if err := s.emailFree(ctx, &em, 0); err != nil {
		return nil, err
	}
	now := time.Now()
	u := &models.User{Login: p.Login, PasswordHash: p.PasswordHash, Role: models.RoleUser, Email: &em, EmailVerifiedAt: &now}
	if err := s.st.CreateUser(ctx, u); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return nil, Conflict("login already taken")
		}
		return nil, err
	}
	_ = s.st.DeletePendingRegistration(ctx, p.Email)
	actor := Actor{UserID: &u.ID, Login: u.Login, Role: u.Role, IP: cl.IP, UserAgent: cl.UserAgent}
	s.audit.Log(ctx, actor, ActRegister, "user", u.ID, D{"email_verified": true})
	return s.startSession(ctx, u, cl, "register")
}

// ResendCode — новый код не чаще раза в минуту и не больше 5 раз на заявку.
func (s *AuthService) ResendCode(ctx context.Context, email string) (*RegisterPending, error) {
	p, err := s.st.PendingRegistration(ctx, strings.TrimSpace(email))
	if errors.Is(err, store.ErrNotFound) {
		return nil, Invalid("code expired, register again")
	} else if err != nil {
		return nil, err
	}
	now := time.Now()
	if wait := p.SentAt.Add(resendCooldown).Sub(now); wait > 0 {
		return nil, Invalid(fmt.Sprintf("new code in %d s", int(wait.Seconds())+1))
	}
	if p.Resends >= maxResends {
		return nil, Invalid("too many codes requested, register again later")
	}
	if s.mailer == nil || !s.mailer.MailConfigured(ctx) {
		return nil, Invalid("email verification is required but mail is not configured")
	}
	p.Resends++
	if err := s.sendCode(ctx, p, now); err != nil {
		return nil, err
	}
	return pendingInfo(p, now), nil
}

func codeEmail(code string) string {
	spaced := html.EscapeString(code[:3] + " " + code[3:])
	return `<p style="font-size:16px;margin:0 0 6px"><b>Подтвердите почту</b></p>
<p style="margin:0 0 18px;color:#6b6b6b">Введите этот код на странице регистрации Invoice Keeper:</p>
<div style="display:inline-block;padding:16px 26px;border-radius:16px;background:#FFF1E6;border:1px solid #FFD3B0;font-size:34px;font-weight:700;letter-spacing:6px;color:#141414;font-family:Menlo,Consolas,monospace">` + spaced + `</div>
<p style="margin:18px 0 0;color:#6b6b6b;font-size:13px">Код действует 15 минут. Если вы не регистрировались — просто проигнорируйте письмо.</p>`
}
