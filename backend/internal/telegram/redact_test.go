package telegram

import (
	"errors"
	"strings"
	"testing"
)

func TestRedactHidesToken(t *testing.T) {
	const tok = "1234567890:AAFakeTokenOnlyForTests_abcdefghijklm" // не настоящий токен
	in := `Post "https://api.telegram.org/bot` + tok + `/getMe": dial tcp 1.2.3.4:443: i/o timeout`
	if out := Redact(in); strings.Contains(out, "AAFake") || !strings.Contains(out, "i/o timeout") {
		t.Fatalf("token not hidden or text lost: %s", out)
	}
	if msg := DescribeError(errors.New(in)); strings.Contains(msg, "AAFake") {
		t.Fatalf("DescribeError leaks token: %s", msg)
	}
}
