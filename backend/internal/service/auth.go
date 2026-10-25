package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"invoicekeeper/internal/models"
	"invoicekeeper/internal/store"
)

type AuthService struct {
	st         *store.Store
	audit      *Audit
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	tgToken    string // токен бота: проверка входа из Telegram Mini App
	mailer     Mailer // письма с кодом подтверждения почты
	verifyMode string // EMAIL_VERIFY: auto | true | false
}

func NewAuthService(st *store.Store, audit *Audit, secret string, accessTTL, refreshTTL time.Duration) *AuthService {
	return &AuthService{st: st, audit: audit, secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL}
}

type claims struct {
	UID  uint   `json:"uid,omitempty"`
	SID  string `json:"sid"`
	Type string `json:"typ"` // access | refresh
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	TokenType    string      `json:"token_type"`
	ExpiresIn    int         `json:"expires_in"`
	User         models.User `json:"user"`
}

// EnsureAdmin создаёт/обновляет администратора из .env. Через панель он не управляется.
// email (ADMIN_EMAIL) необязателен — нужен только для email-напоминаний админу.
func (s *AuthService) EnsureAdmin(ctx context.Context, login, password, email string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	var emailPtr *string
	if email = strings.TrimSpace(email); email != "" {
		emailPtr = &email
	}
	u, err := s.st.UserByLogin(ctx, login)
	if errors.Is(err, store.ErrNotFound) {
		return s.st.CreateUser(ctx, &models.User{Login: login, PasswordHash: string(hash), Role: models.RoleAdmin, IsSystem: true,
			Email: emailPtr, EmailNotifications: emailPtr != nil})
	}
	if err != nil {
		return err
	}
	changed := false
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil || u.Role != models.RoleAdmin || !u.IsSystem {
		u.PasswordHash, u.Role, u.IsSystem = string(hash), models.RoleAdmin, true
		changed = true
	}
	if emailPtr != nil && (u.Email == nil || *u.Email != email) {
		u.Email = emailPtr
		changed = true
	}
	if changed {
		return s.st.SaveUser(ctx, u)
	}
	return nil
}

// Client — откуда пришёл запрос (для журнала).
type Client struct {
	IP        string
	UserAgent string
}

// Login — вход по логину или почте.
func (s *AuthService) Login(ctx context.Context, login, password string, cl Client) (*TokenPair, error) {
	login = strings.TrimSpace(login)
	u, err := s.st.UserByLoginOrEmail(ctx, login)
	if err != nil {
		// сравниваем с фиктивным хешем, чтобы по времени ответа нельзя было узнать, есть ли такой логин
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
	}
	if err != nil || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		s.audit.Log(ctx, Actor{Login: login, IP: cl.IP, UserAgent: cl.UserAgent}, ActLoginFailed, "user", nil, "")
		return nil, Unauthorized("invalid login or password")
	}
	return s.startSession(ctx, u, cl, "")
}

// startSession — новая сессия и пара токенов; via — способ входа для журнала («telegram» для Mini App).
func (s *AuthService) startSession(ctx context.Context, u *models.User, cl Client, via string) (*TokenPair, error) {
	sid, err := randomID()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if err := s.st.CreateSession(ctx, &models.Session{ID: sid, UserID: u.ID, ExpiresAt: now.Add(s.refreshTTL)}); err != nil {
		return nil, err
	}
	pair, err := s.issue(u, sid, true)
	if err != nil {
		return nil, err
	}
	var details any = ""
	if via != "" {
		details = D{"via": via}
	}
	s.audit.Log(ctx, Actor{UserID: &u.ID, Login: u.Login, Role: u.Role, IP: cl.IP, UserAgent: cl.UserAgent}, ActLogin, "user", u.ID, details)
	return pair, nil
}

// Refresh выдаёт новый access-токен по refresh-токену живой сессии.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	c, err := s.parse(refreshToken, "refresh")
	if err != nil {
		return nil, Unauthorized("invalid refresh token")
	}
	u, _, err := s.loadSession(ctx, c.SID)
	if err != nil {
		return nil, err
	}
	pair, err := s.issue(u, c.SID, false)
	if err != nil {
		return nil, err
	}
	pair.RefreshToken = refreshToken
	return pair, nil
}

// Authenticate проверяет access-токен и то, что сессия не завершена.
func (s *AuthService) Authenticate(ctx context.Context, accessToken string) (*models.User, string, error) {
	c, err := s.parse(accessToken, "access")
	if err != nil {
		return nil, "", Unauthorized("invalid or expired token")
	}
	u, _, err := s.loadSession(ctx, c.SID)
	if err != nil {
		return nil, "", err
	}
	return u, c.SID, nil
}

func (s *AuthService) Logout(ctx context.Context, actor Actor, sid string) error {
	if err := s.st.RevokeSession(ctx, sid); err != nil {
		return err
	}
	var uid any
	if actor.UserID != nil {
		uid = *actor.UserID
	}
	s.audit.Log(ctx, actor, ActLogout, "user", uid, "")
	return nil
}

func (s *AuthService) loadSession(ctx context.Context, sid string) (*models.User, *models.Session, error) {
	sess, err := s.st.Session(ctx, sid)
	if err != nil || sess.RevokedAt != nil || time.Now().After(sess.ExpiresAt) {
		return nil, nil, Unauthorized("session expired")
	}
	u, err := s.st.UserByID(ctx, sess.UserID)
	if err != nil {
		return nil, nil, Unauthorized("user not found")
	}
	return u, sess, nil
}

func (s *AuthService) issue(u *models.User, sid string, withRefresh bool) (*TokenPair, error) {
	now := time.Now()
	access, err := s.sign(claims{UID: u.ID, SID: sid, Type: "access",
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)), IssuedAt: jwt.NewNumericDate(now)}})
	if err != nil {
		return nil, err
	}
	pair := &TokenPair{AccessToken: access, TokenType: "Bearer", ExpiresIn: int(s.accessTTL.Seconds()), User: *u}
	if withRefresh {
		pair.RefreshToken, err = s.sign(claims{SID: sid, Type: "refresh",
			RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(now.Add(s.refreshTTL)), IssuedAt: jwt.NewNumericDate(now)}})
	}
	return pair, err
}

func (s *AuthService) sign(c claims) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(s.secret)
}

func (s *AuthService) parse(tok, typ string) (*claims, error) {
	c := &claims{}
	t, err := jwt.ParseWithClaims(tok, c, func(*jwt.Token) (any, error) { return s.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !t.Valid || c.Type != typ {
		return nil, errors.New("bad token")
	}
	return c, nil
}

var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("invoice-keeper-timing-guard"), bcrypt.DefaultCost)

func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
