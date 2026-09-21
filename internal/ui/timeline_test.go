package ui

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/parsaenami/taskii/internal/model"
)

func timelineTestApp() App {
	a := NewApp(Options{Mock: true})
	a.now = func() time.Time { return time.Date(2026, 9, 21, 12, 30, 0, 0, time.UTC) }
	a.tasks = nil
	a.routines = nil
	a.notes = nil
	a.width, a.height = 160, 45
	return a
}

func timelineKey(a App, key string) App {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	switch key {
	case "left":
		msg = tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		msg = tea.KeyMsg{Type: tea.KeyRight}
	case "up":
		msg = tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	case " ":
		msg = tea.KeyMsg{Type: tea.KeySpace}
	}
	updated, _ := a.Update(msg)
	return updated.(App)
}

func TestTimelineMembershipOrderingAndFilters(t *testing.T) {
	a := timelineTestApp()
	now := a.now()
	a.tasks = []model.Task{
		{ID: "later-created", Kind: model.KindAppointment, Date: "2026-09-21", Time: "09:00", CreatedAt: now.Add(time.Minute)},
		{ID: "first", Kind: model.KindAppointment, Date: "2026-09-21", Time: "09:00", Important: true, CreatedAt: now},
		{ID: "done", Kind: model.KindAppointment, Date: "2026-09-21", Time: "11:00", Done: true, Important: true},
		{ID: "deadline-projection", Kind: model.KindAppointment, Date: "2026-09-20", Time: "10:00", DueDate: "2026-09-22"},
		{ID: "future", Kind: model.KindAppointment, Date: "2026-09-22", Time: "10:00"},
		{ID: "task-with-time", Kind: model.KindTask, Date: "2026-09-21", Time: "10:00"},
		{ID: "invalid", Kind: model.KindAppointment, Date: "2026-09-21", Time: "25:00"},
		{ID: "untimed", Kind: model.KindAppointment, Date: "2026-09-21"},
	}
	if got := upcomingIDs(a.timelineTasks()); !reflect.DeepEqual(got, []string{"first", "later-created", "done"}) {
		t.Fatalf("timeline projection = %v", got)
	}
	a.filterImportant, a.filterUndone = true, true
	if got := upcomingIDs(a.timelineTasks()); !reflect.DeepEqual(got, []string{"first"}) {
		t.Fatalf("timeline filters = %v", got)
	}
}

func TestTimelineTabNavigation(t *testing.T) {
	a := timelineTestApp()
	a = timelineKey(a, "right")
	if !a.timeline || a.upcoming {
		t.Fatal("right from Today did not open Timeline")
	}
	a = timelineKey(a, "l")
	if a.timeline || !a.upcoming {
		t.Fatal("l from Timeline did not open Upcoming")
	}
	a = timelineKey(a, "left")
	if !a.timeline || a.upcoming {
		t.Fatal("left from Upcoming did not return to Timeline")
	}
	a = timelineKey(a, "h")
	if a.timeline || a.upcoming {
		t.Fatal("h from Timeline did not return to Today")
	}
	a = timelineTestApp()
	a.focus = focusReports
	chart := a.reportChart
	a = timelineKey(a, "right")
	if a.reportChart == chart || a.timeline {
		t.Fatal("Reports chart navigation was intercepted by Timeline")
	}
}

func TestTimelineActionsTargetSelectedAppointment(t *testing.T) {
	a := timelineTestApp()
	now := a.now()
	a.tasks = []model.Task{
		{ID: "one", Title: "First", Kind: model.KindAppointment, Date: "2026-09-21", Time: "09:00", CreatedAt: now},
		{ID: "two", Title: "Second", Kind: model.KindAppointment, Date: "2026-09-21", Time: "09:00", CreatedAt: now.Add(time.Minute)},
		{ID: "ordinary", Title: "Task", Kind: model.KindTask, Date: "2026-09-21"},
	}
	a.setTimeline(true)
	a = timelineKey(a, "down")
	a = timelineKey(a, "i")
	a = timelineKey(a, " ")
	if a.tasks[0].Important || a.tasks[0].Done || !a.tasks[1].Important || !a.tasks[1].Done {
		t.Fatal("Timeline actions did not follow appointment selection identity")
	}
	a = timelineKey(a, "enter")
	if a.mode != modeAdding || a.taskEditID != "two" {
		t.Fatal("enter did not edit the selected Timeline appointment")
	}
	a = timelineKey(a, "esc")
	a = timelineKey(a, "d")
	if a.mode != modeConfirmDelete || a.deleteItemID != "task:two" {
		t.Fatal("delete confirmation did not pin the selected appointment")
	}
	a = timelineKey(a, "y")
	if got := upcomingIDs(a.tasks); !reflect.DeepEqual(got, []string{"one", "ordinary"}) {
		t.Fatalf("Timeline delete removed wrong task: %v", got)
	}
}

func TestTimelineAdditionStaysOnlyForTodayAppointments(t *testing.T) {
	a := timelineTestApp()
	a.setTimeline(true)
	if !a.addTask("Planning 14:00") || !a.timeline || len(a.timelineTasks()) != 1 || a.selectedTask().Title != "Planning" {
		t.Fatal("today appointment did not stay selected on Timeline")
	}
	if !a.addTask("Untimed task") || a.timeline || a.upcoming {
		t.Fatal("untimed addition from Timeline should follow existing Today switching")
	}
}

func TestTimelineRailMarkersEmptyStateAndDimensions(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 30, 0, 0, time.UTC)
	tasks := []model.Task{
		{ID: "point", Title: "Point", Kind: model.KindAppointment, Date: "2026-09-21", Time: "09:00", Important: true},
		{ID: "range", Title: "Range", Kind: model.KindAppointment, Date: "2026-09-21", Time: "14:00", EndTime: "15:30", Done: true},
	}
	for _, size := range [][2]int{{12, 3}, {23, 5}, {44, 8}, {70, 16}} {
		rendered := renderTimeline(tasks, 0, 0, true, size[0], size[1], now)
		if got := lipgloss.Height(rendered); got != size[1] {
			t.Fatalf("%dx%d timeline height = %d", size[0], size[1], got)
		}
		for i, line := range strings.Split(rendered, "\n") {
			if got := lipgloss.Width(line); got != size[0] {
				t.Fatalf("%dx%d row %d width = %d", size[0], size[1], i, got)
			}
		}
		plain := ansiRe.ReplaceAllString(rendered, "")
		if !strings.Contains(plain, "NOW") {
			t.Fatalf("%dx%d timeline lost NOW", size[0], size[1])
		}
	}
	plain := ansiRe.ReplaceAllString(renderTimeline(tasks, 0, 0, true, 70, 16, now), "")
	for _, want := range []string{timelinePointMarker, timelineRangeMarker, "★", "→ 15:30", "NOW"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("expanded rail missing %q:\n%s", want, plain)
		}
	}
	compact := ansiRe.ReplaceAllString(renderTimeline(tasks, 0, 0, true, 30, 2, now), "")
	if strings.Contains(compact, "↑0") || !strings.Contains(compact, "↓1") {
		t.Fatalf("compact scroll hint should omit zero-count directions:\n%s", compact)
	}
	empty := ansiRe.ReplaceAllString(renderTimeline(nil, 0, 0, true, 50, 9, now), "")
	if !strings.Contains(empty, "NOW") || !strings.Contains(empty, "No appointments today") {
		t.Fatalf("empty Timeline missing current marker or message:\n%s", empty)
	}
	for _, line := range strings.Split(empty, "\n") {
		if !strings.Contains(line, timelineRailMarker) {
			continue
		}
		label := strings.TrimSpace(strings.SplitN(line, timelineRailMarker, 2)[0])
		if label != "" && !strings.HasSuffix(label, ":00") {
			t.Fatalf("rail label should use a whole hour, got %q", label)
		}
	}
}

func TestTimelineRailAlignsWithEventMarkers(t *testing.T) {
	axis := ansiRe.ReplaceAllString(timelineAxisLine(8*60, 40), "")
	rail := ansiRe.ReplaceAllString(timelineRailLine(40), "")
	point := ansiRe.ReplaceAllString(timelineEventLine(model.Task{Title: "Point", Time: "09:00"}, false, 40), "")
	selectedRange := ansiRe.ReplaceAllString(timelineEventLine(model.Task{Title: "Range", Time: "10:00", EndTime: "11:00"}, true, 40), "")

	markerColumn := func(line, marker string) int {
		return lipgloss.Width(strings.SplitN(line, marker, 2)[0])
	}
	want := markerColumn(point, timelinePointMarker)
	for name, got := range map[string]int{
		"hour axis":      markerColumn(axis, timelineRailMarker),
		"plain rail":     markerColumn(rail, timelineRailMarker),
		"selected range": markerColumn(selectedRange, timelineRangeMarker),
	} {
		if got != want {
			t.Fatalf("%s marker column = %d, event marker column = %d", name, got, want)
		}
	}
}

func TestTimelineCurrentMarkerMovesAndOverlapsRemainVisible(t *testing.T) {
	tasks := []model.Task{
		{ID: "early", Title: "Early", Kind: model.KindAppointment, Date: "2026-09-21", Time: "08:00"},
		{ID: "same-a", Title: "Same A", Kind: model.KindAppointment, Date: "2026-09-21", Time: "12:00"},
		{ID: "same-b", Title: "Same B", Kind: model.KindAppointment, Date: "2026-09-21", Time: "12:00"},
		{ID: "late", Title: "Late", Kind: model.KindAppointment, Date: "2026-09-21", Time: "18:00"},
	}
	nowRow := func(rendered string) int {
		for i, line := range strings.Split(ansiRe.ReplaceAllString(rendered, ""), "\n") {
			if strings.Contains(line, "NOW") {
				return i
			}
		}
		return -1
	}
	morning := renderTimeline(tasks, 0, 0, true, 60, 14, time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC))
	afternoon := renderTimeline(tasks, 0, 0, true, 60, 14, time.Date(2026, 9, 21, 15, 0, 0, 0, time.UTC))
	if nowRow(morning) >= nowRow(afternoon) {
		t.Fatalf("NOW line did not move with time: morning=%d afternoon=%d", nowRow(morning), nowRow(afternoon))
	}
	plain := ansiRe.ReplaceAllString(morning, "")
	if strings.Count(plain, "Same A") != 1 || strings.Count(plain, "Same B") != 1 {
		t.Fatal("same-start appointments collided or disappeared")
	}
}

func TestTimelineRowsCarryBackgroundOnEveryCell(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(old)
	applyTheme(currentTheme())

	tasks := []model.Task{
		{Title: "Point", Kind: model.KindAppointment, Date: "2026-09-21", Time: "09:00", Important: true},
		{Title: "Range", Kind: model.KindAppointment, Date: "2026-09-21", Time: "14:00", EndTime: "15:00", Done: true},
	}
	rendered := renderTimeline(tasks, 0, 0, true, 55, 10, time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC))
	if err := requireBackgroundEveryCell(rendered); err != nil {
		t.Fatal(err)
	}
}

func TestTimelineFullPageLayoutsTickAndSimpleIsolation(t *testing.T) {
	for _, layout := range []layout{layoutTasksLeft, layoutTasksRight, layoutStacked, layoutThreeColumn} {
		for _, size := range [][2]int{{70, 24}, {100, 30}, {160, 45}} {
			t.Run(fmt.Sprintf("%s/%dx%d", layout, size[0], size[1]), func(t *testing.T) {
				a := timelineTestApp()
				a.layout = layout
				a.width, a.height = size[0], size[1]
				a.tasks = []model.Task{
					{ID: "one", Title: "Standup", Kind: model.KindAppointment, Date: "2026-09-21", Time: "09:00"},
					{ID: "two", Title: "Review", Kind: model.KindAppointment, Date: "2026-09-21", Time: "14:00", EndTime: "15:00"},
				}
				a.setTimeline(true)
				page := a.View()
				plain := ansiRe.ReplaceAllString(page, "")
				if !strings.Contains(plain, "Timeline") {
					t.Fatal("active Timeline view is not visible")
				}
				// Below the compact strip's irreducible width renderTodayTabs
				// truncates the trailing names rather than overflowing its pane.
				if a.geometry().taskWidth-4 >= lipgloss.Width("Today│Timeline│Upcoming") {
					for _, tab := range []string{"Today", "Timeline", "Upcoming"} {
						if !strings.Contains(plain, tab) {
							t.Fatalf("missing tab %q", tab)
						}
					}
				}
				if lipgloss.Width(page) != a.width || lipgloss.Height(page) != a.height {
					t.Fatalf("page = %dx%d, terminal = %dx%d", lipgloss.Width(page), lipgloss.Height(page), a.width, a.height)
				}
			})
		}
	}

	a := timelineTestApp()
	now := a.now()
	a.now = func() time.Time { return now }
	a.setTimeline(true)
	before := ansiRe.ReplaceAllString(a.View(), "")
	now = now.Add(time.Minute)
	updated, _ := a.Update(pomodoroTickMsg(now))
	a = updated.(App)
	after := ansiRe.ReplaceAllString(a.View(), "")
	if !strings.Contains(before, "12:30 ━ NOW") || !strings.Contains(after, "12:31 ━ NOW") {
		t.Fatal("existing one-second tick did not refresh Timeline current time")
	}

	simple := timelineTestApp()
	simple.simple = true
	simple = timelineKey(simple, "right")
	if simple.timeline || strings.Contains(ansiRe.ReplaceAllString(simple.View(), ""), "Timeline") {
		t.Fatal("Timeline leaked into simple mode")
	}
}

func TestTimelineReservesInputRowWithoutShrinking(t *testing.T) {
	a := timelineTestApp()
	a.tasks = []model.Task{
		{ID: "early", Title: "Early", Kind: model.KindAppointment, Date: "2026-09-21", Time: "08:00"},
		{ID: "late", Title: "Late", Kind: model.KindAppointment, Date: "2026-09-21", Time: "18:00"},
	}
	a.setTimeline(true)

	normalRows := a.visibleRowsFor(focusToday)
	normal := strings.Split(ansiRe.ReplaceAllString(a.View(), ""), "\n")
	rowOf := func(lines []string, needle string) int {
		for i, line := range lines {
			if strings.Contains(line, needle) {
				return i
			}
		}
		return -1
	}
	normalNow, normalLate := rowOf(normal, "NOW"), rowOf(normal, "Late")

	a.mode = modeAdding
	a.input.Focus()
	addingRows := a.visibleRowsFor(focusToday)
	adding := strings.Split(ansiRe.ReplaceAllString(a.View(), ""), "\n")
	if addingRows != normalRows {
		t.Fatalf("Timeline viewport changed when input opened: normal=%d adding=%d", normalRows, addingRows)
	}
	if got := rowOf(adding, "NOW"); got != normalNow {
		t.Fatalf("NOW row moved when input opened: normal=%d adding=%d", normalNow, got)
	}
	if got := rowOf(adding, "Late"); got != normalLate {
		t.Fatalf("event row moved when input opened: normal=%d adding=%d", normalLate, got)
	}
	if rowOf(adding, "+ ") < 0 {
		t.Fatal("reserved row did not render the task input")
	}
}

// requireBackgroundEveryCell tracks SGR background state through the rendered
// bytes. It catches raw separators/padding between independently styled spans,
// the exact class of pane-background regressions this renderer must avoid.
func requireBackgroundEveryCell(rendered string) error {
	background := false
	line, column := 1, 0
	for i := 0; i < len(rendered); {
		if rendered[i] == '\n' {
			background = false
			line, column = line+1, 0
			i++
			continue
		}
		if rendered[i] == 0x1b && i+1 < len(rendered) && rendered[i+1] == '[' {
			end := strings.IndexByte(rendered[i:], 'm')
			if end < 0 {
				return fmt.Errorf("unterminated SGR on line %d", line)
			}
			params := strings.Split(rendered[i+2:i+end], ";")
			for _, param := range params {
				code, _ := strconv.Atoi(param)
				switch code {
				case 0, 49:
					background = false
				case 48:
					background = true
				}
			}
			i += end + 1
			continue
		}
		r, size := utf8.DecodeRuneInString(rendered[i:])
		if r != '\r' && !background {
			return fmt.Errorf("unstyled cell at line %d column %d: %q", line, column, r)
		}
		column += lipgloss.Width(string(r))
		i += size
	}
	return nil
}
