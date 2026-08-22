package ui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"taskii/internal/model"
)

const noteBullet = "• "

// ansiRe matches SGR escape sequences, for stripping styling out of
// third-party widget output before restyling it.
var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

// wrapPlain breaks s into lines of at most width display cells, splitting on
// existing newlines first and then wrapping long runs on word boundaries
// (falling back to a hard break for a single word wider than the pane).
// Operates on PLAIN text — styling is applied afterwards, per line, so no
// escape sequence is ever split in half.
func wrapPlain(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	var out []string
	for _, para := range strings.Split(s, "\n") {
		if para == "" {
			out = append(out, "")
			continue
		}
		var line string
		for _, word := range strings.Fields(para) {
			switch {
			case line == "":
				line = word
			case runewidth.StringWidth(line)+1+runewidth.StringWidth(word) <= width:
				line += " " + word
			default:
				out = append(out, line)
				line = word
			}
			// A single word wider than the pane still has to be broken.
			for runewidth.StringWidth(line) > width {
				cut := ""
				for _, r := range line {
					if runewidth.StringWidth(cut)+runewidth.RuneWidth(r) > width {
						break
					}
					cut += string(r)
				}
				out = append(out, cut)
				line = line[len(cut):]
			}
		}
		if line != "" {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		out = append(out, "")
	}
	return out
}

// noteDisplayLine is one rendered row of the notes board, tagged with the
// note it belongs to so selection can highlight every row of a wrapped note
// and scrolling can work in display lines rather than whole notes.
type noteDisplayLine struct {
	noteIndex int
	text      string
	first     bool // true on the row carrying the bullet
}

// noteLines flattens notes into display rows at the given content width.
// Continuation rows are indented under the bullet so a wrapped note reads as
// one block rather than as several separate bullets.
func noteLines(notes []model.Note, width int) []noteDisplayLine {
	indent := strings.Repeat(" ", runewidth.StringWidth(noteBullet))
	bodyWidth := width - runewidth.StringWidth(noteBullet)
	if bodyWidth < 1 {
		bodyWidth = 1
	}

	var out []noteDisplayLine
	for i, n := range notes {
		for j, l := range wrapPlain(n.Body, bodyWidth) {
			prefix := indent
			if j == 0 {
				prefix = noteBullet
			}
			out = append(out, noteDisplayLine{
				noteIndex: i,
				text:      prefix + l,
				first:     j == 0,
			})
		}
	}
	return out
}

// renderNotes draws the board: bullets, wrapped bodies, a highlight spanning
// every row of the selected note, and the same always-emitted scroll
// indicator the task lists use so the pane's height never changes with scroll
// state.
func renderNotes(notes []model.Note, selected, scrollOffset, visibleRows int, focused, editing bool, width int) string {
	if len(notes) == 0 && !editing {
		return hintStyle.Render("(empty board)") + "\n"
	}
	if visibleRows < 1 {
		visibleRows = 1
	}

	all := noteLines(notes, width)
	end := scrollOffset + visibleRows
	if end > len(all) {
		end = len(all)
	}
	if scrollOffset > len(all) {
		scrollOffset = len(all)
	}

	var lines []string
	for i := scrollOffset; i < end; i++ {
		dl := all[i]
		isSel := focused && dl.noteIndex == selected

		bg := colorPaneBg
		style := lipgloss.NewStyle().Foreground(colorText).Background(bg)
		if isSel {
			bg = colorPanel
			style = lipgloss.NewStyle().Foreground(colorText).Background(bg).Bold(true)
		}
		bulletStyle := lipgloss.NewStyle().Foreground(colorAccent).Background(bg)

		// Built as self-contained spans that each carry their own fg+bg and
		// padded by hand — never re-wrapped in an outer Render, which would
		// drop the background past the first embedded reset.
		var line string
		if dl.first {
			line = bulletStyle.Render(noteBullet) +
				style.Render(strings.TrimPrefix(dl.text, noteBullet))
		} else {
			line = style.Render(dl.text)
		}
		if pad := width - lipgloss.Width(line); pad > 0 {
			line += lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", pad))
		}
		lines = append(lines, line)
	}

	above := scrollOffset
	below := len(all) - end
	indicator := ""
	switch {
	case above > 0 && below > 0:
		indicator = fmt.Sprintf("↑ %d more / ↓ %d more", above, below)
	case above > 0:
		indicator = fmt.Sprintf("↑ %d more", above)
	case below > 0:
		indicator = fmt.Sprintf("↓ %d more", below)
	}
	lines = append(lines, hintStyle.Render(fitToWidth(indicator, width)))

	return strings.Join(lines, "\n")
}
