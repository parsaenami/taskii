package model

import "time"

// Kind distinguishes a plain task from an appointment. Appointments are the
// only entries that carry a Time; a "" Kind in old saved data (before this
// field existed) is treated as KindTask by IsAppointment below, so existing
// task lists keep working unmodified.
type Kind string

const (
	KindTask        Kind = "task"
	KindAppointment Kind = "appointment"
)

type Task struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Done      bool       `json:"done"`
	Important bool       `json:"important,omitempty"`
	Kind      Kind       `json:"kind,omitempty"`
	Date      string     `json:"date"` // YYYY-MM-DD
	Time      string     `json:"time"` // HH:MM; only appointments set this
	CreatedAt time.Time  `json:"created_at"`
	DoneAt    *time.Time `json:"done_at,omitempty"`
}

func (t Task) IsAppointment() bool {
	return t.Kind == KindAppointment
}
