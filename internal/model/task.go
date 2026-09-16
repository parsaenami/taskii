package model

import "time"

// DateFormat is the layout every stored date uses, and the single definition
// the whole app shares — Go layouts are written as the reference date
// Jan 2 15:04:05 2006 MST, where each component is a distinct number, so a
// typo like "2006-01-04" silently renders MINUTES where the day belongs
// rather than failing. Declaring it once means that mistake can only be made
// in one place.
const DateFormat = "2006-01-02"

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
	ID        string `json:"id"`
	Title     string `json:"title"`
	Done      bool   `json:"done"`
	Important bool   `json:"important,omitempty"`
	Kind      Kind   `json:"kind,omitempty"`
	Date      string `json:"date"` // YYYY-MM-DD
	Time      string `json:"time"` // HH:MM; only appointments set this

	// EndTime is the closing HH:MM of an appointment entered as a range
	// ("standup 09:00-09:15"). Empty for a point-in-time appointment.
	//
	// Kept separate from Time rather than packing "09:00-09:15" into it,
	// because Time is the sort key for the day's schedule in several places
	// — keeping it a bare HH:MM means that ordering, and every isTimeLike
	// check, stays correct without special-casing a range.
	EndTime string `json:"end_time,omitempty"`

	// Tags are the "#tag" labels entered anywhere in the task text, stored
	// WITHOUT the leading "#". They read as projects: a task belongs to zero
	// or more of them, and they're rendered in one shared color.
	Tags      []string   `json:"tags,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	DoneAt    *time.Time `json:"done_at,omitempty"`

	// OriginalDate is the date this task was FIRST scheduled for, recorded
	// only when it gets migrated out of the Overdue list. Date itself has to
	// be rewritten to today for the task to appear in Today's list, so
	// without this the original scheduling date — and with it the task's
	// real age — would be destroyed by the very act of migrating.
	//
	// Empty means "never migrated", which is also what every task saved
	// before this field existed decodes to, so old data keeps working. Its
	// presence IS the migrated flag (see IsMigrated); a separate bool could
	// drift out of sync with the date it describes.
	OriginalDate string `json:"original_date,omitempty"`

	// DueDate is the deadline entered as "!Nd" (N days from when it was
	// typed), stored as an absolute YYYY-MM-DD rather than the relative
	// offset. The countdown the user sees is recomputed from it each day, so
	// "!2d" entered today shows "!1d" tomorrow without anything having to
	// rewrite the task. Storing the offset instead would freeze the number.
	//
	// A task with a due date stays in Today's list until its deadline passes
	// — and beyond, since a missed deadline is exactly what most needs
	// looking at — so this is the one field that overrides Date for list
	// membership. Empty means no deadline.
	DueDate string `json:"due_date,omitempty"`
}

// IsMigrated reports whether this task was carried forward from an earlier
// day rather than scheduled for the day it currently sits on.
func (t Task) IsMigrated() bool { return t.OriginalDate != "" }

// HasDueDate reports whether a deadline was set on this task.
func (t Task) HasDueDate() bool { return t.DueDate != "" }

// DaysUntilDue is how many days remain before the deadline: 0 on the due date
// itself, positive while it's still ahead, negative once it has passed. The
// second return is false when there's no due date at all, which callers must
// check — 0 is a meaningful value here ("due today"), not a missing one.
//
// Counts CALENDAR days for the same reason AgeDays does: a deadline set for
// tomorrow reads "1d" from the moment it's entered until midnight, with no
// 24-hour threshold anywhere.
func (t Task) DaysUntilDue(now time.Time) (int, bool) {
	if t.DueDate == "" {
		return 0, false
	}
	d, err := time.ParseInLocation(DateFormat, t.DueDate, time.UTC)
	if err != nil {
		return 0, false
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return int(d.Sub(today) / (24 * time.Hour)), true
}

// IsOverdueBy reports how many days a deadline has been missed by, and
// whether it has been missed at all. Kept separate from DaysUntilDue so
// callers reading "how late is this" don't have to negate a countdown.
func (t Task) IsOverdueBy(now time.Time) (int, bool) {
	days, ok := t.DaysUntilDue(now)
	if !ok || days >= 0 {
		return 0, false
	}
	return -days, true
}

// AgeDays is how many days stale this task is, measured from the date it was
// FIRST scheduled for — so deferring a task repeatedly keeps growing its age
// instead of resetting the counter every time it's carried forward. Returns
// 0 when the scheduling date is unparseable or in the future.
func (t Task) AgeDays(now time.Time) int {
	from := t.OriginalDate
	if from == "" {
		from = t.Date
	}
	d, err := time.ParseInLocation(DateFormat, from, time.UTC)
	if err != nil {
		return 0
	}
	// This counts CALENDAR days, not elapsed hours: a task dated yesterday
	// is 1d old at one minute past midnight, not after a 24-hour wait.
	//
	// Both sides are UTC midnights — now's wall-clock date, re-pinned to UTC
	// — so their difference is an exact whole number of 24-hour days and the
	// division can't truncate. Mixing zones is what caused this to read one
	// day short: a UTC-parsed date minus a LOCAL midnight left a 3.5h
	// remainder at UTC+3:30, which floored yesterday's task to "0d". Using
	// real local midnights instead would reintroduce the same class of bug
	// at a DST boundary, where a calendar day is 23 or 25 hours long.
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	days := int(today.Sub(d) / (24 * time.Hour))
	if days < 0 {
		return 0
	}
	return days
}

func (t Task) IsAppointment() bool {
	return t.Kind == KindAppointment
}
