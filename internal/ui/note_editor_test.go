package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/parsaenami/taskii/internal/model"
)

func noteEditorTestApp(t *testing.T, simple, expanded bool) App {
	t.Helper()
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
	a := NewApp(Options{Mock: true, Simple: simple})
	a.notes, a.routines = nil, nil
	a.focus = focusNotes
	a.layout = layoutThreeColumn
	a.width, a.height = 100, 30
	a.notesExpanded = expanded
	m, _ := a.startNoteEdit(-1)
	return m.(App)
}

func noteEditorKey(a App, key tea.KeyType) App {
	m, _ := a.Update(tea.KeyMsg{Type: key})
	return m.(App)
}

func noteEditorType(a App, value string) App {
	m, _ := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)})
	return m.(App)
}

func noteEditorWidth(a App) int {
	if a.simple {
		return a.simpleListWidth()
	}
	return a.geometry().notesWidth - 4
}

// Locate the final caret by its accent background, measuring display cells
// before it rather than byte/rune indices (Unicode text need not be one cell).
func noteEditorCaret(t *testing.T, a App) (row, col int, char string) {
	t.Helper()
	bg := colorPaneBg
	if a.simple {
		bg = colorBg
	}
	styled := lipgloss.NewStyle().Foreground(bg).Background(colorAccent).Render("X")
	marker := styled[:strings.Index(styled, "X")]
	rendered := a.renderNoteEditor(noteEditorWidth(a))
	count := 0
	for r, line := range strings.Split(rendered, "\n") {
		if i := strings.Index(line, marker); i >= 0 {
			count++
			row, col = r, lipgloss.Width(line[:i])
			tail := line[i+len(marker):]
			char = tail[:strings.IndexByte(tail, 0x1b)]
		}
	}
	if count != 1 {
		t.Fatalf("expected one visible caret, got %d:\n%s", count, rendered)
	}
	return
}

func requireNoteCaret(t *testing.T, a App, wantRow, wantCol int, wantChar string) {
	t.Helper()
	r, c, char := noteEditorCaret(t, a)
	if r != wantRow || c != wantCol || char != wantChar {
		t.Fatalf("caret = (%d,%d,%q), want (%d,%d,%q)\n%s", r, c, char,
			wantRow, wantCol, wantChar, a.renderNoteEditor(noteEditorWidth(a)))
	}
}

func TestNoteEditorNewlinesAndScrolledNavigation(t *testing.T) {
	for _, mode := range []string{"pane", "expanded", "simple"} {
		t.Run(mode, func(t *testing.T) {
			a := noteEditorTestApp(t, mode == "simple", mode == "expanded")
			a = noteEditorType(a, "kkjdcj")
			requireNoteCaret(t, a, 1, 7, " ")
			a = noteEditorKey(a, tea.KeyCtrlJ)
			requireNoteCaret(t, a, 2, 1, " ")
			a = noteEditorType(a, "kjkdjk")
			a = noteEditorKey(a, tea.KeyCtrlJ)
			requireNoteCaret(t, a, 3, 1, " ")
			a = noteEditorType(a, "third")
			a = noteEditorKey(a, tea.KeyCtrlJ)
			a = noteEditorType(a, "fourth")
			requireNoteCaret(t, a, 3, 7, " ")
			a = noteEditorKey(a, tea.KeyUp)
			a = noteEditorKey(a, tea.KeyHome)
			requireNoteCaret(t, a, 2, 1, "t")
			a = noteEditorType(a, "X")
			requireNoteCaret(t, a, 2, 2, "t")
			if got := a.noteInput.Value(); got != "kkjdcj\nkjkdjk\nXthird\nfourth" {
				t.Fatalf("insertion disagrees with visible caret: %q", got)
			}
		})
	}
}

func TestNoteEditorWrapScrollResizeAndUnicode(t *testing.T) {
	for _, mode := range []string{"pane", "expanded", "simple"} {
		t.Run(mode, func(t *testing.T) {
			a := noteEditorTestApp(t, mode == "simple", mode == "expanded")
			// A single input event must populate the viewport before scrolling;
			// no intermediate View or terminal resize is needed to display it.
			a = noteEditorType(a, strings.Repeat("wrapped words ", 100)+"END")
			r, _, _ := noteEditorCaret(t, a)
			if r != 3 || !strings.Contains(ansiRe.ReplaceAllString(a.renderNoteEditor(noteEditorWidth(a)), ""), "END") {
				t.Fatal("long wrapped note did not follow its caret into view")
			}
			for _, size := range [][2]int{{70, 24}, {180, 45}, {90, 30}} {
				m, _ := a.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
				a = m.(App)
				noteEditorCaret(t, a)
				plain := ansiRe.ReplaceAllString(a.renderNoteEditor(noteEditorWidth(a)), "")
				if !strings.Contains(plain, "END") {
					t.Fatalf("resize lost cursor content: %q", plain)
				}
				w, h, _ := noteEditorInterior(noteEditorWidth(a), a.noteEditorHeight())
				if a.noteInput.Width() != w || a.noteInput.Height() != h {
					t.Fatal("textarea geometry was not committed on resize")
				}
			}
			a = noteEditorKey(a, tea.KeyCtrlJ)
			a = noteEditorType(a, "界🙂e\u0301Z")
			r, _, _ = noteEditorCaret(t, a)
			requireNoteCaret(t, a, r, 7, " ") // 2 + 2 + 1 + 1 display cells
			a = noteEditorKey(a, tea.KeyLeft)
			requireNoteCaret(t, a, r, 6, "Z")
			a = noteEditorKey(a, tea.KeyLeft)
			requireNoteCaret(t, a, r, 5, "e\u0301")
			a = noteEditorKey(a, tea.KeyLeft)
			requireNoteCaret(t, a, r, 5, "e\u0301")
			a = noteEditorKey(a, tea.KeyLeft)
			requireNoteCaret(t, a, r, 3, "🙂")
			a = noteEditorKey(a, tea.KeyRight)
			a = noteEditorKey(a, tea.KeyRight)
			a = noteEditorKey(a, tea.KeyRight)
			a = noteEditorType(a, "!")
			if !strings.HasSuffix(a.noteInput.Value(), "界🙂e\u0301!Z") {
				t.Fatal("Unicode insertion corrupted the body")
			}
		})
	}
}

func TestNoteEditorExistingMultilineNote(t *testing.T) {
	for _, mode := range []string{"pane", "expanded", "simple"} {
		t.Run(mode, func(t *testing.T) {
			isolateUICalendarData(t)
			a := noteEditorTestApp(t, mode == "simple", mode == "expanded")
			first := strings.Repeat("wrapped words ", 20) + "first"
			a.notes = []model.Note{{ID: "existing", Body: first + "\nsecond\nthird\nfourth"}}
			m, _ := a.startNoteEdit(0)
			a = m.(App)
			requireNoteCaret(t, a, 3, 7, " ")
			a = noteEditorKey(a, tea.KeyUp)
			a = noteEditorKey(a, tea.KeyHome)
			requireNoteCaret(t, a, 2, 1, "t")
			a = noteEditorType(a, "edited ")
			requireNoteCaret(t, a, 2, 8, "t")
			// UI TestMain isolates XDG paths; exercise the real atomic save too.
			a.noPersist = false
			a = noteEditorKey(a, tea.KeyEnter)
			if a.mode != modeNormal || len(a.notes) != 1 || a.notes[0].ID != "existing" ||
				a.notes[0].Body != first+"\nsecond\nedited third\nfourth" {
				t.Fatalf("edit/save corrupted an existing note: mode=%v notes=%+v", a.mode, a.notes)
			}
			stored, err := model.LoadNotes()
			if err != nil || len(stored) != 1 || stored[0].Body != a.notes[0].Body || stored[0].ID != "existing" {
				t.Fatalf("saved body did not round-trip: %+v, %v", stored, err)
			}
		})
	}
}

func TestNoteEditorBordersAndFrameGeometry(t *testing.T) {
	for _, theme := range themes {
		for _, lay := range []layout{layoutTasksLeft, layoutTasksRight, layoutStacked, layoutThreeColumn} {
			for _, mode := range []string{"pane", "expanded", "simple"} {
				for _, size := range [][2]int{{70, 24}, {100, 30}, {160, 45}} {
					t.Run(fmt.Sprintf("%s/%d/%s/%dx%d", theme.Name, lay, mode, size[0], size[1]), func(t *testing.T) {
						a := noteEditorTestApp(t, mode == "simple", mode == "expanded")
						applyTheme(theme)
						t.Cleanup(setDefaultTheme)
						a.layout = lay
						m, _ := a.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
						a = m.(App)
						if !a.simple && a.geometry().notesHeight == 0 {
							return // This layout omits Notes entirely at this height.
						}
						a = noteEditorType(a, strings.Repeat("long note ", 40)+"END")
						editor := a.renderNoteEditor(noteEditorWidth(a))
						plain := ansiRe.ReplaceAllString(editor, "")
						if !strings.HasPrefix(plain, "╭") || !strings.HasSuffix(plain, "╯") || lipgloss.Height(editor) != a.noteEditorHeight() {
							t.Fatalf("editor border/height is wrong: %q", plain)
						}
						for _, line := range strings.Split(editor, "\n") {
							if lipgloss.Width(line) != noteEditorWidth(a) {
								t.Fatal("editor width exceeds its budget")
							}
						}
						noteEditorCaret(t, a)
						frame := a.View()
						if lipgloss.Height(frame) != a.height {
							t.Fatalf("frame height=%d, want %d", lipgloss.Height(frame), a.height)
						}
						for _, line := range strings.Split(frame, "\n") {
							if lipgloss.Width(line) != a.width {
								t.Fatalf("frame width=%d, want %d", lipgloss.Width(line), a.width)
							}
						}
						if err := requireBackgroundEveryCell(frame); err != nil {
							t.Fatal(err)
						}
						// The complete editor (especially its caret and bottom edge)
						// must survive assembly, including the smallest Notes pane.
						for _, line := range strings.Split(editor, "\n") {
							if !strings.Contains(frame, line) {
								t.Fatal("frame clipped an editor row")
							}
						}
						a = noteEditorKey(a, tea.KeyEnter)
						noteEditorCaret(t, a) // Batch-add reset keeps a visible placeholder caret.
					})
				}
			}
		}
	}
}
