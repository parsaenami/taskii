package ui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"taskii/internal/model"
)

func upcomingTestApp() App {
	a := NewApp(Options{Mock: true})
	a.now = func() time.Time { return time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC) }
	a.tasks = nil
	a.notes = nil
	a.width, a.height = 160, 45
	return a
}

func upcomingKey(a App, key string) App {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	if key == "tab" {
		msg = tea.KeyMsg{Type: tea.KeyTab}
	}
	updated, _ := a.Update(msg)
	return updated.(App)
}

func upcomingIDs(tasks []model.Task) []string {
	ids := make([]string, len(tasks))
	for i, task := range tasks {
		ids[i] = task.ID
	}
	return ids
}

func TestUpcomingDatesOrderingAndFilters(t *testing.T) {
	a := upcomingTestApp()
	created := a.now()
	a.tasks = []model.Task{
		{ID: "later", Date: "2027-01-01", CreatedAt: created},
		{ID: "untimed-new", Date: "2026-09-04", CreatedAt: created.Add(time.Minute)},
		{ID: "afternoon", Date: "2026-09-04", Time: "15:00", Kind: model.KindAppointment, CreatedAt: created},
		{ID: "today", Date: "2026-09-03", CreatedAt: created},
		{ID: "missed", Date: "2026-09-02", CreatedAt: created},
		{ID: "morning", Date: "2026-09-04", Time: "08:00", Kind: model.KindAppointment, Important: true, CreatedAt: created},
		{ID: "untimed-old", Date: "2026-09-04", Important: true, CreatedAt: created},
		{ID: "done", Date: "2026-09-04", Done: true, Important: true, CreatedAt: created.Add(time.Hour)},
	}
	want := []string{"morning", "afternoon", "untimed-old", "untimed-new", "done", "later"}
	if got := upcomingIDs(a.upcomingTasks()); !reflect.DeepEqual(got, want) {
		t.Fatalf("upcoming order = %v, want %v", got, want)
	}
	a.upcoming = true
	if got := upcomingIDs(a.todayTasks()); !reflect.DeepEqual(got, []string{"today"}) {
		t.Fatalf("Upcoming contaminated today's projection: %v", got)
	}
	if got := upcomingIDs(a.overdueTasks()); !reflect.DeepEqual(got, []string{"missed"}) {
		t.Fatalf("overdue projection = %v", got)
	}
	a.filterImportant, a.filterUndone = true, true
	if got := upcomingIDs(a.upcomingTasks()); !reflect.DeepEqual(got, []string{"morning", "untimed-old"}) {
		t.Fatalf("combined filters = %v", got)
	}
}

func TestUpcomingActionsTargetVisibleTasks(t *testing.T) {
	for _, simple := range []bool{false, true} {
		t.Run(fmt.Sprintf("simple=%v", simple), func(t *testing.T) {
			a := upcomingTestApp()
			a.simple = simple
			a.tasks = []model.Task{
				{ID: "today", Date: "2026-09-03"},
				{ID: "future", Date: "2026-09-04"},
			}
			a = upcomingKey(a, "C")
			a = upcomingKey(a, "i")
			if a.tasks[0].Important || !a.tasks[1].Important {
				t.Fatal("importance action targeted an invisible task")
			}
			a = upcomingKey(a, " ")
			if a.tasks[0].Done || !a.tasks[1].Done || a.tasks[1].DoneAt == nil {
				t.Fatal("completion action targeted an invisible task")
			}
			a = upcomingKey(a, "U")
			a = upcomingKey(a, "d")
			if a.mode != modeNormal {
				t.Fatal("empty filtered view allowed a delete")
			}
			a = upcomingKey(a, "U")
			a = upcomingKey(a, "d")
			a = upcomingKey(a, "y")
			if got := upcomingIDs(a.tasks); !reflect.DeepEqual(got, []string{"today"}) {
				t.Fatalf("delete retained tasks %v", got)
			}
			if a.todaySelected != 0 || a.todayScroll != 0 || a.simpleSelected != 0 || a.simpleScroll != 0 {
				t.Fatal("empty view has stale selection or scroll")
			}
		})
	}
}

func TestUpcomingKeyPreservesNotesClearAndSimpleTabs(t *testing.T) {
	a := upcomingTestApp()
	a.focus = focusNotes
	a.notes = []model.Note{{Body: "keep me"}}
	a = upcomingKey(a, "C")
	if a.mode != modeConfirmClearNotes || a.upcoming {
		t.Fatal("C on Notes should still ask to clear the board")
	}
	a = upcomingTestApp()
	a.simple, a.simpleNoteMode = true, true
	a.notes = []model.Note{{Body: "note"}}
	a.tasks = []model.Task{{ID: "today", Date: "2026-09-03"}, {ID: "future", Date: "2026-09-04"}}
	a = upcomingKey(a, "C")
	if !a.upcoming || a.simpleNoteMode || len(a.simpleEntries()) != 1 || a.simpleEntries()[0].isNote {
		t.Fatal("simple Upcoming should expose only future tasks and task input")
	}
	a = upcomingKey(a, "tab")
	if a.upcoming || !a.simpleNoteMode || len(a.simpleEntries()) != 2 {
		t.Fatal("tab should return to the combined list before choosing note input")
	}
	a = upcomingKey(a, "C")
	a = upcomingKey(a, "C")
	if a.upcoming || len(a.simpleEntries()) != 2 {
		t.Fatal("C should restore the combined list")
	}
}

func TestUpcomingMidnightRolloverAndPendingDelete(t *testing.T) {
	for _, simple := range []bool{false, true} {
		t.Run(fmt.Sprintf("simple=%v", simple), func(t *testing.T) {
			a := upcomingTestApp()
			a.simple = simple
			now := a.now()
			a.now = func() time.Time { return now }
			a.tasks = []model.Task{
				{ID: "due", Date: "2026-09-04"},
				{ID: "future", Date: "2026-09-05"},
			}
			a = upcomingKey(a, "C")
			a = upcomingKey(a, "d")
			now = now.AddDate(0, 0, 1)
			updated, _ := a.Update(pomodoroTickMsg(now))
			a = updated.(App)
			if got := upcomingIDs(a.todayTasks()); !reflect.DeepEqual(got, []string{"due"}) {
				t.Fatalf("task did not become due: %v", got)
			}
			a = upcomingKey(a, "y")
			if len(a.tasks) != 2 || a.mode != modeNormal {
				t.Fatal("rollover deleted a different task from the confirmed one")
			}
			a.todaySelected, a.todayScroll = 99, 99
			a.simpleSelected, a.simpleScroll = 99, 99
			now = now.AddDate(0, 0, 2)
			updated, _ = a.Update(pomodoroTickMsg(now))
			a = updated.(App)
			if simple {
				if a.simpleSelected != 0 || a.simpleScroll != 0 {
					t.Fatal("simple selection did not recover from an empty future list")
				}
			} else if a.todaySelected != 0 || a.todayScroll != 0 {
				t.Fatal("pane selection did not recover from an empty future list")
			}
			if len(a.overdueTasks()) != 2 {
				t.Fatal("missed scheduled tasks should use existing overdue behavior")
			}
		})
	}
}

func TestUpcomingDateRenderingAndDimensions(t *testing.T) {
	task := model.Task{ID: "future", Title: "A long scheduled task with a wide character 界", Date: "2026-09-04", Time: "09:30", Kind: model.KindAppointment}
	line := renderTaskLine(task, true, false, 60, colorPaneBg, true)
	if !strings.Contains(line, "2026-09-04 09:30") || task.Time != "09:30" {
		t.Fatal("date and appointment time should render together without changing Task.Time")
	}
	for _, width := range []int{12, 20, 30, 60} {
		if got := lipgloss.Width(renderTaskLine(task, true, false, width, colorPaneBg, true)); got > width {
			t.Fatalf("row width %d exceeds budget %d", got, width)
		}
	}
	for _, simple := range []bool{false, true} {
		for _, lay := range []layout{layoutTasksLeft, layoutTasksRight, layoutStacked, layoutThreeColumn} {
			for _, size := range [][2]int{{100, 30}, {160, 45}} {
				t.Run(fmt.Sprintf("simple=%v/%s/%dx%d", simple, lay, size[0], size[1]), func(t *testing.T) {
					a := upcomingTestApp()
					a.simple, a.layout = simple, lay
					a.width, a.height = size[0], size[1]
					a.tasks = []model.Task{task}
					a = upcomingKey(a, "C")
					page := a.View()
					if !strings.Contains(page, "Upcoming") || !strings.Contains(page, "2026-09-04") {
						t.Fatal("Upcoming title or scheduled date missing")
					}
					if lipgloss.Width(page) > a.width || lipgloss.Height(page) > a.height {
						t.Fatalf("view %dx%d exceeds terminal %dx%d", lipgloss.Width(page), lipgloss.Height(page), a.width, a.height)
					}
				})
			}
		}
	}
}

func TestShowTaskDateSelectsAddedTaskView(t *testing.T) {
	for _, simple := range []bool{false, true} {
		a := upcomingTestApp()
		a.simple = simple
		a.tasks = []model.Task{{ID: "today", Date: "2026-09-03"}, {ID: "future", Date: "2026-09-04"}}
		a.showTaskDate("2026-09-04")
		a.selectTaskByID("future")
		if !a.upcoming {
			t.Fatal("adding a scheduled task should show Upcoming")
		}
		a.showTaskDate("2026-09-03")
		a.selectTaskByID("today")
		if a.upcoming {
			t.Fatal("adding a task today should restore the current-day view")
		}
	}
}

func TestUpcomingRolloverDoesNotRetargetSimpleNoteDeletion(t *testing.T) {
	a := upcomingTestApp()
	a.simple = true
	now := a.now()
	a.now = func() time.Time { return now }
	a.tasks = []model.Task{{ID: "newly-due", Date: "2026-09-04", CreatedAt: now.Add(-time.Hour)}}
	a.notes = []model.Note{{ID: "note", Body: "keep this note", CreatedAt: now}}
	a = upcomingKey(a, "d")
	now = now.AddDate(0, 0, 1)
	updated, _ := a.Update(pomodoroTickMsg(now))
	a = upcomingKey(updated.(App), "y")
	if len(a.tasks) != 1 || len(a.notes) != 1 || a.mode != modeNormal {
		t.Fatal("newly due task changed the target of a note deletion confirmation")
	}
}
