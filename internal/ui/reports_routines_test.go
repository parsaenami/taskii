package ui

import (
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/parsaenami/taskii/internal/model"
	"github.com/parsaenami/taskii/internal/stats"
)

func routineReportFixture() (stats.RoutineWeekReport, []model.Routine) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC) // Thursday
	r := model.Routine{
		ID: "routine", Title: "A routine title long enough to truncate",
		CreatedAt: time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC),
		Schedule:  model.ScheduleEveryDay,
		History: map[string]model.RoutineStatus{
			"2026-09-21": model.RoutineCompleted,
			"2026-09-22": model.RoutineSkipped,
		},
	}
	return stats.ComputeRoutineWeek([]model.Routine{r}, now, time.Monday, nil), []model.Routine{r}
}

func TestReportChartCycleAndLabels(t *testing.T) {
	if chartWeek.String() != "7 Days" {
		t.Fatalf("rolling chart name = %q", chartWeek.String())
	}
	if chartContribution.next() != chartRoutines || chartRoutines.next() != chartWeek || chartWeek.prev() != chartRoutines {
		t.Fatal("routine chart is not in the cycle")
	}
	plain := ansiRe.ReplaceAllString(renderChartTabs(chartRoutines, true, 30), "")
	for _, want := range []string{"7", "M", "C", "R"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("compact tabs %q missing %q", plain, want)
		}
	}
}

func TestRoutineReportRenderingSymbolsSummaryAndZero(t *testing.T) {
	report, routines := routineReportFixture()
	view := ansiRe.ReplaceAllString(renderRoutineReports(report, routines, 76, 14, chartRoutines, true, 0), "")
	for _, want := range []string{"This week", "1 done", "1 skipped", "1 missed", "50.0%", "Mo", "21", "●", "–", "×", "○", "·"} {
		if !strings.Contains(view, want) {
			t.Errorf("routine report missing %q:\n%s", want, view)
		}
	}

	zero := stats.ComputeRoutineWeek(nil, time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC), time.Monday, nil)
	empty := ansiRe.ReplaceAllString(renderRoutineReports(zero, nil, 50, 9, chartRoutines, false, 0), "")
	if !strings.Contains(empty, "Follow-through") || !strings.Contains(empty, "—") || !strings.Contains(empty, "(no routines yet — press R in Tasks)") {
		t.Fatalf("empty routine report =\n%s", empty)
	}
}

func TestRoutineFollowThroughLabelPrecedesBar(t *testing.T) {
	report, routines := routineReportFixture()
	view := ansiRe.ReplaceAllString(renderRoutineReports(report, routines, 76, 14, chartRoutines, true, 0), "")
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "Follow-through") {
			if !strings.HasPrefix(line, "Follow-through ") || strings.TrimSpace(line) == "Follow-through" {
				t.Fatalf("follow-through label and bar are not on one line with one gap: %q", line)
			}
			return
		}
	}
	t.Fatal("follow-through row missing")
}

func TestRoutineReportDoesNotUseTaskSummary(t *testing.T) {
	report, routines := routineReportFixture()
	tasks := stats.Report{Today: stats.Progress{Done: 98, Total: 99}}
	plain := ansiRe.ReplaceAllString(renderReports(tasks, report, routines, 60, 12, chartRoutines, false, 0), "")
	if strings.Contains(plain, "Today") || strings.Contains(plain, "98/99") {
		t.Fatalf("task report leaked into routine chart:\n%s", plain)
	}
}

func TestRoutineStatusSymbols(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(old)
	applyTheme(currentTheme())
	want := map[stats.DayStatus]string{
		stats.DayComplete: "●", stats.DaySkipped: "–", stats.DayMissed: "×",
		stats.DayPending: "○", stats.DayNotDue: "·", stats.DayFuture: "·",
	}
	for status, symbol := range want {
		got, style := routineStatusCell(status)
		if got != symbol {
			t.Errorf("status %s = %q, want %q", status, got, symbol)
		}
		if rendered := style.Render(got); !strings.Contains(rendered, "48;2;") {
			t.Errorf("status %s has no pane background: %q", status, rendered)
		}
	}
}

func TestRoutineReportWeekStartNameTruncationAndDimensions(t *testing.T) {
	report, routines := routineReportFixture()
	sunday := stats.ComputeRoutineWeek(routines, time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC), time.Sunday, nil)
	plain := ansiRe.ReplaceAllString(renderRoutineMatrix(sunday, routines, 40, 5, 0), "")
	first := strings.Split(plain, "\n")[0]
	if !strings.Contains(first, "Su") || strings.Index(first, "Su") > strings.Index(first, "Mo") {
		t.Fatalf("Sunday-first header = %q", first)
	}
	if !strings.Contains(first, "Su Mo Tu We Th Fr Sa") {
		t.Fatalf("routine day columns are not separated: %q", first)
	}

	for _, width := range []int{24, 32, 40, 76} {
		view := renderRoutineReports(report, routines, width, 12, chartRoutines, true, 0)
		if lipgloss.Height(view) > 12 {
			t.Fatalf("width %d height = %d", width, lipgloss.Height(view))
		}
		for i, line := range strings.Split(view, "\n") {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("width %d row %d = %d", width, i, got)
			}
		}
	}
	short := ansiRe.ReplaceAllString(renderRoutineReports(report, routines, 40, 5, chartRoutines, true, 0), "")
	if !strings.Contains(short, "This week") || !strings.Contains(short, "Resize to see") {
		t.Fatalf("short fallback =\n%s", short)
	}
}

func TestRoutineReportScrollAndNoTaskMutation(t *testing.T) {
	a := routineFixture(false)
	a.focus = focusReports
	a.reportChart = chartRoutines
	a.layout = layoutStacked
	a.width, a.height = 120, 24
	base := a.routines[0]
	a.routines = make([]model.Routine, 12)
	for i := range a.routines {
		a.routines[i] = base
		a.routines[i].ID = string(rune('a' + i))
		a.routines[i].Title = "Routine " + a.routines[i].ID
	}
	tasksBefore := append([]model.Task(nil), a.tasks...)

	for i := 0; i < 10; i++ {
		m, _ := a.updateNormal(tea.KeyMsg{Type: tea.KeyDown})
		a = m.(App)
	}
	if a.routineReportScroll == 0 {
		t.Fatal("down did not scroll routine report")
	}
	important, undone := a.filterImportant, a.filterUndone
	for _, key := range []string{"I", "U", " ", "enter", "d", "i"} {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		if key == " " {
			msg = tea.KeyMsg{Type: tea.KeySpace}
		} else if key == "enter" {
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		}
		m, _ := a.updateNormal(msg)
		a = m.(App)
	}
	if a.filterImportant != important || a.filterUndone != undone || a.mode != modeNormal {
		t.Fatal("item operation leaked through Reports focus")
	}
	m, _ := a.updateNormal(tea.KeyMsg{Type: tea.KeyRight})
	a = m.(App)
	if a.reportChart != chartWeek || a.routineReportScroll != 0 {
		t.Fatalf("switch did not wrap/reset: chart=%v scroll=%d", a.reportChart, a.routineReportScroll)
	}
	if !reflect.DeepEqual(a.tasks, tasksBefore) {
		t.Fatal("report navigation mutated tasks")
	}
}

func TestRoutineReportFrameGeometryAcrossLayouts(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(old)
	defer applyTheme(currentTheme())
	for _, theme := range curatedThemes {
		applyTheme(theme)
		for _, layout := range allLayouts {
			for _, size := range [][2]int{{80, 24}, {100, 30}, {160, 45}} {
				a := routineFixture(false)
				a.focus, a.reportChart, a.layout = focusReports, chartRoutines, layout
				a.width, a.height = size[0], size[1]
				frame := a.View()
				if got := lipgloss.Height(frame); got != a.height {
					t.Errorf("theme=%s layout=%v size=%v height=%d", theme.Name, layout, size, got)
				}
				for i, line := range strings.Split(frame, "\n") {
					if got := lipgloss.Width(line); got != a.width {
						t.Errorf("theme=%s layout=%v size=%v row=%d width=%d", theme.Name, layout, size, i, got)
						break
					}
				}
			}
		}
	}
}
