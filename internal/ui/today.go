package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"terminal-dashboard/internal/model"
)

// renderTaskList renders a scrollable viewport of tasks. visibleRows is the
// number of task rows that fit (the caller already reserves a separate line
// for the scroll indicator, so pane height stays constant whether or not the
// indicator is actually shown). scrollOffset is the index of the first
// visible task.
func renderTaskList(tasks []model.Task, selected int, scrollOffset int, visibleRows int, focused bool, overdue bool, width int) string {
	if len(tasks) == 0 {
		// Blank second line matches the indicator line always emitted below,
		// so an empty list is the same height as a populated one.
		return hintStyle.Render("(no tasks)") + "\n"
	}

	if visibleRows < 1 {
		visibleRows = 1
	}

	end := scrollOffset + visibleRows
	if end > len(tasks) {
		end = len(tasks)
	}

	var lines []string
	for i := scrollOffset; i < end; i++ {
		lines = append(lines, renderTaskLine(tasks[i], i == selected && focused, overdue, width))
	}

	// Always emit the indicator line, blank when unneeded, so the list's
	// total line count never changes based on scroll state — otherwise the
	// pane would grow/shrink by a line whenever "N more" appears/disappears.
	above := scrollOffset
	below := len(tasks) - end
	indicator := ""
	switch {
	case above > 0 && below > 0:
		indicator = fmt.Sprintf("↑ %d more / ↓ %d more", above, below)
	case above > 0:
		indicator = fmt.Sprintf("↑ %d more", above)
	case below > 0:
		indicator = fmt.Sprintf("↓ %d more", below)
	}
	lines = append(lines, hintStyle.Render(indicator))

	return strings.Join(lines, "\n")
}

func renderTaskLine(t model.Task, selected bool, overdue bool, width int) string {
	openBracket, closeBracket := "[", "]"
	if t.IsAppointment() {
		openBracket, closeBracket = "{", "}"
	}
	check := openBracket + " " + closeBracket
	if t.Done {
		check = openBracket + "x" + closeBracket
	}

	prefix := ">"
	if !selected {
		prefix = " "
	}

	// Precedence: done/overdue state always wins (it's about whether the item
	// still needs attention), important is next, then appointment-vs-task is
	// purely cosmetic and loses to any of the above.
	var style lipgloss.Style
	switch {
	case t.Done && overdue:
		style = overdueDoneStyle
	case t.Done:
		style = doneStyle
	case overdue:
		style = overdueStyle
	case t.Important:
		style = importantStyle
	case t.IsAppointment():
		style = appointmentStyle
	default:
		style = taskStyle
	}

	bg := colorPaneBg
	if selected {
		bg = colorPanel
	}
	style = style.Copy().Background(bg)
	if selected {
		style = style.Bold(true)
	}

	// Build each segment as PLAIN text first so truncation/padding math is
	// measured once, up front, on plain runes — then style every segment
	// (including padding) as a complete, self-contained span (own fg + the
	// shared bg) and concatenate. Never re-wrap an already-rendered string
	// in a second Render() call: lipgloss only emits a style's SGR codes
	// once at a string's start/end, so wrapping pre-styled text in another
	// Render() does NOT re-apply that outer style after an inner span's own
	// reset — any span styled first and spliced into a string that then
	// gets Render()'d again leaves a background "hole" right after it.
	timePlain := ""
	if t.Time != "" {
		timePlain = t.Time
	}
	starPlain := ""
	if t.Important {
		starPlain = "★"
	}
	title := t.Title

	if width > 0 {
		prefix, check, timePlain, starPlain, title = fitSegmentsToWidth(prefix, check, timePlain, starPlain, title, width)
	}

	var b strings.Builder
	b.WriteString(style.Render(prefix + " " + check + " "))
	if timePlain != "" {
		b.WriteString(timeStyle.Copy().Background(bg).Render(timePlain))
		b.WriteString(style.Render(" "))
	}
	if starPlain != "" {
		b.WriteString(importantStyle.Copy().Background(bg).Render(starPlain))
		b.WriteString(style.Render(" "))
	}
	b.WriteString(style.Render(title))

	if width > 0 {
		rendered := b.String()
		if pad := width - lipgloss.Width(rendered); pad > 0 {
			b.WriteString(style.Render(strings.Repeat(" ", pad)))
		}
	}

	return b.String()
}

// fitSegmentsToWidth measures the plain (unstyled) assembled line and, if it
// exceeds w, shortens the title with an ellipsis first (matching fitToWidth's
// truncate-from-the-end behavior) since the title is the one segment safe to
// shrink without losing meaning — prefix/check/time/star stay intact.
func fitSegmentsToWidth(prefix, check, timePlain, starPlain, title string, w int) (string, string, string, string, string) {
	assemble := func(title string) string {
		s := prefix + " " + check + " "
		if timePlain != "" {
			s += timePlain + " "
		}
		if starPlain != "" {
			s += starPlain + " "
		}
		return s + title
	}
	if lipgloss.Width(assemble(title)) <= w {
		return prefix, check, timePlain, starPlain, title
	}
	fixedWidth := lipgloss.Width(assemble(""))
	budget := w - fixedWidth
	if budget < 1 {
		// Not even room for the fixed prefix — nothing sensible to show for
		// the title; leave it empty rather than corrupting prefix/check.
		return prefix, check, timePlain, starPlain, ""
	}
	runes := []rune(title)
	for len(runes) > 0 {
		candidate := string(runes) + "…"
		if lipgloss.Width(candidate) <= budget {
			return prefix, check, timePlain, starPlain, candidate
		}
		runes = runes[:len(runes)-1]
	}
	return prefix, check, timePlain, starPlain, ""
}

// fitToWidth pads or truncates s (by rune display width) to exactly w cells.
// Used only for plain (unstyled) helper text — colored task rows compute
// their own width via fitSegmentsToWidth so styling never gets re-wrapped.
func fitToWidth(s string, w int) string {
	cur := lipgloss.Width(s)
	if cur == w {
		return s
	}
	if cur < w {
		return s + strings.Repeat(" ", w-cur)
	}
	if w <= 1 {
		return strings.Repeat(".", w)
	}
	runes := []rune(s)
	// Trim rune-by-rune until the ellipsis-suffixed string fits, since
	// display width and rune count diverge for wide/multi-byte glyphs (★).
	for len(runes) > 0 {
		candidate := string(runes) + "…"
		if lipgloss.Width(candidate) <= w {
			return candidate + strings.Repeat(" ", w-lipgloss.Width(candidate))
		}
		runes = runes[:len(runes)-1]
	}
	return strings.Repeat(".", w)
}
