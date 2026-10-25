package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// sign собирает initData так же, как Telegram.
func sign(token string, vals url.Values) string {
	keys := make([]string, 0, len(vals))
	for k := range vals {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var lines []string
	for _, k := range keys {
		lines = append(lines, k+"="+vals.Get(k))
	}
	sec := hmac.New(sha256.New, []byte("WebAppData"))
	sec.Write([]byte(token))
	m := hmac.New(sha256.New, sec.Sum(nil))
	m.Write([]byte(strings.Join(lines, "\n")))
	vals.Set("hash", hex.EncodeToString(m.Sum(nil)))
	return vals.Encode()
}

func TestVerifyTelegramInitData(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	tok := "123456:ABC-test"
	v := url.Values{}
	v.Set("auth_date", strconv.FormatInt(now.Add(-time.Hour).Unix(), 10))
	v.Set("query_id", "AAH")
	v.Set("user", `{"id":7001,"first_name":"Алиса","username":"alice_tg"}`)
	data := sign(tok, v)
	u, err := VerifyTelegramInitData(tok, data, 24*time.Hour, now)
	if err != nil || u.ID != 7001 || u.Username != "alice_tg" {
		t.Fatalf("valid data rejected: %v %+v", err, u)
	}
	if _, err := VerifyTelegramInitData("999:other", data, 24*time.Hour, now); err == nil {
		t.Fatal("accepted with another bot token")
	}
	forged := strings.Replace(data, "7001", "7002", 1)
	if _, err := VerifyTelegramInitData(tok, forged, 24*time.Hour, now); err == nil {
		t.Fatal("accepted forged user id")
	}
	if _, err := VerifyTelegramInitData(tok, data, 24*time.Hour, now.Add(48*time.Hour)); err == nil {
		t.Fatal("accepted expired data")
	}
	if _, err := VerifyTelegramInitData("", data, 24*time.Hour, now); err == nil {
		t.Fatal("accepted without bot token")
	}
}
