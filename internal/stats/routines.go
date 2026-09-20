package stats

import (
	"time"

	"github.com/parsaenami/taskii/internal/model"
)

// DayStatus is a view-level state. Pending and future are never persisted.
type DayStatus string

const (
	DayNotDue   DayStatus = "not_due"
	DayFuture   DayStatus = "future"
	DayPending  DayStatus = "pending"
	DayComplete DayStatus = "completed"
	DaySkipped  DayStatus = "skipped"
	DayMissed   DayStatus = "missed"
)

type RoutineDay struct {
	Date   string
	Status DayStatus
}

type RoutineWeek struct {
	RoutineID string
	Days      []RoutineDay // seven calendar days starting on the preferred weekday
}

type RoutineWeekReport struct {
	Start         string
	Routines      []RoutineWeek
	Completed     int
	Skipped       int
	Missed        int
	FollowThrough Progress // Done=completed; Total=completed+missed
}

// ComputeRoutineWeek is deliberately separate from task Compute: task reports
// retain their rolling-week and activity-day semantics. Reconcile historical
// days before calling this function. On dates through LastEvaluatedDate,
// history is authoritative: an absent entry means the routine was not due
// under the schedule in effect then. Unsettled past dates fall back to the
// current schedule and appear missed without mutating the passed-in routines.
func ComputeRoutineWeek(routines []model.Routine, now time.Time, weekStart time.Weekday, workdays []time.Weekday) RoutineWeekReport {
	if weekStart < time.Sunday || weekStart > time.Saturday {
		weekStart = time.Monday
	}
	today, _ := time.Parse(model.DateFormat, now.Format(model.DateFormat))
	start := today.AddDate(0, 0, -(int(today.Weekday())-int(weekStart)+7)%7)
	result := RoutineWeekReport{Start: start.Format(model.DateFormat), Routines: make([]RoutineWeek, 0, len(routines))}
	for _, r := range routines {
		week := RoutineWeek{RoutineID: r.ID, Days: make([]RoutineDay, 0, 7)}
		created := r.CreatedAt.In(now.Location()).Format(model.DateFormat)
		for offset := 0; offset < 7; offset++ {
			date := start.AddDate(0, 0, offset)
			key := date.Format(model.DateFormat)
			status := DayNotDue
			// Reconciliation closes every date through LastEvaluatedDate under
			// the schedule/workdays that were in effect on that date. A history
			// entry proves it was due; the absence of one proves it was not.
			// Never re-evaluate those settled days under today's preferences.
			settled := r.LastEvaluatedDate != "" && key <= r.LastEvaluatedDate
			due := key >= created && (settled && r.History[key] != "" ||
				!settled && model.ScheduledOnDate(r, date, workdays))
			if due {
				switch {
				case key > today.Format(model.DateFormat):
					status = DayFuture
				case r.History[key] == model.RoutineCompleted:
					status = DayComplete
					result.Completed++
				case r.History[key] == model.RoutineSkipped:
					status = DaySkipped
					result.Skipped++
				case key < today.Format(model.DateFormat) || r.History[key] == model.RoutineMissed:
					status = DayMissed
					result.Missed++
				default:
					status = DayPending
				}
			}
			week.Days = append(week.Days, RoutineDay{Date: key, Status: status})
		}
		result.Routines = append(result.Routines, week)
	}
	result.FollowThrough = Progress{Done: result.Completed, Total: result.Completed + result.Missed}
	return result
}
