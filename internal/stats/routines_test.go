package stats

import (
	"reflect"
	"testing"
	"time"

	"github.com/parsaenami/taskii/internal/model"
)

func TestRoutineWeekSundayAndMondayBoundaries(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC) // Sunday
	r := model.Routine{ID: "daily", Title: "Read", CreatedAt: time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC), Schedule: model.ScheduleEveryDay,
		History: map[string]model.RoutineStatus{"2026-09-17": model.RoutineCompleted, "2026-09-18": model.RoutineSkipped, "2026-09-20": model.RoutineCompleted}}
	workdays := []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}
	report := ComputeRoutineWeek([]model.Routine{r}, now, time.Monday, workdays)
	if report.Start != "2026-09-14" || report.Completed != 2 || report.Skipped != 1 || report.Missed != 2 || report.FollowThrough != (Progress{Done: 2, Total: 4}) {
		t.Fatalf("Monday report = %+v", report)
	}
	want := []DayStatus{DayNotDue, DayNotDue, DayMissed, DayComplete, DaySkipped, DayMissed, DayComplete}
	var got []DayStatus
	for _, day := range report.Routines[0].Days {
		got = append(got, day.Status)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("statuses = %v, want %v", got, want)
	}
	report = ComputeRoutineWeek([]model.Routine{r}, now, time.Sunday, workdays)
	if report.Start != "2026-09-20" || report.Completed != 1 || report.Missed != 0 || report.Routines[0].Days[1].Status != DayFuture {
		t.Fatalf("Sunday week = %+v", report)
	}
	report = ComputeRoutineWeek([]model.Routine{r}, now.AddDate(0, 0, -1), time.Monday, workdays)
	if report.Routines[0].Days[5].Status != DayPending || report.Routines[0].Days[6].Status != DayFuture {
		t.Fatalf("Saturday pending and Sunday future = %+v", report.Routines[0].Days)
	}
}

func TestRoutineWeekWorkdaysPreferenceDoesNotChangeTaskCompute(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	r := model.Routine{ID: "weekend", Title: "Hike", CreatedAt: now.AddDate(0, 0, -7), Schedule: model.ScheduleWorkdays}
	report := ComputeRoutineWeek([]model.Routine{r}, now, time.Sunday, []time.Weekday{time.Saturday, time.Sunday})
	if report.Routines[0].Days[0].Status != DayPending || report.Routines[0].Days[1].Status != DayNotDue || report.Routines[0].Days[6].Status != DayFuture {
		t.Fatalf("weekend preference = %+v", report.Routines[0].Days)
	}
	if taskReport := Compute(nil, now); len(taskReport.WeekBars) != 7 {
		t.Fatalf("task rolling week changed: %+v", taskReport)
	}
}

func TestRoutineWeekCustomWeekStart(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC) // Sunday
	r := model.Routine{
		ID: "daily", Title: "Read", CreatedAt: now.AddDate(0, 0, -10),
		Schedule: model.ScheduleEveryDay,
		History:  map[string]model.RoutineStatus{"2026-09-16": model.RoutineCompleted},
	}
	report := ComputeRoutineWeek([]model.Routine{r}, now, time.Wednesday, nil)
	if report.Start != "2026-09-16" {
		t.Fatalf("Wednesday report start = %s", report.Start)
	}
	if got := report.Routines[0].Days[0]; got.Date != "2026-09-16" || got.Status != DayComplete {
		t.Fatalf("first day = %+v", got)
	}
	if got := report.Routines[0].Days[6]; got.Date != "2026-09-22" || got.Status != DayFuture {
		t.Fatalf("last day = %+v", got)
	}
}

func TestRoutineWeekPreservesSettledOldSchedule(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC) // Thursday
	r := model.Routine{
		ID: "changed", Title: "Exercise", CreatedAt: now.AddDate(0, 0, -7),
		Schedule: model.ScheduleCustom, CustomWeekdays: []time.Weekday{time.Thursday},
		LastEvaluatedDate: "2026-09-23",
		History: map[string]model.RoutineStatus{
			"2026-09-21": model.RoutineCompleted, // previously due on Monday
			"2026-09-22": model.RoutineSkipped,   // previously due on Tuesday
			"2026-09-23": model.RoutineMissed,    // previously due on Wednesday
		},
	}
	report := ComputeRoutineWeek([]model.Routine{r}, now, time.Monday, nil)
	want := []DayStatus{DayComplete, DaySkipped, DayMissed, DayPending, DayNotDue, DayNotDue, DayNotDue}
	for i, day := range report.Routines[0].Days {
		if day.Status != want[i] {
			t.Errorf("day %s: got %s, want %s", day.Date, day.Status, want[i])
		}
	}
	if report.Completed != 1 || report.Skipped != 1 || report.Missed != 1 || report.FollowThrough != (Progress{Done: 1, Total: 2}) {
		t.Fatalf("settled counts changed: %+v", report)
	}

	// The same rule applies when Workdays change, even if the routine's own
	// recurrence remains workdays: no old day is reclassified by the new set.
	r.Schedule = model.ScheduleWorkdays
	r.CustomWeekdays = nil
	report = ComputeRoutineWeek([]model.Routine{r}, now, time.Monday, []time.Weekday{time.Thursday})
	for i, day := range report.Routines[0].Days {
		if day.Status != want[i] {
			t.Errorf("workday change, day %s: got %s, want %s", day.Date, day.Status, want[i])
		}
	}
}
