package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func shortcutKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func TestShortcutsInterceptBeforeEveryMode(t *testing.T) {
	for _, m := range []mode{
		modeNormal, modeAdding, modeConfirmDelete, modeNoteEditing,
		modeConfirmClearNotes, modeSettings,
	} {
		a := NewApp(Options{Simple: m == modeNormal})
		a.noPersist = true
		a.width, a.height = 100, 30
		a.mode = m
		before := a
		if m == modeAdding {
			a.input.SetValue("draft?")
			before = a
		}

		model, _ := a.Update(shortcutKey('?'))
		got := model.(App)
		if !got.shortcutsOpen {
			t.Fatalf("mode %v: ? did not open shortcuts", m)
		}
		if got.mode != before.mode {
			t.Errorf("mode %v: help changed underlying mode from %v to %v", m, before.mode, got.mode)
		}
		if m == modeAdding && got.input.Value() != "draft?" {
			t.Errorf("mode %v: ? was inserted into the editor: %q", m, got.input.Value())
		}

		model, _ = got.Update(tea.KeyMsg{Type: tea.KeyEsc})
		closed := model.(App)
		if closed.shortcutsOpen || closed.mode != before.mode {
			t.Errorf("mode %v: Esc did not close help while preserving mode", m)
		}
	}
}

func TestShortcutsCloseAndCtrlCBehavior(t *testing.T) {
	a := NewApp(Options{})
	a.noPersist = true
	a.width, a.height = 100, 30
	a.shortcutsOpen = true

	model, _ := a.Update(shortcutKey('q'))
	if got := model.(App); got.shortcutsOpen {
		t.Fatal("q did not close shortcuts")
	}

	a.shortcutsOpen = true
	model, cmd := a.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !model.(App).shortcutsOpen {
		t.Fatal("ctrl+c closed shortcuts instead of retaining global quit behavior")
	}
	if cmd == nil || cmd() != tea.Quit() {
		t.Fatal("ctrl+c did not return tea.Quit while shortcuts were open")
	}
}

func TestShortcutsModalFitsNarrowHeightsAndWidths(t *testing.T) {
	for _, size := range [][2]int{{30, 24}, {42, 12}, {80, 6}, {12, 5}} {
		a := NewApp(Options{})
		a.noPersist = true
		a.width, a.height = size[0], size[1]
		a.shortcutsOpen = true

		modal := a.renderShortcutsModal()
		for i, line := range strings.Split(modal, "\n") {
			if got := lipgloss.Width(line); got != a.shortcutsModalWidth() {
				t.Errorf("%dx%d modal line %d width = %d, want %d", a.width, a.height, i, got, a.shortcutsModalWidth())
			}
		}
		view := a.View()
		for i, line := range strings.Split(view, "\n") {
			if got := lipgloss.Width(line); got != a.width {
				t.Errorf("%dx%d page line %d width = %d, want %d", a.width, a.height, i, got, a.width)
			}
		}
	}
}
