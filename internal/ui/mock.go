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

	// entry builds a task from the SAME annotation syntax the user types, so
	// the mock set can't drift from what the parser actually accepts — a
	// hand-built model.Task could express states the input syntax can't
	// produce, which would make --mock a misleading demo.
	//
	// Relative "!Nd" deadlines are resolved against `now` here exactly as
	// addTask does, so a mock deadline counts down like a real one.
	entry := func(id, raw, dateOffset string, done, important bool) model.Task {
		p, ok := parseTaskInput(raw)
		if !ok {
			// A malformed mock entry is a bug in this file, not user input.
			panic("mock: unparseable task input: " + raw)
		}
		t := model.Task{
			ID:        id,
			Title:     p.title,
			Done:      done,
			Important: important,
			Kind:      p.kind,
			Date:      dateOffset,
			Time:      p.time,
			EndTime:   p.endTime,
			Tags:      p.tags,
			CreatedAt: now,
		}
		if p.dueSet {
			t.DueDate = now.AddDate(0, 0, p.dueInDays).Format(dateFormat)
		}
		return t
	}

	tasks := []model.Task{
		mk("m1", "Review pull requests", day(0), false, true, "09:30"),
		mk("m2", "Write weekly status update", day(0), false, false, ""),
		mk("m3", "Team standup", day(0), false, false, "10:00"),
		mk("m4", "Reply to client emails", day(0), true, false, ""),
		mk("m5", "Fix flaky CI test", day(0), false, true, ""),
		mk("m6", "Grocery run", day(0), true, false, "18:00"),

		// One example of every annotation, so --mock shows the whole syntax
		// at a glance rather than only the plain rows above.

		// Tags: one, several, and one on an appointment.
		entry("m10", "Fix login redirect #api", day(0), false, false),
		entry("m11", "Draft the launch post #marketing #q4", day(0), false, false),
		entry("m12", "Design review #design 14:00-15:30", day(0), false, false),

		// Time range vs. a point in time (m3 above is the point form).
		entry("m13", "Sprint planning 11:00-12:30", day(0), false, false),

		// Deadline examples live behind dueDatesEnabled; see below.
		entry("m14", "Submit the compliance forms #ops", day(0), false, true),
		entry("m15", "Renew the SSL certificate #infra", day(0), false, false),
		entry("m16", "Book the offsite venue", day(0), false, false),

		// Everything at once, to prove the segments coexist on one row.
		entry("m19", "Quarterly board deck #ops #q4 09:00-10:30", day(0), false, true),

		// Plain overdue tasks (no deadline) — these DO belong in the Overdue
		// pane, and one has been carried forward so the ▲ marker shows.
		mk("m7", "Finish quarterly report", day(-1), false, true, ""),
		mk("m8", "Call the dentist", day(-2), false, false, ""),
		entry("m20", "Update the runbook #infra", day(-9), false, false),
	}

	// Deadline examples: the full countdown, from still-ahead ("!Nd") to
	// already missed ("‼Nd"). Gated with the feature itself — with deadlines
	// off, a "!2d" would survive parsing as literal title text and read as a
	// bug rather than a demo.
	//
	// The missed ones set DueDate directly because the "!Nd" syntax only
	// accepts offsets into the future: a task reaches the overdue state by
	// the clock moving, not by being entered that way.
	if dueDatesEnabled {
		withDue := func(id, raw string, scheduled, dueOffset int, important bool) model.Task {
			t := entry(id, raw, day(scheduled), false, important)
			t.DueDate = day(dueOffset)
			return t
		}
		tasks = append(tasks,
			withDue("m14b", "Submit the compliance forms #ops", 0, 0, true),
			withDue("m15b", "Renew the SSL certificate #infra", 0, 2, false),
			withDue("m16b", "Book the offsite venue", 0, 5, false),
			withDue("m17", "Send the signed contract #legal", -5, -2, true),
			withDue("m18", "File the expense report", -3, -1, false),
			withDue("m23", "Reply to the security questionnaire #ops", -12, -8, false),
		)
	}

	// A migrated task: carried forward into today from an earlier day, which
	// is what the red ▲ marks. OriginalDate is what keeps its age counting
	// from the original date rather than resetting on each carry-forward.
	migrated := entry("m21", "Chase the vendor invoice #ops", day(0), false, false)
	migrated.OriginalDate = day(-4)
	tasks = append(tasks, migrated)

	migratedImportant := entry("m22", "Rotate the API keys #infra", day(0), false, true)
	migratedImportant.OriginalDate = day(-6)
	tasks = append(tasks, migratedImportant)

	// Some of the backfilled history is marked as carried-forward-and-
	// completed, which is the only way the bar chart's third band (migrated
	// work finished that day) has anything to draw. Without these the chart
	// would render as the same two colours it had before that band existed.
	carriedDone := func(id, title string, offset, carriedFrom int) model.Task {
		t := mk(id, title, day(offset), true, false, "")
		t.OriginalDate = day(carriedFrom)
		doneAt := now.AddDate(0, 0, offset)
		t.DoneAt = &doneAt
		return t
	}
	tasks = append(tasks,
		carriedDone("m30", "Patch the staging config", -1, -3),
		carriedDone("m31", "Close out the incident review", -1, -4),
		carriedDone("m32", "Archive last quarter's boards", -2, -5),
		carriedDone("m33", "Reconcile the billing export", -4, -7),
		carriedDone("m34", "Follow up on the audit findings", -5, -9),
	)

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
