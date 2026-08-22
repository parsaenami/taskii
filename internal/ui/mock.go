package ui

import (
	"fmt"
	"time"

	"taskii/internal/model"
)

// mockTasks builds a realistic-looking task set for --mock: a mix of done
// and undone today, a couple of stale overdue items, and enough history
// (spread across the past several weeks) that the heatmap and week/month
// reports have something to show instead of all zeros.
func mockTasks(now time.Time) []model.Task {
	day := func(offset int) string {
		return now.AddDate(0, 0, offset).Format(dateFormat)
	}
	// timeStr non-empty implies an appointment, matching the real add-flow's
	// rule (a trailing HH:MM is what turns an entry into an appointment).
	mk := func(id, title, dateOffset string, done, important bool, timeStr string) model.Task {
		kind := model.KindTask
		if timeStr != "" {
			kind = model.KindAppointment
		}
		return model.Task{
			ID:        id,
			Title:     title,
			Done:      done,
			Important: important,
			Kind:      kind,
			Date:      dateOffset,
			Time:      timeStr,
			CreatedAt: now,
		}
	}

	tasks := []model.Task{
		mk("m1", "Review pull requests", day(0), false, true, "09:30"),
		mk("m2", "Write weekly status update", day(0), false, false, ""),
		mk("m3", "Team standup", day(0), false, false, "10:00"),
		mk("m4", "Reply to client emails", day(0), true, false, ""),
		mk("m5", "Fix flaky CI test", day(0), false, true, ""),
		mk("m6", "Grocery run", day(0), true, false, "18:00"),

		mk("m7", "Finish quarterly report", day(-1), false, true, ""),
		mk("m8", "Call the dentist", day(-2), false, false, ""),
	}

	// Backfill a few weeks of mostly-done history so This Week/This Month
	// progress and the contribution heatmap aren't flat zero.
	titles := []string{
		"Plan sprint", "Deploy release", "Update docs", "Pair on bug fix",
		"1:1 with manager", "Refactor auth module", "Clean up backlog",
		"Write tests", "Review design doc", "Prep demo",
	}
	id := 100
	for offset := -3; offset >= -60; offset -= dayStep(offset) {
		if offset%7 == 0 {
			continue // leave occasional gaps so the heatmap isn't solid
		}
		n := 1 + (offset*-1)%3
		for i := 0; i < n; i++ {
			id++
			done := id%5 != 0 // mostly done, a few stragglers
			// Derive the title index from offset+i rather than id: ids that
			// are undone (id%5==0) all share one residue class mod 10, so
			// keying title choice off id directly picked the same 1-2 titles
			// for every undone item. offset+i isn't correlated with id%5.
			title := titles[((offset*-1)+i)%len(titles)]
			tasks = append(tasks, mk(fmt.Sprintf("m%d", id), title, day(offset), done, id%9 == 0, ""))
		}
	}

	return tasks
}

func dayStep(offset int) int {
	if offset%3 == 0 {
		return 2
	}
	return 1
}

// mockNotes gives the --mock board some content, including a deliberately
// long note and a multi-line one so wrapping and scrolling are exercised.
func mockNotes() []model.Note {
	bodies := []string{
		"Ask Sam about the staging deploy window",
		"Refactor idea: pull the retry logic out of the client and into a\nsmall middleware so the timeout policy lives in one place",
		"Book flights before prices jump",
		"The migration script assumes UTC timestamps everywhere — double check the legacy rows before running it in production, several of them look like they were written with a local offset baked in",
		"Standup moved to 10:15",
	}
	notes := make([]model.Note, 0, len(bodies))
	for i, b := range bodies {
		notes = append(notes, model.Note{
			ID:   fmt.Sprintf("mock-note-%d", i),
			Body: b,
		})
	}
	return notes
}
