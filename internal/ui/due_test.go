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
	now := testNow()
	today := now.Format(dateFormat)
	for _, tc := range []struct {
		name string
		raw  string
		want parsedTask
	}{
		{
			name: "due today",
			raw:  "file the report !0d",
			want: parsedTask{title: "file the report", date: today, kind: model.KindTask, dueSet: true},
		},
		{
			name: "due tomorrow",
			raw:  "ship it !1d",
			want: parsedTask{title: "ship it", date: today, kind: model.KindTask, dueInDays: 1, dueSet: true},
		},
		{
			// Both end-anchored annotations are accepted in either order:
			// they're independent, so imposing one would be an arbitrary rule.
			name: "due then time",
			raw:  "standup !2d 09:00",
			want: parsedTask{title: "standup", date: today, time: "09:00", kind: model.KindAppointment, dueInDays: 2, dueSet: true},
		},
		{
			name: "time then due",
			raw:  "standup 09:00 !2d",
			want: parsedTask{title: "standup", date: today, time: "09:00", kind: model.KindAppointment, dueInDays: 2, dueSet: true},
		},
		{
			name: "with range and tags",
			raw:  "planning #ops 11:00-12:30 !3d",
			want: parsedTask{
				title: "planning #ops", date: today, time: "11:00", endTime: "12:30",
				kind: model.KindAppointment, tags: []string{"ops"}, dueInDays: 3, dueSet: true,
			},
		},
		{
			// Only the LAST fields are annotations, so an interior "!2d"
			// stays part of the title.
			name: "interior due stays in title",
			raw:  "check !2d in the docs",
			want: parsedTask{title: "check !2d in the docs", date: today, kind: model.KindTask},
		},
		{
			name: "no due date",
			raw:  "plain task",
			want: parsedTask{title: "plain task", date: today, kind: model.KindTask},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseTaskInput(tc.raw, now)
			if err != nil {
				t.Fatalf("parseTaskInput(%q) returned error: %v", tc.raw, err)
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
	if got := a.tasks[0].Title; got != "ship it" {
		t.Fatalf("Title = %q, want deadline annotation removed", got)
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

func TestRelativeDuePhrase(t *testing.T) {
	for _, tc := range []struct {
		days int
		want string
	}{
		{-12, "12 days ago"},
		{-1, "yesterday"},
		{0, "today"},
		{1, "tomorrow"},
		{5, "in 5 days"},
	} {
		if got := relativeDuePhrase(tc.days); got != tc.want {
			t.Errorf("relativeDuePhrase(%d) = %q, want %q", tc.days, got, tc.want)
		}
	}
}

func TestDuePhraseRendering(t *testing.T) {
	plain := func(s string) string { return ansiRe.ReplaceAllString(s, "") }
	row := func(due string) string {
		return plain(renderTaskLine(
			model.Task{Title: "Task", Date: "2026-09-13", DueDate: due},
			false, false, 44, colorPaneBg, testNow(), false))
	}

	for _, tc := range []struct{ due, want string }{
		{"2026-09-18", "in 5 days"},
		{"2026-09-14", "tomorrow"},
		{"2026-09-13", "today"},
		{"2026-09-12", "yesterday"},
		{"2026-09-01", "12 days ago"},
	} {
		got := row(tc.due)
		if !strings.HasSuffix(got, tc.want) {
			t.Errorf("due %s: row %q should end with %q", tc.due, got, tc.want)
		}
		if width := lipgloss.Width(got); width != 44 {
			t.Errorf("due %s: row width = %d, want 44", tc.due, width)
		}
	}

	// A task with no deadline gets no due phrase.
	if got := plain(renderTaskLine(model.Task{Title: "Task", Date: "2026-09-13"},
		false, false, 44, colorPaneBg, testNow(), false)); strings.Contains(got, "today") || strings.Contains(got, "tomorrow") || strings.Contains(got, "days") {
		t.Errorf("undated task should carry no due phrase: %q", got)
	}
}

func TestDuePhraseTruncatesTitleFirst(t *testing.T) {
	const width = 32
	plain := ansiRe.ReplaceAllString(renderTaskLine(
		model.Task{Title: "A deliberately very long task title", Date: "2026-09-13", DueDate: "2026-09-18"},
		false, false, width, colorPaneBg, testNow(), false), "")

	if got := lipgloss.Width(plain); got != width {
		t.Fatalf("row width = %d, want %d: %q", got, width, plain)
	}
	if !strings.HasSuffix(plain, "in 5 days") {
		t.Errorf("due phrase was not pinned at right edge: %q", plain)
	}
	if !strings.Contains(plain, "…") {
		t.Errorf("title was not truncated before due phrase: %q", plain)
	}
}

// TestDueDateRoundTrip pins that opening the editor and saving unchanged is a
// no-op — including for an overdue task, whose deadline would otherwise be
// silently rescheduled to today on every save.
// The deadline phrase must not share the ★'s colour: the two sit on one row
// on a row often enough that one hue would read as a single compound symbol
// rather than two independent facts.
func TestDuePhraseColorDiffersFromStar(t *testing.T) {
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
		false, false, 44, colorPaneBg, testNow(), false)

	if fgSeq(colorWarning) == fgSeq(colorPurple) {
		t.Skip("theme gives ★ and the due phrase the same hue; nothing to distinguish")
	}
	if !strings.Contains(row, fgSeq(colorWarning)) {
		t.Errorf("star colour missing from the row: %q", row)
	}
	if !strings.Contains(row, fgSeq(colorPurple)) {
		t.Errorf("due phrase colour missing from the row: %q", row)
	}

	// A missed deadline switches to the danger hue.
	late := renderTaskLine(
		model.Task{Title: "Task", Date: "2026-09-13", DueDate: "2026-09-11"},
		false, false, 44, colorPaneBg, testNow(), false)
	if !strings.Contains(late, fgSeq(colorDanger)) {
		t.Errorf("missed deadline should use the danger colour: %q", late)
	}
}

func TestDueDateRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		due   string
		field string
	}{
		{"2026-09-16", "!3d"},
		{"2026-09-13", "!0d"},
		{"2026-09-11", "‼2d"},
		{"2026-09-01", "‼12d"},
	} {
		a := NewApp(Options{})
		a.noPersist = true
		a.now = testNow
		a.tasks = []model.Task{{ID: "x", Title: "Task", Date: "2026-09-13", DueDate: tc.due}}
		a.focus = focusToday

		m, _ := a.updateNormal(tea.KeyMsg{Type: tea.KeyEnter})
		a = m.(App)
		shown := a.input.Value()
		if shown != "Task "+tc.field {
			t.Errorf("editor showed %q, want machine syntax %q", shown, "Task "+tc.field)
		}

		m, _ = a.updateAdding(tea.KeyMsg{Type: tea.KeyEnter})
		a = m.(App)
		if got := a.tasks[0].DueDate; got != tc.due {
			t.Errorf("editor showed %q; deadline moved %s -> %s", shown, tc.due, got)
		}
	}
}

func TestEditClearsDueDate(t *testing.T) {
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
