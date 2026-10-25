package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

const DateLayout = "2006-01-02"

// Date — календарная дата без времени (колонка DATE, JSON "YYYY-MM-DD").
type Date struct{ time.Time }

func NewDate(y int, m time.Month, d int) Date {
	return Date{time.Date(y, m, d, 0, 0, 0, 0, time.UTC)}
}

// DateOf берёт календарную дату из t в её собственном часовом поясе.
func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return NewDate(y, m, d)
}

func ParseDate(s string) (Date, error) {
	t, err := time.Parse(DateLayout, s)
	if err != nil {
		return Date{}, fmt.Errorf("invalid date %q, expected YYYY-MM-DD", s)
	}
	return Date{t}, nil
}

func (d Date) String() string { return d.Format(DateLayout) }

func (d Date) AddDays(n int) Date { return Date{d.Time.AddDate(0, 0, n)} }

// DaysUntil — сколько дней от d до other (отрицательно, если other раньше).
func (d Date) DaysUntil(other Date) int {
	return int(other.Time.Sub(d.Time).Hours() / 24)
}

func (d Date) Before(o Date) bool { return d.Time.Before(o.Time) }
func (d Date) After(o Date) bool  { return d.Time.After(o.Time) }

func (d Date) MarshalJSON() ([]byte, error) { return json.Marshal(d.String()) }

func (d *Date) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	v, err := ParseDate(s)
	if err != nil {
		return err
	}
	*d = v
	return nil
}

func (d *Date) Scan(v any) error {
	switch t := v.(type) {
	case time.Time:
		*d = DateOf(t)
	case string:
		if len(t) >= 10 {
			p, err := ParseDate(t[:10])
			if err != nil {
				return err
			}
			*d = p
		}
	case []byte:
		return d.Scan(string(t))
	case nil:
		*d = Date{}
	default:
		return fmt.Errorf("cannot scan %T into Date", v)
	}
	return nil
}

func (d Date) Value() (driver.Value, error) {
	if d.IsZero() {
		return nil, nil
	}
	return d.String(), nil
}

func (Date) GormDataType() string { return "date" }
