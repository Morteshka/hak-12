package service

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"invoicekeeper/internal/models"
	"invoicekeeper/internal/store"
)

// ---------- машина времени (демо) ----------
//
// Администратор может «перемотать» системную дату на N дней вперёд, чтобы показать напоминания
// за 7/3/1 день, просрочку, конец пробного периода и автопродление вживую, не дожидаясь неделю.
// Сдвиг хранится в app_settings: его видят и API, и бот (отдельный процесс) — через кэш на 3 секунды.
// Все сроки, статусы и ступени напоминаний считаются от PaymentService.Today(), поэтому сдвиг
// действует на всю систему сразу. Выключается переменной DEMO_TIME_TRAVEL=false.

const (
	SettingTimeOffset = "demo_time_offset_days"
	maxTimeOffset     = 730
	clockCacheTTL     = 3 * time.Second
)

type clockCache struct {
	mu     sync.Mutex
	offset int
	at     time.Time
}

// TimeState — что показывает интерфейс: «сегодня» в системе, реальная дата и сдвиг.
type TimeState struct {
	Today      string `json:"today"`
	RealToday  string `json:"real_today"`
	OffsetDays int    `json:"offset_days"`
	Enabled    bool   `json:"enabled"` // машина времени разрешена (DEMO_TIME_TRAVEL)
}

// EnableTimeTravel включает машину времени (по умолчанию выключена — сдвиг всегда 0).
func (s *PaymentService) EnableTimeTravel(on bool) { s.timeTravel = on }

// Offset — текущий сдвиг в днях (из БД, с кэшем).
func (s *PaymentService) Offset() int {
	if !s.timeTravel || s.st == nil {
		return 0
	}
	s.clk.mu.Lock()
	defer s.clk.mu.Unlock()
	if !s.clk.at.IsZero() && time.Since(s.clk.at) < clockCacheTTL {
		return s.clk.offset
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	v, err := s.st.Setting(ctx, SettingTimeOffset)
	switch {
	case err == nil:
		n, _ := strconv.Atoi(v)
		s.clk.offset = min(max(n, 0), maxTimeOffset)
	case errors.Is(err, store.ErrNotFound):
		s.clk.offset = 0
	} // при ошибке БД оставляем прежнее значение
	s.clk.at = time.Now()
	return s.clk.offset
}

// RealToday — настоящая дата без сдвига.
func (s *PaymentService) RealToday() models.Date { return models.DateOf(time.Now().In(s.loc)) }

// TimeState — состояние машины времени.
func (s *PaymentService) TimeState() TimeState {
	return TimeState{Today: s.Today().String(), RealToday: s.RealToday().String(), OffsetDays: s.Offset(), Enabled: s.timeTravel}
}

// SetTimeOffset — администратор задаёт сдвиг даты. 0 — вернуться в настоящее.
// Шаг назад снимает отметки «напоминание отправлено» у неоплаченных периодов, чтобы демо можно было повторить.
func (s *PaymentService) SetTimeOffset(ctx context.Context, a Actor, days int) (TimeState, error) {
	if err := requireAdmin(a); err != nil {
		return TimeState{}, err
	}
	if !s.timeTravel {
		return TimeState{}, Forbidden("time travel is disabled (DEMO_TIME_TRAVEL=false)")
	}
	if days < 0 || days > maxTimeOffset {
		return TimeState{}, Invalid("offset_days must be between 0 and 730")
	}
	prev := s.Offset()
	if err := s.st.SetSetting(ctx, SettingTimeOffset, strconv.Itoa(days)); err != nil {
		return TimeState{}, err
	}
	s.clk.mu.Lock()
	s.clk.offset, s.clk.at = days, time.Now()
	s.clk.mu.Unlock()
	if days < prev {
		if err := s.st.ResetUnpaidReminders(ctx); err != nil {
			return TimeState{}, err
		}
	}
	if err := s.EnsureAll(ctx); err != nil { // новые периоды до нового горизонта (автопродление)
		return TimeState{}, err
	}
	s.audit.Log(ctx, a, ActTimeTravel, "system", nil, D{"from_days": prev, "to_days": days, "today": s.Today().String()})
	s.kick() // напоминания уходят сразу
	return s.TimeState(), nil
}
