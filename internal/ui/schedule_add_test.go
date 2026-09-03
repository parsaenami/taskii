package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"taskii/internal/model"
)

func TestScheduledInputCreatesAndSelectsTask(t *testing.T) {
	for _, simple := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "simple"}[simple], func(t *testing.T) {
			a := NewApp(Options{Mock: true, Simple: simple})
			a.tasks, a.notes = nil, nil
			now := time.Date(2026, time.September, 3, 12, 0, 0, 0, time.UTC)
			a.now = func() time.Time { return now }
			a.width, a.height = 120, 40
			a.mode = modeAdding
			a.input.SetValue("Read a chapter 09-05")
			updated, _ := a.Update(tea.KeyMsg{Type: tea.KeyEnter})
			a = updated.(App)
			if len(a.tasks) != 1 {
				t.Fatalf("created %d tasks, want 1", len(a.tasks))
			}
			task := a.tasks[0]
			if task.Title != "Read a chapter" || task.Date != "2026-09-05" || task.Time != "" || task.Kind != model.KindTask {
				t.Fatalf("unexpected scheduled task: %+v", task)
			}
			if !a.upcoming || len(a.todayTasks()) != 0 || a.input.Value() != "" {
				t.Fatal("new future task must be visible in Upcoming and input cleared")
			}
			if simple {
				entries := a.simpleEntries()
				if len(entries) != 1 || entries[a.simpleSelected].task.ID != task.ID {
					t.Fatal("simple selection did not follow scheduled task")
				}
			} else if selected := a.selectedTask(); selected == nil || selected.ID != task.ID {
				t.Fatal("selection did not follow scheduled task")
			}

			// Continue the existing batch-add flow with a legacy appointment.
			now = now.Add(time.Second)
			a.input.SetValue("Call Alex 14:30")
			updated, _ = a.Update(tea.KeyMsg{Type: tea.KeyEnter})
			a = updated.(App)
			if len(a.tasks) != 2 || a.upcoming {
				t.Fatal("legacy appointment must switch back to the current day")
			}
			if task = a.tasks[1]; task.Date != "2026-09-03" || task.Kind != model.KindAppointment || task.Time != "14:30" {
				t.Fatalf("legacy appointment changed: %+v", task)
			}
		})
	}
}

func TestInvalidScheduledInputCanBeCorrected(t *testing.T) {
	a := NewApp(Options{Mock: true})
	a.tasks = nil
	a.now = func() time.Time { return time.Date(2026, time.September, 3, 12, 0, 0, 0, time.UTC) }
	a.width, a.height = 120, 40
	a.mode = modeAdding
	a.input.Focus()
	const invalid = "Read 02-30"
	a.input.SetValue(invalid)
	updated, _ := a.Update(tea.KeyMsg{Type: tea.KeyEnter})
	a = updated.(App)
	if len(a.tasks) != 0 || a.input.Value() != invalid || a.err == "" || a.mode != modeAdding || !a.input.Focused() {
		t.Fatal("invalid date must preserve the input and display an error")
	}
	a.input.SetValue("Read 09-05")
	updated, _ = a.Update(tea.KeyMsg{Type: tea.KeyEnter})
	a = updated.(App)
	if len(a.tasks) != 1 || a.input.Value() != "" || a.err != "" {
		t.Fatal("corrected input must save once and clear the validation error")
	}
}

func TestScheduledTaskPersistsExistingDateField(t *testing.T) {
	t.Chdir(t.TempDir())
	a := NewApp(Options{Mock: true})
	a.tasks = nil
	a.noPersist = false
	a.now = func() time.Time { return time.Date(2026, time.September, 3, 12, 0, 0, 0, time.UTC) }
	a.width, a.height = 120, 40
	if !a.addTask("Call Alex 09-05 14:30") || a.err != "" {
		t.Fatalf("add failed: %s", a.err)
	}
	loaded, err := model.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0] != a.tasks[0] {
		t.Fatalf("scheduled task did not survive reload: %+v", loaded)
	}
}
