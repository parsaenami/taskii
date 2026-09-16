package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"taskii/internal/model"
)

func TestEnterEditsTask(t *testing.T) {
	a := NewApp(Options{})
	a.noPersist = true
	a.width, a.height = 100, 30
	a.now = testNow
	a.tasks = []model.Task{{ID: "x", Title: "Original", Date: "2026-09-13"}}
	a.focus = focusToday
	a.todaySelected = 0

	m, _ := a.updateNormal(tea.KeyMsg{Type: tea.KeyEnter})
	a = m.(App)
	if a.mode != modeAdding {
		t.Fatalf("enter should open the inline editor, got mode %v", a.mode)
	}
	if a.taskEditID != "x" {
		t.Fatalf("taskEditID = %q, want the selected task", a.taskEditID)
	}
	if got := a.input.Value(); got != "Original" {
		t.Fatalf("editor should be pre-filled, got %q", got)
	}

	a.input.SetValue("Renamed")
	m, _ = a.updateAdding(tea.KeyMsg{Type: tea.KeyEnter})
	a = m.(App)
	if a.tasks[0].Title != "Renamed" {
		t.Errorf("title = %q, want Renamed", a.tasks[0].Title)
	}
	if len(a.tasks) != 1 {
		t.Errorf("editing must not add a task; have %d", len(a.tasks))
	}
	// Unlike adding, a completed edit closes the editor — reopening it blank
	// would be a different action than the one requested.
	if a.mode != modeNormal || a.taskEditID != "" {
		t.Errorf("editor should close after saving, got mode=%v editID=%q", a.mode, a.taskEditID)
	}
}

// TestEnterDoesNotEditInOverdue pins editing as Today-only. The inline input
// renders inside the Today pane — the same line modeAdding uses — so an edit
// started from Overdue opened the editor in a different box than the row
// being edited. Space carries the task forward; it's edited in Today.
func TestEnterDoesNotEditInOverdue(t *testing.T) {
	a := NewApp(Options{})
	a.noPersist = true
	a.width, a.height = 100, 30
	a.now = testNow
	a.tasks = []model.Task{
		{ID: "today", Title: "Today task", Date: "2026-09-13"},
		{ID: "old", Title: "Old task", Date: "2026-09-10"},
	}
	a.focus = focusOverdue
	a.overdueSelected = 0

	m, _ := a.updateNormal(tea.KeyMsg{Type: tea.KeyEnter})
	a = m.(App)

	if a.mode != modeNormal {
		t.Errorf("enter in Overdue should be inert, got mode %v", a.mode)
	}
	if a.taskEditID != "" {
		t.Errorf("enter in Overdue set an edit target: %q", a.taskEditID)
	}

	// And the help bar must not advertise a key that does nothing there.
	var keys []string
	for _, g := range a.helpGroups() {
		for _, k := range g.keys {
			keys = append(keys, k.key)
		}
	}
	for _, k := range keys {
		if k == "enter" {
			t.Errorf("Overdue help bar still offers [enter]: %v", keys)
		}
	}
}

// Simple mode is the exception: one merged list and one input line, so an
// overdue row there has no second pane to land in and stays editable. It also
// must edit the row under the cursor, not whatever a.focus/todaySelected
// happen to hold — neither applies in this mode.
func TestSimpleModeEditsSelectedOverdueTask(t *testing.T) {
	a := NewApp(Options{Simple: true})
	a.noPersist = true
	a.width, a.height = 100, 30
	a.now = testNow
	a.tasks = []model.Task{
		{ID: "today", Title: "Today task", Date: "2026-09-13", CreatedAt: testNow()},
		{ID: "old", Title: "Old task", Date: "2026-09-10", CreatedAt: testNow().Add(-time.Hour)},
	}

	// Find the overdue row and put the cursor on it.
	entries := a.simpleEntries()
	idx := -1
	for i, e := range entries {
		if !e.isNote && e.task.ID == "old" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("overdue task missing from the merged list: %+v", entries)
	}
	a.simpleSelected = idx

	m, _ := a.updateNormal(tea.KeyMsg{Type: tea.KeyEnter})
	a = m.(App)

	if a.mode != modeAdding {
		t.Fatalf("enter should open the editor in simple mode, got %v", a.mode)
	}
	if a.taskEditID != "old" {
		t.Errorf("edited the wrong task: taskEditID = %q, want \"old\"", a.taskEditID)
	}
	if got := a.input.Value(); got != "Old task" {
		t.Errorf("editor pre-filled with %q, want the selected row's title", got)
	}
}

func TestEditRoundTripsAppointment(t *testing.T) {
	a := NewApp(Options{})
	a.noPersist = true
	a.width, a.height = 100, 30
	a.now = testNow
	a.tasks = []model.Task{{
		ID: "x", Title: "Standup", Time: "09:30",
		Kind: model.KindAppointment, Date: "2026-09-13",
	}}
	a.focus = focusToday

	m, _ := a.updateNormal(tea.KeyMsg{Type: tea.KeyEnter})
	a = m.(App)
	// The editor shows the same "title HH:MM" syntax addTask parses, so
	// saving unchanged must leave the appointment intact.
	if got := a.input.Value(); got != "Standup 09:30" {
		t.Fatalf("editor value = %q, want the round-trippable form", got)
	}
	m, _ = a.updateAdding(tea.KeyMsg{Type: tea.KeyEnter})
	a = m.(App)
	if a.tasks[0].Time != "09:30" || !a.tasks[0].IsAppointment() {
		t.Errorf("appointment lost on round-trip: %+v", a.tasks[0])
	}

	// Dropping the time demotes it back to a plain task.
	a.focus = focusToday
	m, _ = a.updateNormal(tea.KeyMsg{Type: tea.KeyEnter})
	a = m.(App)
	a.input.SetValue("Standup")
	m, _ = a.updateAdding(tea.KeyMsg{Type: tea.KeyEnter})
	a = m.(App)
	if a.tasks[0].IsAppointment() || a.tasks[0].Time != "" {
		t.Errorf("expected demotion to a plain task, got %+v", a.tasks[0])
	}
}

func TestEditEmptyTitleIsNoOp(t *testing.T) {
	a := NewApp(Options{})
	a.noPersist = true
	a.width, a.height = 100, 30
	a.now = testNow
	a.tasks = []model.Task{{ID: "x", Title: "Keep me", Date: "2026-09-13"}}
	a.focus = focusToday

	m, _ := a.updateNormal(tea.KeyMsg{Type: tea.KeyEnter})
	a = m.(App)
	a.input.SetValue("   ")
	m, _ = a.updateAdding(tea.KeyMsg{Type: tea.KeyEnter})
	a = m.(App)
	// `d` is the deliberate way to delete; an emptied editor must not do it.
	if len(a.tasks) != 1 || a.tasks[0].Title != "Keep me" {
		t.Errorf("emptying the editor should not delete or blank the task: %+v", a.tasks)
	}
}

func TestAddAfterEditDoesNotOverwrite(t *testing.T) {
	a := NewApp(Options{})
	a.noPersist = true
	a.width, a.height = 100, 30
	a.now = testNow
	a.tasks = []model.Task{{ID: "x", Title: "Existing", Date: "2026-09-13"}}
	a.focus = focusToday

	// Edit, cancel, then add: the stale edit target must not hijack the add.
	m, _ := a.updateNormal(tea.KeyMsg{Type: tea.KeyEnter})
	a = m.(App)
	m, _ = a.updateAdding(tea.KeyMsg{Type: tea.KeyEsc})
	a = m.(App)
	m, _ = a.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	a = m.(App)
	if a.taskEditID != "" {
		t.Fatalf("add should clear the edit target, got %q", a.taskEditID)
	}
	a.input.SetValue("Brand new")
	m, _ = a.updateAdding(tea.KeyMsg{Type: tea.KeyEnter})
	a = m.(App)
	if len(a.tasks) != 2 {
		t.Fatalf("expected an added task, have %d", len(a.tasks))
	}
	if a.tasks[0].Title != "Existing" {
		t.Errorf("add overwrote the previously edited task: %q", a.tasks[0].Title)
	}
}

// TestNoteInputStaysOpenWhenAdding pins the notes board to the same rhythm as
// the task input: enter files the note and leaves the editor ready for the
// next one, and esc is the deliberate way out.
func TestNoteInputStaysOpenWhenAdding(t *testing.T) {
	a := NewApp(Options{})
	a.noPersist = true
	a.width, a.height = 100, 30
	a.now = testNow
	a.focus = focusNotes

	m, _ := a.startNoteEdit(-1)
	a = m.(App)
	a.noteInput.SetValue("first")
	m, _ = a.updateNoteEditing(tea.KeyMsg{Type: tea.KeyEnter})
	a = m.(App)

	if a.mode != modeNoteEditing {
		t.Fatalf("editor should stay open after adding, got mode %v", a.mode)
	}
	if got := a.noteInput.Value(); got != "" {
		t.Errorf("editor should be cleared for the next note, got %q", got)
	}
	if len(a.notes) != 1 || a.notes[0].Body != "first" {
		t.Fatalf("note not saved: %+v", a.notes)
	}

	a.noteInput.SetValue("second")
	m, _ = a.updateNoteEditing(tea.KeyMsg{Type: tea.KeyEnter})
	a = m.(App)
	if len(a.notes) != 2 {
		t.Errorf("expected two notes without reopening the editor, have %d", len(a.notes))
	}

	m, _ = a.updateNoteEditing(tea.KeyMsg{Type: tea.KeyEsc})
	a = m.(App)
	if a.mode != modeNormal {
		t.Errorf("esc should leave the editor, got mode %v", a.mode)
	}
}

// Editing an EXISTING note still closes on save: the user asked to change one
// note, not to start a batch.
func TestNoteEditClosesOnSave(t *testing.T) {
	a := NewApp(Options{})
	a.noPersist = true
	a.width, a.height = 100, 30
	a.now = testNow
	a.notes = []model.Note{{ID: "n1", Body: "original"}}
	a.focus = focusNotes
	a.notesSelected = 0

	m, _ := a.startNoteEdit(0)
	a = m.(App)
	a.noteInput.SetValue("edited")
	m, _ = a.updateNoteEditing(tea.KeyMsg{Type: tea.KeyEnter})
	a = m.(App)

	if a.mode != modeNormal {
		t.Errorf("editing an existing note should close the editor, got %v", a.mode)
	}
	if len(a.notes) != 1 || a.notes[0].Body != "edited" {
		t.Errorf("note not updated in place: %+v", a.notes)
	}
}
