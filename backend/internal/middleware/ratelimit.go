package middleware

import (
	"strings"
	"sync"
	"time"
)

// Limiter — счётчик неудачных попыток в скользящем окне (в памяти процесса).
// Защищает вход и регистрацию от подбора пароля и массового создания аккаунтов.
type Limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	hits   map[string][]time.Time
	now    func() time.Time
}

func NewLimiter(max int, window time.Duration) *Limiter {
	l := &Limiter{max: max, window: window, hits: map[string][]time.Time{}, now: time.Now}
	go l.gc()
	return l
}

func norm(key string) string { return strings.ToLower(strings.TrimSpace(key)) }

// prune оставляет только попытки внутри окна. Вызывать под мьютексом.
func (l *Limiter) prune(key string, now time.Time) []time.Time {
	ts := l.hits[key]
	i := 0
	for i < len(ts) && now.Sub(ts[i]) >= l.window {
		i++
	}
	ts = ts[i:]
	if len(ts) == 0 {
		delete(l.hits, key)
	} else {
		l.hits[key] = ts
	}
	return ts
}

// Blocked — исчерпан ли лимит; второе значение — через сколько можно снова.
func (l *Limiter) Blocked(key string) (bool, time.Duration) {
	key = norm(key)
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	ts := l.prune(key, now)
	if len(ts) < l.max {
		return false, 0
	}
	return true, l.window - now.Sub(ts[0])
}

// Hit — записать попытку.
func (l *Limiter) Hit(key string) {
	key = norm(key)
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hits[key] = append(l.prune(key, l.now()), l.now())
}

// Reset — забыть попытки (успешный вход).
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	delete(l.hits, norm(key))
	l.mu.Unlock()
}

func (l *Limiter) gc() {
	t := time.NewTicker(l.window)
	for range t.C {
		l.mu.Lock()
		now := l.now()
		for k := range l.hits {
			l.prune(k, now)
		}
		l.mu.Unlock()
	}
}
