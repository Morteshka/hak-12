package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/mail"
	"strconv"
	"strings"

	"invoicekeeper/internal/store"
)

// Почтовый сервер (SMTP) можно задать в .env или прямо в панели («Настройки → Почта»).
// Значения из .env важнее. Пароль в базе хранится зашифрованным (AES-GCM, ключ — из JWT_SECRET).
const (
	SettingSMTPHost     = "smtp_host"
	SettingSMTPPort     = "smtp_port"
	SettingSMTPUser     = "smtp_user"
	SettingSMTPPassword = "smtp_password" // зашифрован
	SettingSMTPFrom     = "smtp_from"
)

// SMTPEnv — почтовый сервер из .env.
type SMTPEnv struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
}

// MailConfig — итоговые настройки почты (из .env или из панели).
type MailConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
	FromEnv  bool
}

func (m MailConfig) Configured() bool { return m.Host != "" && m.From != "" }

// EffectiveMail — какой почтовый сервер использовать сейчас.
func EffectiveMail(ctx context.Context, st *store.Store, env ChannelEnv) (MailConfig, error) {
	if env.SMTP.Host != "" {
		e := env.SMTP
		from := e.From
		if from == "" {
			from = e.User
		}
		return MailConfig{Host: e.Host, Port: e.Port, User: e.User, Password: e.Password, From: from, FromEnv: true}, nil
	}
	m, err := st.Settings(ctx)
	if err != nil {
		return MailConfig{}, err
	}
	return mailFromSettings(m, env.LinkSecret), nil
}

func mailFromSettings(m map[string]string, secret string) MailConfig {
	port, _ := strconv.Atoi(m[SettingSMTPPort])
	if port == 0 {
		port = 465
	}
	pw, _ := decryptSecret(secret, m[SettingSMTPPassword])
	from := m[SettingSMTPFrom]
	if from == "" {
		from = m[SettingSMTPUser]
	}
	return MailConfig{Host: m[SettingSMTPHost], Port: port, User: m[SettingSMTPUser], Password: pw, From: from}
}

// MailSettings — что видит администратор в «Настройки → Почта» (пароль не отдаётся).
type MailSettings struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	User        string `json:"user"`
	From        string `json:"from"`
	HasPassword bool   `json:"has_password"`
	FromEnv     bool   `json:"from_env"`
	Configured  bool   `json:"configured"`
}

// SMTPInput — PATCH /settings {smtp: {...}}. Host "" — отключить почту.
// Password nil или "" — оставить сохранённый пароль.
type SMTPInput struct {
	Host     string  `json:"host"`
	Port     int     `json:"port"`
	User     string  `json:"user"`
	Password *string `json:"password"`
	From     string  `json:"from"`
}

func (s *SettingsService) mailSettings(ctx context.Context) (*MailSettings, error) {
	mc, err := EffectiveMail(ctx, s.st, s.env)
	if err != nil {
		return nil, err
	}
	return &MailSettings{Host: mc.Host, Port: mc.Port, User: mc.User, From: mc.From, HasPassword: mc.Password != "", FromEnv: mc.FromEnv, Configured: mc.Configured()}, nil
}

func (s *SettingsService) saveSMTP(ctx context.Context, in *SMTPInput) (D, error) {
	if s.env.SMTP.Host != "" {
		return nil, Invalid("smtp is set in .env")
	}
	host := strings.ToLower(strings.TrimSpace(in.Host))
	if host == "" { // отключить почту
		for _, k := range []string{SettingSMTPHost, SettingSMTPPort, SettingSMTPUser, SettingSMTPPassword, SettingSMTPFrom} {
			if err := s.st.SetSetting(ctx, k, ""); err != nil {
				return nil, err
			}
		}
		return D{"smtp": "disabled"}, nil
	}
	if len(host) > 253 || strings.ContainsAny(host, " /:@") {
		return nil, Invalid("smtp host is invalid")
	}
	if in.Port < 1 || in.Port > 65535 {
		return nil, Invalid("smtp port is invalid")
	}
	user := strings.TrimSpace(in.User)
	from := strings.TrimSpace(in.From)
	if from == "" {
		from = user
	}
	if a, err := mail.ParseAddress(from); err != nil || a.Address == "" {
		return nil, Invalid("smtp from must be an email address")
	}
	vals := map[string]string{SettingSMTPHost: host, SettingSMTPPort: strconv.Itoa(in.Port), SettingSMTPUser: user, SettingSMTPFrom: from}
	if in.Password != nil && *in.Password != "" {
		enc, err := encryptSecret(s.env.LinkSecret, *in.Password)
		if err != nil {
			return nil, err
		}
		vals[SettingSMTPPassword] = enc
	}
	for k, v := range vals {
		if err := s.st.SetSetting(ctx, k, v); err != nil {
			return nil, err
		}
	}
	return D{"smtp_host": host, "smtp_port": in.Port, "smtp_user": user, "smtp_from": from, "smtp_password_changed": vals[SettingSMTPPassword] != ""}, nil
}

// --- шифрование секретов в app_settings ---

func secretKey(secret string) []byte {
	k := sha256.Sum256([]byte("invoice-keeper/smtp:" + secret))
	return k[:]
}

func encryptSecret(secret, plain string) (string, error) {
	block, err := aes.NewCipher(secretKey(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return "v1:" + base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(plain), nil)), nil
}

func decryptSecret(secret, enc string) (string, error) {
	if enc == "" {
		return "", nil
	}
	if !strings.HasPrefix(enc, "v1:") {
		return "", errors.New("unknown secret format")
	}
	raw, err := base64.StdEncoding.DecodeString(enc[3:])
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(secretKey(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("secret is too short")
	}
	out, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
