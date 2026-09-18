package stats

import (
	"testing"
	"time"

	"taskii/internal/model"
)

func TestScheduledTasksEnterReportsOnTheirDate(t *testing.T) {
	now := time.Date(2026, time.September, 3, 12, 0, 0, 0, time.UTC)
	tasks := []model.Task{
		{Date: "2026-09-03", Done: true},
		{Date: "2026-09-04"},
		{Date: "2026-09-04", Done: true},
	}

	before := Compute(tasks, now)
	for name, progress := range map[string]Progress{
		"today": before.Today, "week": before.Week, "month": before.Month,
	} {
		if progress != (Progress{Done: 1, Total: 1}) {
			t.Errorf("%s includes future tasks: %+v", name, progress)
		}
	}
	for _, bars := range [][]DayBar{before.WeekBars, before.MonthBars} {
		for _, bar := range bars {
			if bar.Date.Format(dateFormat) > "2026-09-03" {
				t.Errorf("future date included in reports: %v", bar.Date)
			}
		}
	}

	after := Compute(tasks, now.AddDate(0, 0, 1))
	if after.Today != (Progress{Done: 1, Total: 2}) {
		t.Errorf("scheduled tasks missing on their date: %+v", after.Today)
	}
	if after.Week != (Progress{Done: 2, Total: 3}) || after.Month != after.Week {
		t.Errorf("wrong totals after scheduled date: week=%+v month=%+v", after.Week, after.Month)
	}
}
