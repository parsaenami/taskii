package stats

import (
	"testing"
	"time"

	"taskii/internal/model"
)

func day(s string) time.Time {
	d, _ := time.ParseInLocation(dateFormat, s, time.UTC)
	return d
}

// TestCompletionCreditedToDayFinished is a regression test for bars that
// appeared to shrink. A task that outlives its scheduled day — carried
// forward, or held in Today's list by a deadline — is finished days after
// its Date. Crediting that completion to the Date silently turned the old
// day's open band done (so its bar looked like it lost height) while the day
// the work actually happened gained nothing.
func TestCompletionCreditedToDayFinished(t *testing.T) {
	now := day("2026-09-13")
	doneAt := now

	late := model.Task{
		ID: "x", Title: "Late thing",
		Date: "2026-09-10", DueDate: "2026-09-11",
		Done: true, DoneAt: &doneAt,
	}

	rep := Compute([]model.Task{late}, now)
	byDay := map[string]DayBar{}
	for _, b := range rep.WeekBars {
		byDay[dayKey(b.Date)] = b
	}

	if got := byDay["2026-09-10"]; got.Total != 0 {
		t.Errorf("scheduled day should no longer carry the task: %+v", got)
	}
	today := byDay["2026-09-13"]
	if today.Total != 1 || today.Done != 1 {
		t.Errorf("completion should land on the day finished: %+v", today)
	}
	// Work that reached today from an earlier day is "carried", whether it
	// got here by migration or by a deadline holding it open.
	if today.DoneMigrated != 1 {
		t.Errorf("completing older work should count as carried: %+v", today)
	}
}

// An open task still belongs to the day it's scheduled for — only
// completions move.
func TestOpenTaskStaysOnScheduledDay(t *testing.T) {
	now := day("2026-09-13")
	open := model.Task{ID: "x", Date: "2026-09-10", DueDate: "2026-09-11"}

	rep := Compute([]model.Task{open}, now)
	for _, b := range rep.WeekBars {
		switch dayKey(b.Date) {
		case "2026-09-10":
			if b.Total != 1 || b.Done != 0 {
				t.Errorf("open task should sit on its scheduled day: %+v", b)
			}
		case "2026-09-13":
			if b.Total != 0 {
				t.Errorf("open task should not reach today: %+v", b)
			}
		}
	}
}

// The heatmap, streak and progress rollups ask the same "what happened on
// day X" question the bars do, so they must agree.
func TestAllReportsUseTheDayFinished(t *testing.T) {
	now := day("2026-09-13")
	doneAt := now
	late := model.Task{
		ID: "x", Date: "2026-08-30", Done: true, DoneAt: &doneAt,
		OriginalDate: "2026-08-28",
	}

	rep := Compute([]model.Task{late}, now)

	if rep.Today.Done != 1 {
		t.Errorf("Today progress = %+v, want the completion counted today", rep.Today)
	}
	// Finishing something today is a one-day streak; keyed on Date it would
	// have registered as nothing at all.
	if rep.Streak != 1 {
		t.Errorf("Streak = %d, want 1", rep.Streak)
	}
	for _, c := range rep.Heatmap {
		if c.Date == "2026-09-13" && c.Done != 1 {
			t.Errorf("heatmap cell for today = %+v, want Done 1", c)
		}
		if c.Date == "2026-08-30" && c.Done != 0 {
			t.Errorf("heatmap should not credit the scheduled day: %+v", c)
		}
	}
}

// Tasks completed before DoneAt existed have no timestamp; they must keep
// reporting against their Date rather than dropping out of the charts.
func TestLegacyCompletionsFallBackToDate(t *testing.T) {
	now := day("2026-09-13")
	legacy := model.Task{ID: "old", Date: "2026-09-11", Done: true} // no DoneAt

	rep := Compute([]model.Task{legacy}, now)
	var found bool
	for _, b := range rep.WeekBars {
		if dayKey(b.Date) == "2026-09-11" && b.Done == 1 {
			found = true
		}
	}
	if !found {
		t.Error("a completion without DoneAt should still count on its Date")
	}
}
