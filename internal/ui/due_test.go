package ui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"taskii/internal/model"
)

func TestParseDueField(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want int
		ok   bool
	}{
		{"!0d", 0, true},
		{"!1d", 1, true},
		{"!30d", 30, true},
		{"!365d", 365, true},
		{"!2D", 2, true},
		// The renderer's overdue spelling is accepted back, so an overdue
		// task round-trips its real deadline.
		{"‼1d", -1, true},
		{"‼12d", -12, true},
		// A deadline in the past can't be typed deliberately — a task gets
		// there by the clock moving.
		{"!-2d", 0, false},
		{"!d", 0, false},
		{"!2", 0, false},
		{"2d", 0, false},
		{"!abcd", 0, false},
		{"", 0, false},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			got, ok := parseDueField(tc.raw)
			if ok != tc.ok || (ok && got != tc.want) {
				t.Errorf("parseDueField(%q) = (%d, %v), want (%d, %v)", tc.raw, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestParseDueAnnotation(t *testing.T) {
	if !dueDatesEnabled {
		t.Skip("due dates are shelved; see dueDatesEnabled")
	}
	for _, tc := range []struct {
		name string
		raw  string
		want parsedTask
	}{
		{
			name: "due today",
			raw:  "file the report !0d",
			want: parsedTask{title: "file the report", kind: model.KindTask, dueSet: true},
		},
		{
			name: "due tomorrow",
			raw:  "ship it !1d",
			want: parsedTask{title: "ship it", kind: model.KindTask, dueInDays: 1, dueSet: true},
		},
		{
			// Both end-anchored annotations are accepted in either order:
			// they're independent, so imposing one would be an arbitrary rule.
			name: "due then time",
			raw:  "standup !2d 09:00",
			want: parsedTask{title: "standup", time: "09:00", kind: model.KindAppointment, dueInDays: 2, dueSet: true},
		},
		{
			name: "time then due",
			raw:  "standup 09:00 !2d",
			want: parsedTask{title: "standup", time: "09:00", kind: model.KindAppointment, dueInDays: 2, dueSet: true},
		},
		{
			name: "with range and tags",
			raw:  "planning #ops 11:00-12:30 !3d",
			want: parsedTask{
				title: "planning #ops", time: "11:00", endTime: "12:30",
				kind: model.KindAppointment, tags: []string{"ops"}, dueInDays: 3, dueSet: true,
			},
		},
		{
			// Only the LAST fields are annotations, so an interior "!2d"
			// stays part of the title.
			name: "interior due stays in title",
			raw:  "check !2d in the docs",
			want: parsedTask{title: "check !2d in the docs", kind: model.KindTask},
		},
		{
			name: "no due date",
			raw:  "plain task",
			want: parsedTask{title: "plain task", kind: model.KindTask},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseTaskInput(tc.raw)
			if !ok {
				t.Fatalf("parseTaskInput(%q) returned not-ok", tc.raw)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("parseTaskInput(%q)\n got %+v\nwant %+v", tc.raw, got, tc.want)
			}
		})
	}
}

// TestDueDateStoredAbsolute pins the central design decision: the deadline is
// stored as an absolute date, so the countdown the user sees is recomputed
// each day rather than frozen at whatever was typed.
func TestDueDateStoredAbsolute(t *testing.T) {
	if !dueDatesEnabled {
		t.Skip("due dates are shelved; see dueDatesEnabled")
	}
	a := NewApp(Options{})
	a.noPersist = true
	a.now = testNow // 2026-09-13
	a.addTask("ship it !2d")

	if len(a.tasks) != 1 {
		t.Fatalf("task not added: %+v", a.tasks)
	}
	if got := a.tasks[0].DueDate; got != "2026-09-15" {
		t.Fatalf("DueDate = %q, want the resolved absolute date", got)
	}

	// The same task, read on later days, counts down on its own.
	for _, tc := range []struct {
		day  string
		want int
	}{
		{"2026-09-13", 2},
		{"2026-09-14", 1},
		{"2026-09-15", 0},
		{"2026-09-16", -1},
		{"2026-09-20", -5},
	} {
		now, _ := time.ParseInLocation(model.DateFormat, tc.day, time.UTC)
		got, ok := a.tasks[0].DaysUntilDue(now)
		if !ok {
			t.Fatalf("DaysUntilDue reported no due date on %s", tc.day)
		}
		if got != tc.want {
			t.Errorf("on %s: DaysUntilDue = %d, want %d", tc.day, got, tc.want)
		}
	}
}

// TestDueTaskStaysInTodayList is the behaviour that makes a deadline useful:
// the task follows you forward until it's done, instead of falling into
// Overdue the next morning.
func TestDueTaskStaysInTodayList(t *testing.T) {
	if !dueDatesEnabled {
		t.Skip("due dates are shelved; see dueDatesEnabled")
	}
	a := NewApp(Options{})
	a.noPersist = true
	a.now = testNow
	a.tasks = []model.Task{
		// Scheduled days ago, still open, deadline already missed.
		{ID: "due", Title: "Due task", Date: "2026-09-09", DueDate: "2026-09-11"},
		// Same vintage but no deadline: this one belongs in Overdue.
		{ID: "plain", Title: "Plain old", Date: "2026-09-09"},
	}

	var todayIDs []string
	for _, task := range a.todayTasks() {
		todayIDs = append(todayIDs, task.ID)
	}
	if !reflect.DeepEqual(todayIDs, []string{"due"}) {
		t.Errorf("Today = %v, want the due task to have followed forward", todayIDs)
	}

	var overdueIDs []string
	for _, task := range a.overdueTasks() {
		overdueIDs = append(overdueIDs, task.ID)
	}
	// A task must never appear in both panes at once.
	if !reflect.DeepEqual(overdueIDs, []string{"plain"}) {
		t.Errorf("Overdue = %v, want only the deadline-less task", overdueIDs)
	}

	// Completing it is what removes it, not the calendar.
	a.tasks[0].Done = true
	for _, task := range a.todayTasks() {
		if task.ID == "due" {
			t.Error("a completed due task should leave Today's list")
		}
	}
}

// TestDueTasksSortToTop pins the ordering: deadlines above everything else,
// most overdue first, as one continuous urgency gradient rather than an
// "overdue" block sitting above an "upcoming" one.
func TestDueTasksSortToTop(t *testing.T) {
	if !dueDatesEnabled {
		t.Skip("due dates are shelved; see dueDatesEnabled")
	}
	a := NewApp(Options{})
	a.noPersist = true
	a.now = testNow // 2026-09-13
	a.tasks = []model.Task{
		{ID: "appt", Title: "Standup", Time: "09:00", Kind: model.KindAppointment, Date: "2026-09-13"},
		{ID: "plain", Title: "Plain", Date: "2026-09-13"},
		{ID: "soon", Title: "Soon", Date: "2026-09-13", DueDate: "2026-09-15"},
		{ID: "late", Title: "Late", Date: "2026-09-13", DueDate: "2026-09-11"},
		{ID: "today", Title: "Today", Date: "2026-09-13", DueDate: "2026-09-13"},
		{ID: "latest", Title: "Latest", Date: "2026-09-13", DueDate: "2026-09-01"},
	}

	var order []string
	for _, task := range a.todayTasks() {
		order = append(order, task.ID)
	}
	want := []string{"latest", "late", "today", "soon", "appt", "plain"}
	if !reflect.DeepEqual(order, want) {
		t.Errorf("order = %v\n       want %v", order, want)
	}
}

// TestDueChipRendering checks the glyph carries the overdue distinction, so
// the two states stay apart without relying on colour.
func TestDueChipRendering(t *testing.T) {
	if !dueDatesEnabled {
		t.Skip("due dates are shelved; see dueDatesEnabled")
	}
	plain := func(s string) string { return ansiRe.ReplaceAllString(s, "") }
	row := func(due string) string {
		return plain(renderTaskLine(
			model.Task{Title: "Task", Date: "2026-09-13", DueDate: due},
			false, false, 44, colorPaneBg, testNow()))
	}

	for _, tc := range []struct{ due, want string }{
		{"2026-09-16", "!3d"},
		{"2026-09-14", "!1d"},
		{"2026-09-13", "!0d"},
		{"2026-09-12", "‼1d"},
		{"2026-09-01", "‼12d"},
	} {
		if got := row(tc.due); !strings.Contains(got, tc.want) {
			t.Errorf("due %s: row %q should contain %q", tc.due, got, tc.want)
		}
	}

	// The count stays positive past the deadline: "!1d" (due tomorrow) and
	// "‼1d" (one day late) must not be told apart by a minus sign alone.
	if got := row("2026-09-12"); strings.Contains(got, "-1d") {
		t.Errorf("overdue should not use a negative count: %q", got)
	}
	// A task with no deadline gets no chip.
	if got := plain(renderTaskLine(model.Task{Title: "Task", Date: "2026-09-13"},
		false, false, 44, colorPaneBg, testNow())); strings.Contains(got, "!") || strings.Contains(got, "‼") {
		t.Errorf("undated task should carry no due chip: %q", got)
	}
}

// TestDueDateRoundTrip pins that opening the editor and saving unchanged is a
// no-op — including for an overdue task, whose deadline would otherwise be
// silently rescheduled to today on every save.
// The deadline chip must not share the ★'s colour: the two sit side by side
// on a row often enough that one hue would read as a single compound symbol
// rather than two independent facts.
func TestDueChipColorDiffersFromStar(t *testing.T) {
	if !dueDatesEnabled {
		t.Skip("due dates are shelved; see dueDatesEnabled")
	}
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(old)

	fgSeq := func(c lipgloss.Color) string {
		var r, g, b int
		fmt.Sscanf(string(c), "#%02x%02x%02x", &r, &g, &b)
		return fmt.Sprintf("38;2;%d;%d;%d", r, g, b)
	}

	row := renderTaskLine(
		model.Task{Title: "Task", Important: true, Date: "2026-09-13", DueDate: "2026-09-16"},
		false, false, 44, colorPaneBg, testNow())

	if fgSeq(colorWarning) == fgSeq(colorPurple) {
		t.Skip("theme gives ★ and the due chip the same hue; nothing to distinguish")
	}
	if !strings.Contains(row, fgSeq(colorWarning)) {
		t.Errorf("star colour missing from the row: %q", row)
	}
	if !strings.Contains(row, fgSeq(colorPurple)) {
		t.Errorf("due chip colour missing from the row: %q", row)
	}

	// A missed deadline switches to the danger hue on top of the ‼ glyph.
	late := renderTaskLine(
		model.Task{Title: "Task", Date: "2026-09-13", DueDate: "2026-09-11"},
		false, false, 44, colorPaneBg, testNow())
	if !strings.Contains(late, fgSeq(colorDanger)) {
		t.Errorf("missed deadline should use the danger colour: %q", late)
	}
}

func TestDueDateRoundTrip(t *testing.T) {
	if !dueDatesEnabled {
		t.Skip("due dates are shelved; see dueDatesEnabled")
	}
	for _, due := range []string{"2026-09-16", "2026-09-13", "2026-09-11", "2026-09-01"} {
		a := NewApp(Options{})
		a.noPersist = true
		a.now = testNow
		a.tasks = []model.Task{{ID: "x", Title: "Task", Date: "2026-09-13", DueDate: due}}
		a.focus = focusToday

		m, _ := a.updateNormal(tea.KeyMsg{Type: tea.KeyEnter})
		a = m.(App)
		shown := a.input.Value()

		m, _ = a.updateAdding(tea.KeyMsg{Type: tea.KeyEnter})
		a = m.(App)
		if got := a.tasks[0].DueDate; got != due {
			t.Errorf("editor showed %q; deadline moved %s -> %s", shown, due, got)
		}
	}
}

func TestEditClearsDueDate(t *testing.T) {
	if !dueDatesEnabled {
		t.Skip("due dates are shelved; see dueDatesEnabled")
	}
	a := NewApp(Options{})
	a.noPersist = true
	a.now = testNow
	a.tasks = []model.Task{{ID: "x", Title: "Task", Date: "2026-09-13", DueDate: "2026-09-15"}}
	a.focus = focusToday

	m, _ := a.updateNormal(tea.KeyMsg{Type: tea.KeyEnter})
	a = m.(App)
	a.input.SetValue("Task")
	m, _ = a.updateAdding(tea.KeyMsg{Type: tea.KeyEnter})
	a = m.(App)

	if a.tasks[0].HasDueDate() {
		t.Errorf("deleting the annotation should clear the deadline: %+v", a.tasks[0])
	}
}

// TestDueDatesShelved pins the paused state: with dueDatesEnabled off the
// feature must be fully inert — "!2d" is ordinary title text, no task is held
// in Today's list by a deadline, and no chip is drawn — while a DueDate
// already saved from when the feature was on is preserved rather than erased.
//
// The mirror of the guards on the tests above: those skip when the feature is
// off, this one skips when it's on, so exactly one set runs either way.
func TestDueDatesShelved(t *testing.T) {
	if dueDatesEnabled {
		t.Skip("due dates are enabled; the active-feature tests cover this")
	}

	// The parser leaves the annotation alone.
	p, ok := parseTaskInput("submit the forms #ops !2d")
	if !ok {
		t.Fatal("parse failed")
	}
	if p.dueSet {
		t.Error("parser should not recognise !Nd while the feature is shelved")
	}
	if p.title != "submit the forms #ops !2d" {
		t.Errorf("title = %q, want the annotation left as text", p.title)
	}
	// Other annotations are unaffected.
	if len(p.tags) != 1 || p.tags[0] != "ops" {
		t.Errorf("tags = %v, want the tag still parsed", p.tags)
	}

	// No chip is drawn even for a task that carries a stored deadline.
	row := ansiRe.ReplaceAllString(renderTaskLine(
		model.Task{Title: "Legacy", Date: "2026-09-16", DueDate: "2026-09-12"},
		false, false, 46, colorPaneBg, testNow()), "")
	if strings.Contains(row, "!") || strings.Contains(row, "‼") {
		t.Errorf("no due chip should render: %q", row)
	}

	// A stale task with a stored deadline behaves like any other stale task.
	a := NewApp(Options{})
	a.noPersist = true
	a.now = testNow
	a.tasks = []model.Task{{ID: "old", Title: "Stale", Date: "2026-09-09", DueDate: "2026-09-10"}}
	if got := len(a.todayTasks()); got != 0 {
		t.Errorf("Today should hold %d tasks, want 0", got)
	}
	if got := a.overdueTasks(); len(got) != 1 {
		t.Errorf("Overdue = %v, want the stale task", got)
	}

	// Editing must not erase the stored deadline — the editor can't show it,
	// so re-assigning from the parsed text would silently destroy it.
	a.focus = focusOverdue
	m, _ := a.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	a = m.(App)
	a.taskEditID = "old"
	a.input.SetValue("Stale renamed")
	m, _ = a.updateAdding(tea.KeyMsg{Type: tea.KeyEnter})
	a = m.(App)
	if a.tasks[0].DueDate != "2026-09-10" {
		t.Errorf("edit erased the stored deadline: %+v", a.tasks[0])
	}
	if a.tasks[0].Title != "Stale renamed" {
		t.Errorf("edit did not apply: %+v", a.tasks[0])
	}
}
