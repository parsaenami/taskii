package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RoutineSchedule describes when a routine is due. Custom weekdays are stored
// as Go weekday numbers (Sunday=0 through Saturday=6).
type RoutineSchedule string

const (
	ScheduleEveryDay RoutineSchedule = "every_day"
	ScheduleWorkdays RoutineSchedule = "workdays"
	ScheduleCustom   RoutineSchedule = "custom"
)

type RoutineStatus string

const (
	RoutineCompleted RoutineStatus = "completed"
	RoutineSkipped   RoutineStatus = "skipped"
	RoutineMissed    RoutineStatus = "missed"
)

type Routine struct {
	ID                string                   `json:"id"`
	Title             string                   `json:"title"`
	CreatedAt         time.Time                `json:"created_at"`
	Schedule          RoutineSchedule          `json:"schedule"`
	CustomWeekdays    []time.Weekday           `json:"custom_weekdays,omitempty"`
	History           map[string]RoutineStatus `json:"history,omitempty"`
	LastEvaluatedDate string                   `json:"last_evaluated_date,omitempty"`
}

// ValidateWeekdays rejects empty, repeated and out-of-range weekday sets.
// A nil preference is handled separately by Settings.EffectiveWorkdays.
func ValidateWeekdays(days []time.Weekday) error {
	if len(days) == 0 {
		return errors.New("at least one weekday is required")
	}
	var seen [7]bool
	for _, day := range days {
		if day < time.Sunday || day > time.Saturday {
			return fmt.Errorf("invalid weekday %d", day)
		}
		if seen[day] {
			return fmt.Errorf("repeated weekday %d", day)
		}
		seen[day] = true
	}
	return nil
}

func validDate(date string) bool {
	d, err := time.Parse(DateFormat, date)
	return err == nil && d.Format(DateFormat) == date
}

func (r Routine) Validate() error {
	if strings.TrimSpace(r.ID) == "" || strings.TrimSpace(r.Title) == "" || r.CreatedAt.IsZero() {
		return errors.New("routine requires id, title and created_at")
	}
	switch r.Schedule {
	case ScheduleEveryDay, ScheduleWorkdays:
		if len(r.CustomWeekdays) != 0 {
			return errors.New("custom_weekdays requires custom schedule")
		}
	case ScheduleCustom:
		if err := ValidateWeekdays(r.CustomWeekdays); err != nil {
			return fmt.Errorf("custom_weekdays: %w", err)
		}
	default:
		return fmt.Errorf("invalid routine schedule %q", r.Schedule)
	}
	if r.LastEvaluatedDate != "" && !validDate(r.LastEvaluatedDate) {
		return errors.New("invalid last_evaluated_date")
	}
	for date, status := range r.History {
		if !validDate(date) {
			return fmt.Errorf("invalid history date %q", date)
		}
		switch status {
		case RoutineCompleted, RoutineSkipped, RoutineMissed:
		default:
			return fmt.Errorf("invalid history status %q", status)
		}
	}
	return nil
}

func ValidateRoutines(routines []Routine) error {
	seen := make(map[string]bool, len(routines))
	for i, r := range routines {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("routine %d: %w", i, err)
		}
		if seen[r.ID] {
			return fmt.Errorf("duplicate routine id %q", r.ID)
		}
		seen[r.ID] = true
	}
	return nil
}

// ScheduledOnDate answers only the recurrence question; creation and history
// are independent. Pass the workday preference in explicitly, including when
// reconciling an old schedule before changing preferences.
func ScheduledOnDate(r Routine, date time.Time, workdays []time.Weekday) bool {
	switch r.Schedule {
	case ScheduleEveryDay:
		return true
	case ScheduleWorkdays:
		for _, day := range workdays {
			if day == date.Weekday() {
				return true
			}
		}
	case ScheduleCustom:
		for _, day := range r.CustomWeekdays {
			if day == date.Weekday() {
				return true
			}
		}
	}
	return false
}

// ReconcileRoutine closes all days before today's local calendar date. Existing
// decisions (including a previously recorded miss) are never overwritten.
// The cursor advances across unscheduled days so future runs do not rescan them.
// Pass the OLD workdays when changing calendar preferences, then save the new
// preference; otherwise an unprocessed day can be judged by the new schedule.
func ReconcileRoutine(r *Routine, today time.Time, workdays []time.Weekday) (bool, error) {
	if r == nil {
		return false, errors.New("nil routine")
	}
	if err := r.Validate(); err != nil {
		return false, err
	}
	if workdays == nil {
		workdays = defaultWorkdays
	}
	if err := ValidateWeekdays(workdays); err != nil {
		return false, fmt.Errorf("workdays: %w", err)
	}
	// UTC midnights are date-only counters, immune to local DST transitions.
	loc := today.Location()
	created := r.CreatedAt.In(loc).Format(DateFormat)
	start, _ := time.Parse(DateFormat, created)
	if r.LastEvaluatedDate != "" {
		last, _ := time.Parse(DateFormat, r.LastEvaluatedDate)
		if !last.Before(start) {
			start = last.AddDate(0, 0, 1)
		}
	}
	yesterday, _ := time.Parse(DateFormat, today.In(loc).AddDate(0, 0, -1).Format(DateFormat))
	if start.After(yesterday) {
		return false, nil
	}
	if r.History == nil {
		r.History = make(map[string]RoutineStatus)
	}
	for day := start; !day.After(yesterday); day = day.AddDate(0, 0, 1) {
		key := day.Format(DateFormat)
		if ScheduledOnDate(*r, day, workdays) && r.History[key] == "" {
			r.History[key] = RoutineMissed
		}
	}
	r.LastEvaluatedDate = yesterday.Format(DateFormat)
	return true, nil
}

func LoadRoutines() ([]Routine, error) {
	path, err := dataPath("routines.json")
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []Routine{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return []Routine{}, nil
	}
	var routines []Routine
	if err := json.Unmarshal(b, &routines); err != nil {
		return nil, err
	}
	if routines == nil {
		return nil, errors.New("routines.json must be an array")
	}
	if err := ValidateRoutines(routines); err != nil {
		return nil, err
	}
	return routines, nil
}

func SaveRoutines(routines []Routine) error {
	if err := ValidateRoutines(routines); err != nil {
		return err
	}
	if routines == nil {
		routines = []Routine{}
	}
	path, err := dataPath("routines.json")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(routines, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(path, b)
}
