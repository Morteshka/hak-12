package middleware

import (
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	now := time.Unix(1000, 0)
	l := &Limiter{max: 3, window: time.Minute, hits: map[string][]time.Time{}, now: func() time.Time { return now }}
	for i := 0; i < 3; i++ {
		if b, _ := l.Blocked("Alice"); b {
			t.Fatalf("blocked too early at %d", i)
		}
		l.Hit("alice ")
	}
	if b, wait := l.Blocked("ALICE"); !b || wait != time.Minute {
		t.Fatalf("want blocked for 1m, got %v %v", b, wait)
	}
	now = now.Add(61 * time.Second)
	if b, _ := l.Blocked("alice"); b {
		t.Fatal("window should have passed")
	}
	l.Hit("bob")
	l.Reset("BOB")
	if len(l.hits) != 0 {
		t.Fatalf("reset/prune left %v", l.hits)
	}
}
