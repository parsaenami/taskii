package ui

import (
	"strings"
	"testing"
	"time"

	"taskii/internal/stats"
)

// TestMockCoversEveryAnnotation guards --mock as the app's living demo: it
// should exercise every feature a screenshot or a first run ought to show.
// Without this, a new annotation gets added and the mock silently keeps
// demoing the old feature set.
func TestMockCoversEveryAnnotation(t *testing.T) {
	now := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	tasks := mockTasks(now)

	var (
		tagged, multiTagged     int
		ranged, pointInTime     int
		dueUpcoming, dueOverdue int
		dueToday                int
		migrated, migratedDone  int
		important, done         int
	)
	for _, task := range tasks {
		if len(task.Tags) > 0 {
			tagged++
		}
		if len(task.Tags) > 1 {
			multiTagged++
		}
		if task.EndTime != "" {
			ranged++
		} else if task.Time != "" {
			pointInTime++
		}
		if days, ok := task.DaysUntilDue(now); ok {
			switch {
			case days > 0:
				dueUpcoming++
			case days == 0:
				dueToday++
			default:
				dueOverdue++
			}
		}
		if task.IsMigrated() {
			migrated++
			if task.Done {
				migratedDone++
			}
		}
		if task.Important {
			important++
		}
		if task.Done {
			done++
		}
	}

	checks := []struct {
		name string
		got  int
	}{
		{"tagged tasks", tagged},
		{"tasks with several tags", multiTagged},
		{"time ranges", ranged},
		{"point-in-time appointments", pointInTime},
		{"migrated tasks", migrated},
		{"migrated tasks completed", migratedDone},
		{"important tasks", important},
		{"completed tasks", done},
	}
	// Deadline examples are gated with the feature itself.
	if dueDatesEnabled {
		checks = append(checks,
			struct {
				name string
				got  int
			}{"upcoming deadlines", dueUpcoming},
			struct {
				name string
				got  int
			}{"deadlines due today", dueToday},
			struct {
				name string
				got  int
			}{"missed deadlines", dueOverdue},
		)
	}
	for _, c := range checks {
		if c.got == 0 {
			t.Errorf("mock data has no %s", c.name)
		}
	}
}

// A deadline keeps a task in Today's list; only deadline-less stale tasks
// belong in Overdue. The mock must demonstrate both, and never the same task
// in both panes.
func TestMockSeparatesDueFromOverdue(t *testing.T) {
	now := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	a := NewApp(Options{})
	a.noPersist = true
	a.now = func() time.Time { return now }
	a.tasks = mockTasks(now)

	today := a.todayTasks()
	overdue := a.overdueTasks()
	if len(today) == 0 || len(overdue) == 0 {
		t.Fatalf("expected both panes populated, got today=%d overdue=%d", len(today), len(overdue))
	}

	inToday := map[string]bool{}
	for _, task := range today {
		inToday[task.ID] = true
	}
	for _, task := range overdue {
		if inToday[task.ID] {
			t.Errorf("task %q appears in both panes", task.Title)
		}
		if task.HasDueDate() {
			t.Errorf("task %q has a deadline but sits in Overdue", task.Title)
		}
	}

	// At least one missed deadline should be riding along in Today, since
	// that's the behaviour most worth demonstrating.
	var carriedPastDeadline int
	for _, task := range today {
		if _, late := task.IsOverdueBy(now); late {
			carriedPastDeadline++
		}
	}
	if dueDatesEnabled && carriedPastDeadline == 0 {
		t.Error("no missed deadline in Today's list to demonstrate the ‼ state")
	}
}

// The bar chart's third band needs completed carried-forward work to draw.
func TestMockFeedsMigratedBarBand(t *testing.T) {
	now := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	rep := stats.Compute(mockTasks(now), now)

	var withMigrated int
	for _, b := range rep.WeekBars {
		if b.DoneMigrated > 0 {
			withMigrated++
		}
		if b.DoneMigrated > b.Done {
			t.Errorf("%s: DoneMigrated %d exceeds Done %d", b.Date.Format("2006-01-02"), b.DoneMigrated, b.Done)
		}
	}
	if withMigrated < 2 {
		t.Errorf("only %d week days show migrated completions; the third band would barely appear", withMigrated)
	}
}

// The mock's rows must render inside their pane rather than overflowing it.
func TestMockRowsFitTheirPane(t *testing.T) {
	now := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	a := NewApp(Options{})
	a.noPersist = true
	a.now = func() time.Time { return now }
	a.tasks = mockTasks(now)

	const width = 44
	for _, task := range a.todayTasks() {
		row := renderTaskLine(task, false, false, width, colorPaneBg, now)
		if got := len(strings.Split(ansiRe.ReplaceAllString(row, ""), "\n")); got != 1 {
			t.Errorf("task %q wrapped to %d lines", task.Title, got)
		}
	}
}
