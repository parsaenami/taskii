package ui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"taskii/internal/model"
)

func newTestApp() App {
	a := NewApp(Options{Mock: true})
	a.now = func() time.Time { return time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC) }
	a.tasks = nil
	a.notes = nil
	a.width, a.height = 120, 40
	return a
}

func pressKey(a App, key string) App {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	switch key {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	}
	updated, _ := a.Update(msg)
	return updated.(App)
}

func TestTaskEditing(t *testing.T) {
	for _, simple := range []bool{false, true} {
		for _, overdue := range []bool{false, true} {
			t.Run(fmt.Sprintf("simple=%t/overdue=%t", simple, overdue), func(t *testing.T) {
				a := newTestApp()
				a.simple = simple
				date := a.now().Format(dateFormat)
				if overdue {
					date = a.now().AddDate(0, 0, -1).Format(dateFormat)
				}
				doneAt := a.now().Add(-time.Hour)
				original := model.Task{
					ID: "edited", Title: "Купить молоко", Kind: model.KindAppointment,
					Time: "09:00", Date: date, CreatedAt: a.now().Add(-2 * time.Hour),
					Important: true, Done: !overdue,
				}
				if original.Done {
					original.DoneAt = &doneAt
				}
				other := model.Task{ID: "other", Title: "Other", Kind: model.KindAppointment,
					Time: "12:00", Date: date, CreatedAt: a.now()}
				a.tasks = []model.Task{original, other}
				a.notes = []model.Note{{Body: "Unrelated note"}}
				a.selectTaskByID(original.ID)
				a = pressKey(a, "E")
				if a.mode != modeEditing || a.taskEditID != original.ID || a.input.Value() != "Купить молоко 09:00" {
					t.Fatalf("editor was not prefilled: mode=%v id=%q input=%q", a.mode, a.taskEditID, a.input.Value())
				}
				a.input.SetValue("Купить хлеб 20:00")
				a = pressKey(a, "enter")
				want := original
				want.Title, want.Time = "Купить хлеб", "20:00"
				if !reflect.DeepEqual(a.tasks, []model.Task{want, other}) {
					t.Fatalf("editing changed unexpected fields: got %+v, want %+v", a.tasks, []model.Task{want, other})
				}
				if a.mode != modeNormal || a.taskEditID != "" || a.input.Focused() {
					t.Fatal("saving did not close the editor")
				}
				if selected := a.selectedTask(); selected == nil || selected.ID != original.ID {
					t.Fatal("selection did not follow the edited task")
				}
				if !simple && overdue && a.focus != focusOverdue {
					t.Fatal("editing moved the overdue task to Today")
				}
			})
		}
	}
}

func TestTaskInputParsing(t *testing.T) {
	for _, tc := range []struct {
		raw, title, taskTime string
		kind                 model.Kind
	}{
		{"  Купить молоко  ", "Купить молоко", "", model.KindTask},
		{"Meeting 00:00", "Meeting", "00:00", model.KindAppointment},
		{"Meeting 23:59", "Meeting", "23:59", model.KindAppointment},
		{"Meeting 24:00", "Meeting 24:00", "", model.KindTask},
		{"Meeting 09:60", "Meeting 09:60", "", model.KindTask},
		{"At 12:00 today", "At 12:00 today", "", model.KindTask},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			a := newTestApp()
			a.addTask(tc.raw)
			if len(a.tasks) != 1 {
				t.Fatal("add failed")
			}
			added := a.tasks[0]
			if added.Title != tc.title || added.Kind != tc.kind || added.Time != tc.taskTime {
				t.Fatalf("unexpected parsed task: %+v", added)
			}
			a = pressKey(a, "E")
			a = pressKey(a, "enter")
			if !reflect.DeepEqual(a.tasks[0], added) {
				t.Fatal("saving unchanged input modified the task")
			}
			a = pressKey(a, "E")
			a.input.SetValue("Renamed")
			a = pressKey(a, "enter")
			if a.tasks[0].Kind != model.KindTask || a.tasks[0].Time != "" {
				t.Fatal("removing time did not convert the appointment to a task")
			}
			a = pressKey(a, "E")
			a.input.SetValue(tc.raw)
			a = pressKey(a, "enter")
			if !reflect.DeepEqual(a.tasks[0], added) {
				t.Fatal("edit and add parsed the same input differently")
			}
		})
	}
}

func TestTaskEditCancelAndEmptyInput(t *testing.T) {
	for _, raw := range []string{"", "   ", "12:00"} {
		t.Run(fmt.Sprintf("input=%q", raw), func(t *testing.T) {
			a := newTestApp()
			a.addTask("Original 10:00")
			original := a.tasks[0]
			a = pressKey(a, "E")
			a.input.SetValue(raw)
			a = pressKey(a, "enter")
			if a.mode != modeEditing || !reflect.DeepEqual(a.tasks[0], original) {
				t.Fatal("empty title was saved")
			}
			a.input.SetValue("Discard this")
			a = pressKey(a, "esc")
			if a.mode != modeNormal || a.taskEditID != "" || a.input.Focused() || !reflect.DeepEqual(a.tasks[0], original) {
				t.Fatal("cancel did not discard the edit")
			}
		})
	}
}

func TestTaskEditGates(t *testing.T) {
	for _, focus := range []focusedPane{focusToday, focusOverdue, focusNotes, focusReports} {
		a := newTestApp()
		a.focus = focus
		if edited := pressKey(a, "E"); edited.mode != modeNormal {
			t.Fatalf("empty pane %v opened an editor", focus)
		}
	}
	a := newTestApp()
	a.addTask("Hidden task")
	for _, focus := range []focusedPane{focusNotes, focusReports} {
		a.focus = focus
		if edited := pressKey(a, "E"); edited.mode != modeNormal {
			t.Fatalf("pane %v opened a task editor", focus)
		}
	}
	a.focus, a.notesExpanded = focusNotes, true
	if edited := pressKey(a, "E"); edited.mode != modeNormal || !edited.notesExpanded {
		t.Fatal("task editing escaped expanded Notes")
	}
	a.simple = true
	a.notes = []model.Note{{Body: "Selected note"}}
	a.simpleSelected = 0
	if edited := pressKey(a, "E"); edited.mode != modeNormal {
		t.Fatal("simple mode opened task editing for a note")
	}
}

func TestTaskEditPersistsByID(t *testing.T) {
	t.Chdir(t.TempDir())
	a := newTestApp()
	a.tasks = []model.Task{
		{ID: "first", Title: "First", Date: a.now().Format(dateFormat)},
		{ID: "second", Title: "Second", Date: a.now().Format(dateFormat)},
	}
	a.noPersist = false
	a = pressKey(a, "E")
	// The edit target must be independent of the current list position.
	a.todaySelected = 1
	a.input.SetValue("Updated 14:30")
	a = pressKey(a, "enter")
	loaded, err := model.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, a.tasks) || loaded[0].Title != "Updated" || loaded[1].Title != "Second" {
		t.Fatalf("wrong edit persisted: %+v", loaded)
	}
}

func TestTaskEditRendering(t *testing.T) {
	for _, simple := range []bool{false, true} {
		for _, lay := range []layout{layoutTasksLeft, layoutTasksRight, layoutStacked, layoutThreeColumn} {
			for _, focus := range []focusedPane{focusToday, focusOverdue} {
				t.Run(fmt.Sprintf("simple=%t/layout=%v/focus=%v", simple, lay, focus), func(t *testing.T) {
					a := newTestApp()
					a.simple, a.layout, a.focus = simple, lay, focus
					date := a.now().Format(dateFormat)
					if focus == focusOverdue {
						date = a.now().AddDate(0, 0, -1).Format(dateFormat)
					}
					a.tasks = []model.Task{{ID: "long", Title: strings.Repeat("длинный ", 30),
						Kind: model.KindAppointment, Time: "23:59", Date: date}}
					a = pressKey(a, "E")
					if a.input.Value() != a.tasks[0].Title+" 23:59" {
						t.Fatal("long prefill was truncated")
					}
					for _, width := range []int{120, 80, 160} {
						updated, _ := a.Update(tea.WindowSizeMsg{Width: width, Height: 40})
						a = updated.(App)
						page := a.View()
						plain := ansi.Strip(page)
						if lipgloss.Width(page) != width || lipgloss.Height(page) != 40 {
							t.Fatalf("wrong frame size at %d: %dx%d", width, lipgloss.Width(page), lipgloss.Height(page))
						}
						if !strings.Contains(plain, "~ ") || !strings.Contains(plain, "23:59") || !strings.Contains(plain, "[enter] save") {
							t.Fatal("editor, appointment time or save hint missing")
						}
					}
				})
			}
		}
	}
}
