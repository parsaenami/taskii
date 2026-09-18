package ui

import (
	"fmt"
	"time"
)

// isMonthDayToken and nextScheduledDate are shared by parseTaskInput (in
// app.go) for recognizing an optional "MM-DD" scheduled-date annotation.
// Dates without a year select their next occurrence, including today, using
// now's local calendar date.

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
