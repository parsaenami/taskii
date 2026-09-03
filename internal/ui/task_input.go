package ui

import (
	"fmt"
	"strings"
	"time"

	"taskii/internal/model"
)

type parsedTaskInput struct {
	Title string
	Date  string
	Time  string
	Kind  model.Kind
}

// parseTaskInput recognizes an optional MM-DD and trailing appointment time.
// Dates without a year select their next occurrence, including today, using
// now's local calendar date. The title retains its internal whitespace.
func parseTaskInput(raw string, now time.Time) (parsedTaskInput, error) {
	input := parsedTaskInput{
		Title: strings.TrimSpace(raw),
		Date:  now.Format(dateFormat),
		Kind:  model.KindTask,
	}
	fields := strings.Fields(input.Title)
	if len(fields) == 0 {
		return parsedTaskInput{}, fmt.Errorf("task title cannot be empty")
	}

	last := fields[len(fields)-1]
	if isTimeLike(last) {
		input.Time = last
		input.Kind = model.KindAppointment
		input.Title = strings.TrimSpace(strings.TrimSuffix(input.Title, last))
		fields = fields[:len(fields)-1]
	} else if len(fields) >= 2 && isMonthDayToken(fields[len(fields)-2]) && len(last) == 5 && last[2] == ':' {
		// Keep legacy time-like title text, but an explicitly scheduled
		// appointment must not silently turn into an ordinary task.
		return parsedTaskInput{}, fmt.Errorf("invalid appointment time %q: use HH:MM (00:00–23:59)", last)
	}

	if len(fields) > 0 {
		last = fields[len(fields)-1]
		if isMonthDayToken(last) {
			date, err := nextScheduledDate(last, now)
			if err != nil {
				return parsedTaskInput{}, err
			}
			input.Date = date
			input.Title = strings.TrimSpace(strings.TrimSuffix(input.Title, last))
		}
	}
	if input.Title == "" {
		return parsedTaskInput{}, fmt.Errorf("task title cannot be empty")
	}
	return input, nil
}

func isMonthDayToken(s string) bool {
	if len(s) != 5 || s[2] != '-' {
		return false
	}
	for _, i := range []int{0, 1, 3, 4} {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func nextScheduledDate(monthDay string, now time.Time) (string, error) {
	// A leap year validates the month/day without rejecting February 29.
	date, err := time.Parse(dateFormat, "2000-"+monthDay)
	if err != nil {
		return "", fmt.Errorf("invalid scheduled date %q: use a valid MM-DD", monthDay)
	}
	today := now.Format(dateFormat)
	for year := now.Year(); ; year++ {
		// Compare calendar dates, not instants; UTC avoids normalization of
		// midnight in locations with daylight-saving transitions at midnight.
		candidate := time.Date(year, date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
		if candidate.Month() != date.Month() || candidate.Day() != date.Day() {
			continue // February 29 in a non-leap year.
		}
		if scheduled := candidate.Format(dateFormat); scheduled >= today {
			return scheduled, nil
		}
	}
}
