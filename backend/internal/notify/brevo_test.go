package notify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendBrevo(t *testing.T) {
	var got map[string]any
	var key string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key = r.Header.Get("api-key")
		_ = json.NewDecoder(r.Body).Decode(&got)
		if key != "good" {
			w.WriteHeader(401)
			w.Write([]byte(`{"code":"unauthorized","message":"Key not found"}`))
			return
		}
		w.WriteHeader(201)
		w.Write([]byte(`{"messageId":"<1@x>"}`))
	}))
	defer srv.Close()
	old := brevoURL
	brevoURL = srv.URL
	defer func() { brevoURL = old }()

	c := SMTPConfig{Host: "api.brevo.com", Port: 443, User: "me@gmail.com", Password: "good", From: "Invoice Keeper <me@gmail.com>"}
	if err := c.Send("anna@test.ru", "Код", "<b>1</b>"); err != nil {
		t.Fatalf("send: %v", err)
	}
	s := got["sender"].(map[string]any)
	if s["email"] != "me@gmail.com" || s["name"] != "Invoice Keeper" || got["subject"] != "Код" {
		t.Fatalf("body: %v", got)
	}
	c.Password = "bad"
	err := c.Send("anna@test.ru", "Код", "x")
	if err == nil || !strings.Contains(humanMailError(err, c), "API-ключ") {
		t.Fatalf("bad key: %v / %s", err, humanMailError(err, c))
	}
}
