package notify

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"invoicekeeper/internal/models"
)

func TestWebhookDeliveryResult(t *testing.T) {
	var gotSig, gotEvent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotSig = r.Header.Get("X-InvoiceKeeper-Signature")
		gotEvent = r.Header.Get("X-InvoiceKeeper-Event")
		if strings.Contains(string(b), `"missing"`) {
			http.Error(w, "no such hook", http.StatusNotFound)
			return
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	h := &Hub{cfg: Config{WebhookSecret: "s3cret"}, client: &http.Client{Timeout: 2 * time.Second}}
	ev := models.PaymentEvent{Event: models.EventPaymentPaid, PaymentID: 7, Name: "Аренда"}
	d, err := h.deliverWebhook(context.Background(), srv.URL+"/hook", ev)
	if err != nil || !d.OK || d.StatusCode != 200 || d.Response != `{"ok":true}` {
		t.Fatalf("ok delivery: %+v %v", d, err)
	}
	if gotEvent != "payment.paid" || gotSig != Sign("s3cret", []byte(d.Payload)) {
		t.Fatalf("headers: event=%q sig=%q", gotEvent, gotSig)
	}
	if d.PaymentID == nil || *d.PaymentID != 7 {
		t.Fatalf("payment id: %v", d.PaymentID)
	}

	ev.Name = "missing"
	d, err = h.deliverWebhook(context.Background(), srv.URL+"/hook", ev)
	if err == nil || d.OK || d.StatusCode != 404 || !strings.Contains(d.Error, "404") {
		t.Fatalf("404 delivery: %+v %v", d, err)
	}

	d, err = h.deliverWebhook(context.Background(), "http://127.0.0.1:1/hook", ev)
	if err == nil || d.OK || d.StatusCode != 0 || d.Error == "" {
		t.Fatalf("refused delivery: %+v %v", d, err)
	}
}

func TestWebhookEchoRoute(t *testing.T) {
	h := &Hub{cfg: Config{InternalURL: "http://backend:8080/"}}
	if got := h.route("https://site.example/api/v1/webhook/echo"); got != "http://backend:8080/api/v1/webhook/echo" {
		t.Fatalf("echo route: %s", got)
	}
	if got := h.route("https://hook.example/x"); got != "https://hook.example/x" {
		t.Fatalf("other route: %s", got)
	}
	if got := (&Hub{}).route("https://site.example/api/v1/webhook/echo"); got != "https://site.example/api/v1/webhook/echo" {
		t.Fatalf("no internal url: %s", got)
	}
}
