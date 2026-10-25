package service

import (
	"testing"
	"time"
)

func TestTelegramLinkToken(t *testing.T) {
	now := time.Now()
	tok := TelegramLinkToken("secret-secret-123", 42, now)
	if len(tok) > 64 {
		t.Fatalf("token too long for Telegram: %d", len(tok))
	}
	for _, r := range tok {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			t.Fatalf("bad char %q in %s", r, tok)
		}
	}
	if id, ok := ParseTelegramLinkToken("secret-secret-123", tok, now); !ok || id != 42 {
		t.Fatalf("parse: %v %v", id, ok)
	}
	// подпись бывает с «_» внутри — такие ссылки тоже должны работать
	for uid := uint(1); uid <= 3000; uid++ {
		tk := TelegramLinkToken("secret-secret-123", uid, now)
		if id, ok := ParseTelegramLinkToken("secret-secret-123", tk, now); !ok || id != uid {
			t.Fatalf("parse %s: %v %v", tk, id, ok)
		}
	}
	if _, ok := ParseTelegramLinkToken("other-secret-1234", tok, now); ok {
		t.Fatal("accepted with wrong secret")
	}
	if _, ok := ParseTelegramLinkToken("secret-secret-123", tok, now.Add(25*time.Hour)); ok {
		t.Fatal("accepted expired token")
	}
	forged := "L43" + tok[3:]
	if _, ok := ParseTelegramLinkToken("secret-secret-123", forged, now); ok {
		t.Fatal("accepted forged id")
	}
}
