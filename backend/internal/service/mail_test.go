package service

import "testing"

func TestSecretRoundTrip(t *testing.T) {
	enc, err := encryptSecret("jwt-secret", "app-password-123")
	if err != nil || enc == "" || enc == "app-password-123" {
		t.Fatalf("encrypt: %q %v", enc, err)
	}
	if got, err := decryptSecret("jwt-secret", enc); err != nil || got != "app-password-123" {
		t.Fatalf("decrypt: %q %v", got, err)
	}
	if _, err := decryptSecret("other-secret", enc); err == nil {
		t.Fatal("decrypt with another key must fail")
	}
	if got, _ := decryptSecret("jwt-secret", ""); got != "" {
		t.Fatal("empty secret")
	}
}

func TestMailFromSettings(t *testing.T) {
	enc, _ := encryptSecret("k", "pw")
	mc := mailFromSettings(map[string]string{SettingSMTPHost: "smtp.yandex.ru", SettingSMTPUser: "a@yandex.ru", SettingSMTPPassword: enc}, "k")
	if !mc.Configured() || mc.Port != 465 || mc.From != "a@yandex.ru" || mc.Password != "pw" {
		t.Fatalf("mail config: %+v", mc)
	}
	if (MailConfig{}).Configured() {
		t.Fatal("empty config must be off")
	}
}

func TestCodeHashIsCaseInsensitiveByEmail(t *testing.T) {
	s := &AuthService{secret: []byte("x")}
	if s.codeHash("Anna@Mail.ru", "123456") != s.codeHash("anna@mail.ru", "123456") {
		t.Fatal("email case must not matter")
	}
	if s.codeHash("anna@mail.ru", "123456") == s.codeHash("anna@mail.ru", "123457") {
		t.Fatal("different codes must differ")
	}
}
