package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

func renderPane(title string, body string, focused bool, width, height int) string {
	borderColor := colorBorder
	if focused {
		borderColor = colorBorderFocus
	}
	// The border carries colorPaneBg, not just a foreground: it's the pane's
	// own outermost edge, so filling it with the pane background keeps each
	// box reading as one solid surface against the (deliberately different)
	// page background. Without a Background here the glyph cells fall through
	// to the terminal default and the box is outlined in black.
	borderStyle := lipgloss.NewStyle().Foreground(borderColor).Background(colorPaneBg)

	// Width parameter is the intended outer rendered width; the border (1
	// col each side) sits outside it, and paneStyle's own Padding(0,1) is
	// carved out of it — so the interior content area is width-4.
	contentWidth := width - 4
	if contentWidth < 1 {
		contentWidth = 1
	}
	// Height includes border top + bottom (2 lines).
	contentHeight := height - 2
	if contentHeight < 1 {
		contentHeight = 1
	}

	lines := strings.Split(body, "\n")

	// Pad content to exactly contentWidth/contentHeight OURSELVES, with the
	// pane's own background, and then wrap it in a HAND-BUILT border rather
	// than handing pre-styled (already-ANSI) content to another
	// lipgloss Style.Render() call for the width/height/border pass.
	// lipgloss's own internal width/alignment processing — reliable on
	// plain text — turned out to mis-handle already-styled multi-segment
	// ANSI content when re-rendered a second time: a correctly-produced
	// single 59-cell-wide styled row got spuriously split across two
	// output lines merely by being passed through style.Render() again
	// after padding. Confirmed by isolating: identical padding logic
	// against a plain (unstyled) string round-tripped through
	// style.Render() correctly, but the real ANSI-heavy row did not.
	// Building the border by hand sidesteps that lipgloss codepath
	// entirely for content that's already fully styled.
	blank := lipgloss.NewStyle().Background(colorPaneBg)
	for i, l := range lines {
		switch pad := contentWidth - lipgloss.Width(l); {
		case pad > 0:
			lines[i] = l + blank.Render(strings.Repeat(" ", pad))
		case pad < 0:
			// Clamp overlong lines. Height is already capped below, but width
			// wasn't: a body line wider than contentWidth pushed this pane
			// past the width its caller budgeted, which then dragged the
			// whole JoinHorizontal layout past the terminal edge. Truncating
			// keeps the box's geometry authoritative over its contents.
			lines[i] = truncateANSI(l, contentWidth)
		}
	}
	for len(lines) < contentHeight {
		lines = append(lines, blank.Render(strings.Repeat(" ", contentWidth)))
	}
	if len(lines) > contentHeight {
		lines = lines[:contentHeight]
	}

	pad := blank.Render(" ")
	bottom := borderStyle.Render("╰"+strings.Repeat("─", width-2)) + borderStyle.Render("╯")

	// Title rides ON the top border, inset from the left corner:
	//   ╭─ Today (6) ─────────────╮
	// The dashes either side are part of the border run, so the title is
	// built as border/title/border spans that each carry their own colors —
	// concatenated, never re-wrapped, per this file's usual rule.
	top := borderStyle.Render("╭")
	if title == "" {
		top += borderStyle.Render(strings.Repeat("─", width-2))
	} else {
		// The title's own decoration inside the width-2 border run is
		// "─ " before and " " after — 3 cells beyond the title itself.
		// Below that there's no room for a title on the border at all and
		// we fall back to a plain rule.
		const decor = 3
		avail := width - 2 - decor
		if avail < 1 {
			top += borderStyle.Render(strings.Repeat("─", width-2))
		} else {
			t := title
			if lipgloss.Width(t) > avail {
				t = fitToWidth(t, avail)
			}
			trail := width - 2 - decor - lipgloss.Width(t)
			if trail < 0 {
				trail = 0
			}
			top += borderStyle.Render("─ ") +
				paneTitleStyle.Render(t) +
				borderStyle.Render(" "+strings.Repeat("─", trail))
		}
	}
	top += borderStyle.Render("╮")

	var b strings.Builder
	b.WriteString(top)
	for _, l := range lines {
		b.WriteString("\n")
		b.WriteString(borderStyle.Render("│"))
		b.WriteString(pad)
		b.WriteString(l)
		b.WriteString(pad)
		b.WriteString(borderStyle.Render("│"))
	}
	b.WriteString("\n")
	b.WriteString(bottom)
	return b.String()
}

// truncateANSI cuts s to at most w display cells, preserving the ANSI escape
// sequences it passes over (so styling and, crucially, the background stay
// applied) and re-terminating with a reset. lipgloss's own truncation isn't
// used here for the same reason renderPane hand-builds its border: it is
// unreliable when re-processing already-styled multi-segment content.
func truncateANSI(s string, w int) string {
	if w <= 0 {
		return ""
	}
	var b strings.Builder
	width := 0
	styled := false
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			// Copy the whole escape sequence through without counting width.
			j := i + 1
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				j++
			}
			b.WriteString(s[i:j])
			styled = true
			i = j
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		rw := runewidth.RuneWidth(r)
		if width+rw > w {
			break
		}
		b.WriteString(s[i : i+size])
		width += rw
		i += size
	}
	if styled {
		b.WriteString("\x1b[0m")
	}
	return b.String()
}
