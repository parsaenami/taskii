package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/rivo/uniseg"
)

// Three editable rows plus the rounded border, shrinking to a single editable
// row when Notes is cramped. The editor takes priority over the board rows.
func (a App) noteEditorHeight() int {
	available := a.geometry().notesHeight - 2
	if a.simple {
		available = a.simpleBodyHeight() - simpleTabsHeight
		if !a.upcoming && len(a.dueRoutines()) > 0 {
			available--
		}
	}
	return max(1, min(5, available))
}

func noteEditorInterior(width, height int) (int, int, bool) {
	bordered := width >= 3 && height >= 3
	if bordered {
		return width - 2, height - 2, true
	}
	return max(1, width), max(1, height), false
}

// Geometry must be committed before textarea input/navigation, not on the View
// copy: the widget calculates wrapping and viewport offsets during Update.
func (a *App) syncNoteInputGeometry() {
	width := a.geometry().notesWidth - 4
	if a.simple {
		width = a.simpleListWidth()
	}
	w, h, _ := noteEditorInterior(width, a.noteEditorHeight())
	a.noteInput.SetWidth(w)
	a.noteInput.SetHeight(h)
	a.refreshNoteInputViewport()
}

// textarea.SetValue/InsertString/SetWidth/SetHeight do not reposition the
// viewport. Populate its content first (View updates the widget's viewport),
// then let Update reposition it against the current wrapped content. Otherwise
// a long insertion can be clamped against stale content and leave a blank view.
func (a *App) refreshNoteInputViewport() {
	_ = a.noteInput.View()
	a.noteInput, _ = a.noteInput.Update(nil)
}

func joinNoteEditor(board string, boardRows int, editor string) string {
	if boardRows <= 0 {
		return editor
	}
	lines := strings.Split(board, "\n")
	if len(lines) > boardRows {
		lines = lines[:boardRows]
	}
	return strings.Join(lines, "\n") + "\n" + editor
}

// The native caret is marked by reverse-video SGR in the textarea's actual
// viewport output. Preserve that span while rebuilding plain text with themed
// backgrounds; logical Line()/ColumnOffset cannot identify a viewport cell.
// No pre-styled ANSI is fed back into a lipgloss Render call.
func restyleNoteInput(line string, text, caret lipgloss.Style) string {
	var plain strings.Builder
	pos := 0
	caretStart, caretEnd := -1, -1
	reverse := false
	write := func(s string) {
		if reverse && s != "" {
			caretStart, caretEnd = plain.Len(), plain.Len()+len(s)
		}
		plain.WriteString(s)
	}
	for _, loc := range ansiRe.FindAllStringIndex(line, -1) {
		write(line[pos:loc[0]])
		for _, code := range strings.Split(line[loc[0]+2:loc[1]-1], ";") {
			switch code {
			case "", "0", "27":
				reverse = false
			case "7":
				reverse = true
			}
		}
		pos = loc[1]
	}
	write(line[pos:])
	value := plain.String()
	if caretStart < 0 {
		return text.Render(value)
	}
	// The widget moves in runes, so its marker can split a combining sequence
	// or an emoji grapheme. Color the whole marked grapheme without separating
	// its code points with SGR resets or losing a zero-width caret entirely.
	graphemes := uniseg.NewGraphemes(value)
	start, end := caretStart, caretEnd
	for graphemes.Next() {
		from, to := graphemes.Positions()
		if from < caretEnd && to > caretStart {
			start, end = min(start, from), max(end, to)
		}
	}
	return text.Render(value[:start]) + caret.Render(value[start:end]) + text.Render(value[end:])
}

// Shared by the pane, expanded Notes, and simple mode. Border glyphs, text,
// caret, and filler are styled independently, with exact cell budgets.
func (a App) renderNoteEditor(width int) string {
	height := a.noteEditorHeight()
	w, h, bordered := noteEditorInterior(width, height)
	bg := colorPaneBg
	if a.simple {
		bg = colorBg
	}
	fg := colorText
	if a.noteInput.Value() == "" {
		fg = colorMuted
	}
	text := lipgloss.NewStyle().Foreground(fg).Background(bg)
	caret := lipgloss.NewStyle().Foreground(bg).Background(colorAccent)
	border := lipgloss.NewStyle().Foreground(colorBorderFocus).Background(bg)
	lines := strings.Split(a.noteInput.View(), "\n")
	var out []string
	if bordered {
		out = append(out, border.Render("╭"+strings.Repeat("─", w)+"╮"))
	}
	for row := 0; row < h; row++ {
		line := ""
		if row < len(lines) {
			line = restyleNoteInput(lines[row], text, caret)
		}
		line = padPanelLine(line, w, bg)
		if bordered {
			line = border.Render("│") + line + border.Render("│")
		}
		out = append(out, line)
	}
	if bordered {
		out = append(out, border.Render("╰"+strings.Repeat("─", w)+"╯"))
	}
	return strings.Join(out, "\n")
}
