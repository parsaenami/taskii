package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"taskii/internal/model"
	"taskii/internal/stats"
)

func testNow() time.Time { return time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC) }

// TestAgeCountsFromOriginalDate is the core of the migration design: age is
// measured from when a task was FIRST scheduled, so carrying it forward
// repeatedly keeps growing the number instead of resetting it to 1d. A reset
// would make deferring a task free, which is exactly what the badge exists to
// discourage.
func TestAgeCountsFromOriginalDate(t *testing.T) {
	now := testNow()
	for _, tc := range []struct {
		name string
		task model.Task
		want int
	}{
		{"never migrated", model.Task{Date: "2026-09-12"}, 1},
		{"never migrated, week old", model.Task{Date: "2026-09-06"}, 7},
		{"migrated once, still stale", model.Task{Date: "2026-09-12", OriginalDate: "2026-09-11"}, 2},
		{"migrated repeatedly", model.Task{Date: "2026-09-12", OriginalDate: "2026-08-20"}, 24},
		{"scheduled today", model.Task{Date: "2026-09-13"}, 0},
		{"unparseable date", model.Task{Date: "garbage"}, 0},
		{"future date clamps to 0", model.Task{Date: "2026-12-01"}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.task.AgeDays(now); got != tc.want {
				t.Errorf("AgeDays() = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestAgeIsTimezoneIndependent is a regression test for ages reading one day
// short. AgeDays counts CALENDAR days, so a task dated yesterday is 1d old
// regardless of the observer's zone or the time of day — there is no 24-hour
// waiting period. The original bug subtracted a UTC-parsed date from a LOCAL
// midnight, leaving a sub-day remainder that floored every age down by one
// in any zone east of UTC; it was invisible to tests running in UTC alone.
func TestAgeIsTimezoneIndependent(t *testing.T) {
	zones := map[string]*time.Location{
		"UTC":      time.UTC,
		"UTC+3:30": time.FixedZone("IRST", 3*3600+1800),
		"UTC+13":   time.FixedZone("NZDT", 13*3600),
		"UTC-8":    time.FixedZone("PST", -8*3600),
		"UTC-11":   time.FixedZone("SST", -11*3600),
	}
	// Each moment of the day must give the same answer: the hour is not
	// part of the calculation.
	hours := []int{0, 1, 9, 15, 23}

	for zname, loc := range zones {
		for _, h := range hours {
			now := time.Date(2026, 9, 13, h, 30, 0, 0, loc)
			for _, tc := range []struct {
				date string
				want int
			}{
				{"2026-09-13", 0},
				{"2026-09-12", 1},
				{"2026-09-11", 2},
				{"2026-09-06", 7},
				{"2026-08-20", 24},
			} {
				got := model.Task{Date: tc.date}.AgeDays(now)
				if got != tc.want {
					t.Errorf("%s at %02d:30, task dated %s: AgeDays = %d, want %d",
						zname, h, tc.date, got, tc.want)
				}
			}
		}
	}
}

// TestAgeAcrossDSTBoundary pins that a calendar day of 23 or 25 hours still
// counts as exactly one day.
func TestAgeAcrossDSTBoundary(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("tzdata unavailable:", err)
	}
	// 2026-03-08 is the US spring-forward date: that calendar day is 23h long.
	now := time.Date(2026, 3, 9, 10, 0, 0, 0, ny)
	if got := (model.Task{Date: "2026-03-08"}).AgeDays(now); got != 1 {
		t.Errorf("across spring-forward: AgeDays = %d, want 1", got)
	}
	if got := (model.Task{Date: "2026-03-07"}).AgeDays(now); got != 2 {
		t.Errorf("spanning spring-forward: AgeDays = %d, want 2", got)
	}
	// 2026-11-01 is fall-back: that calendar day is 25h long.
	now = time.Date(2026, 11, 2, 10, 0, 0, 0, ny)
	if got := (model.Task{Date: "2026-11-01"}).AgeDays(now); got != 1 {
		t.Errorf("across fall-back: AgeDays = %d, want 1", got)
	}
}

func TestMigrateSelectedStampsOriginalDate(t *testing.T) {
	a := NewApp(Options{})
	a.noPersist = true
	a.width, a.height = 100, 30
	a.now = testNow
	a.focus = focusOverdue
	a.tasks = []model.Task{{ID: "x", Title: "Stale", Date: "2026-09-10"}}
	a.overdueSelected = 0

	a.migrateSelected()

	got := a.tasks[0]
	if got.Date != "2026-09-13" {
		t.Errorf("Date = %q, want today", got.Date)
	}
	if got.OriginalDate != "2026-09-10" {
		t.Errorf("OriginalDate = %q, want the pre-migration date", got.OriginalDate)
	}
	if !got.IsMigrated() {
		t.Error("expected IsMigrated() after migrating")
	}
	if got.AgeDays(testNow()) != 3 {
		t.Errorf("AgeDays = %d, want 3", got.AgeDays(testNow()))
	}

	// Migrating again (as happens when it rolls over undone) must NOT rebase
	// the age: OriginalDate is stamped once and then left alone.
	a.tasks[0].Date = "2026-09-12"
	a.focus = focusOverdue
	a.overdueSelected = 0
	a.migrateSelected()
	if a.tasks[0].OriginalDate != "2026-09-10" {
		t.Errorf("second migration rebased OriginalDate to %q", a.tasks[0].OriginalDate)
	}
	if a.tasks[0].AgeDays(testNow()) != 3 {
		t.Errorf("age reset on re-migration: %d", a.tasks[0].AgeDays(testNow()))
	}
}

// TestSpaceMigratesInOverdueOnly pins the split: space finishes a task in
// Today and carries one forward in Overdue.
func TestSpaceMigratesInOverdueOnly(t *testing.T) {
	base := func() App {
		a := NewApp(Options{})
		a.noPersist = true
		a.width, a.height = 100, 30
		a.now = testNow
		a.tasks = []model.Task{
			{ID: "today", Title: "Today task", Date: "2026-09-13"},
			{ID: "old", Title: "Old task", Date: "2026-09-10"},
		}
		return a
	}

	a := base()
	a.focus = focusToday
	a.todaySelected = 0
	m, _ := a.updateNormal(tea.KeyMsg{Type: tea.KeySpace})
	a = m.(App)
	if !a.tasks[0].Done {
		t.Error("space in Today should toggle done")
	}
	if a.tasks[1].IsMigrated() {
		t.Error("space in Today must not migrate the overdue task")
	}

	b := base()
	b.focus = focusOverdue
	b.overdueSelected = 0
	m, _ = b.updateNormal(tea.KeyMsg{Type: tea.KeySpace})
	b = m.(App)
	if b.tasks[1].Done {
		t.Error("space in Overdue must not mark done")
	}
	if !b.tasks[1].IsMigrated() || b.tasks[1].Date != "2026-09-13" {
		t.Errorf("space in Overdue should migrate to today, got date=%q migrated=%v",
			b.tasks[1].Date, b.tasks[1].IsMigrated())
	}
}

// TestTaskRowMarkers checks the glyph columns: an age badge instead of a
// checkbox in Overdue, and ▲ alongside (not instead of) ★ in Today.
func TestTaskRowMarkers(t *testing.T) {
	now := testNow()
	plain := func(s string) string { return ansiRe.ReplaceAllString(s, "") }

	todayRow := func(task model.Task) string {
		return plain(renderTaskLine(task, false, false, 40, colorPaneBg, now))
	}
	overdueRow := func(task model.Task) string {
		return plain(renderTaskLine(task, false, true, 40, colorPaneBg, now))
	}

	if got := todayRow(model.Task{Title: "A", Date: "2026-09-13"}); !strings.Contains(got, "[ ]") {
		t.Errorf("Today row should keep its checkbox: %q", got)
	}
	if got := todayRow(model.Task{Title: "A", Date: "2026-09-13"}); strings.Contains(got, "▲") {
		t.Errorf("un-migrated task should carry no ▲: %q", got)
	}

	mig := model.Task{Title: "A", Date: "2026-09-13", OriginalDate: "2026-09-10"}
	if got := todayRow(mig); !strings.Contains(got, "▲") {
		t.Errorf("migrated task should carry ▲: %q", got)
	}

	migImp := mig
	migImp.Important = true
	got := todayRow(migImp)
	if !strings.Contains(got, "▲") || !strings.Contains(got, "★") {
		t.Errorf("migrated+important should carry BOTH ▲ and ★: %q", got)
	}
	// The star comes first: importance is a property of the task itself,
	// while the triangle describes how it got here, so the intrinsic fact
	// leads.
	if strings.Index(got, "★") > strings.Index(got, "▲") {
		t.Errorf("★ should precede ▲: %q", got)
	}

	// Overdue rows trade the checkbox for a right-aligned age badge.
	od := overdueRow(model.Task{Title: "A", Date: "2026-09-06"})
	if strings.Contains(od, "[ ]") {
		t.Errorf("Overdue row should not show a checkbox: %q", od)
	}
	if !strings.Contains(od, "7d") {
		t.Errorf("Overdue row should show its age: %q", od)
	}
	// Ages of differing digit counts must start their titles on one column.
	short := overdueRow(model.Task{Title: "T", Date: "2026-09-12"})
	long := overdueRow(model.Task{Title: "T", Date: "2026-08-20"})
	if strings.Index(short, "T") != strings.Index(long, "T") {
		t.Errorf("age column not aligned:\n 1d: %q\n24d: %q", short, long)
	}
}

// TestDayBarCountsMigratedSubset pins DoneMigrated as a SUBSET of Done, not a
// sibling — so Total == Done + open still holds and the renderer's band math
// needs no special case.
func TestDayBarCountsMigratedSubset(t *testing.T) {
	now := testNow()
	tasks := []model.Task{
		// 3 scheduled today, 2 carried forward; 2 normal + 1 carried done.
		{ID: "1", Date: "2026-09-13", Done: true},
		{ID: "2", Date: "2026-09-13", Done: true},
		{ID: "3", Date: "2026-09-13"},
		{ID: "4", Date: "2026-09-13", OriginalDate: "2026-09-09", Done: true},
		{ID: "5", Date: "2026-09-13", OriginalDate: "2026-09-11"},
	}
	rep := stats.Compute(tasks, now)
	last := rep.WeekBars[len(rep.WeekBars)-1]
	if last.Total != 5 {
		t.Errorf("Total = %d, want 5", last.Total)
	}
	if last.Done != 3 {
		t.Errorf("Done = %d, want 3 (all completions, carried included)", last.Done)
	}
	if last.DoneMigrated != 1 {
		t.Errorf("DoneMigrated = %d, want 1", last.DoneMigrated)
	}
	if last.DoneMigrated > last.Done {
		t.Error("DoneMigrated must be a subset of Done")
	}
}

// TestBarCellBands walks one bar's cells and asserts the three colour bands
// stack bottom-up as carried / done / open, compositing at boundaries that
// land mid-row rather than leaving a background gap.
func TestBarCellBands(t *testing.T) {
	b := stats.DayBar{Done: 3, DoneMigrated: 1, Total: 5}
	blank := lipgloss.NewStyle().Background(colorPaneBg)
	fgOf := func(top, bottom float64) lipgloss.Color {
		_, st := barCell(b, top, bottom, colorPurple, colorGreen, colorMuted, blank)
		return st.GetForeground().(lipgloss.Color)
	}

	// One unit per row: bands land on row boundaries.
	if got := fgOf(1, 0); got != colorPurple {
		t.Errorf("bottom band should be the carried colour, got %v", got)
	}
	if got := fgOf(3, 2); got != colorGreen {
		t.Errorf("middle band should be the done colour, got %v", got)
	}
	if got := fgOf(5, 4); got != colorMuted {
		t.Errorf("top band should be the open colour, got %v", got)
	}

	// Two units per row: every boundary falls mid-cell and must composite
	// the band above as its background so the bar reads as solid.
	_, st := barCell(b, 2, 0, colorPurple, colorGreen, colorMuted, blank)
	if st.GetBackground().(lipgloss.Color) != colorGreen {
		t.Errorf("carried/done boundary should composite over done, got %v", st.GetBackground())
	}
	_, st = barCell(b, 4, 2, colorPurple, colorGreen, colorMuted, blank)
	if st.GetBackground().(lipgloss.Color) != colorMuted {
		t.Errorf("done/open boundary should composite over open, got %v", st.GetBackground())
	}
	// The bar's own top edge has nothing above it: the pane background is right.
	_, st = barCell(b, 6, 4, colorPurple, colorGreen, colorMuted, blank)
	if st.GetBackground().(lipgloss.Color) != colorPaneBg {
		t.Errorf("bar top edge should sit on the pane background, got %v", st.GetBackground())
	}

	// An empty day paints nothing.
	if g, _ := barCell(stats.DayBar{}, 1, 0, colorPurple, colorGreen, colorMuted, blank); g != " " {
		t.Errorf("empty bar should render blank, got %q", g)
	}
}
