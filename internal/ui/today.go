package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"taskii/internal/model"
)

// renderTaskList renders a scrollable viewport of tasks. visibleRows is the
// number of task rows that fit (the caller already reserves a separate line
// for the scroll indicator, so pane height stays constant whether or not the
// indicator is actually shown). scrollOffset is the index of the first
// visible task.
func renderTaskList(tasks []model.Task, selected int, scrollOffset int, visibleRows int, focused bool, overdue bool, width int, now time.Time) string {
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
		lines = append(lines, renderTaskLine(tasks[i], i == selected && focused, overdue, width, colorPaneBg, now))
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

func renderTaskLine(t model.Task, selected bool, overdue bool, width int, surface lipgloss.Color, now time.Time) string {
	// In the Overdue pane the checkbox column is replaced by an age badge:
	// there's no toggling-done from that list any more (space migrates the
	// task to Today instead), so a checkbox would offer an action the pane
	// no longer has. The age is what the row is actually there to tell you.
	check := ""
	if overdue {
		// Right-aligned in a fixed 3-cell column ("  1d" would misalign
		// against "24d"), so titles start on the same column down the list
		// however stale the tasks are.
		check = fmt.Sprintf("%3s", fmt.Sprintf("%dd", t.AgeDays(now)))
	} else {
		openBracket, closeBracket := "[", "]"
		if t.IsAppointment() {
			openBracket, closeBracket = "{", "}"
		}
		check = openBracket + " " + closeBracket
		if t.Done {
			check = openBracket + "x" + closeBracket
		}
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

	// surface is the background this row sits on: the pane shade inside a
	// bordered pane, or the page shade in simple mode, which has no panes.
	// Every segment below is re-based onto it, since the shared styles
	// (taskStyle, doneStyle, …) all bake in colorPaneBg.
	bg := surface
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
		if t.EndTime != "" {
			timePlain += "-" + t.EndTime
		}
	}
	starPlain := ""
	if t.Important {
		starPlain = "★"
	}
	// A migrated task carries ▲ in Today's list as a standing reminder that
	// it was carried forward. It stacks WITH the star rather than replacing
	// it: importance and staleness are independent facts about a task, and
	// showing only one would hide the other.
	migratedPlain := ""
	if t.IsMigrated() && !overdue {
		migratedPlain = "▲"
	}
	title := t.Title

	// The deadline chip: "!Nd" while the deadline is still ahead, "‼Nd" once
	// it has been missed. The glyph — not the colour — is what carries the
	// distinction, so the two states stay apart in a monochrome terminal and
	// for anyone who can't separate the two hues. The count stays positive
	// in both directions, which keeps "!1d" (due tomorrow) from having to be
	// told apart from "-1d" (a day late) by a single character.
	duePlain := ""
	dueOverdue := false
	if days, ok := t.DaysUntilDue(now); dueDatesEnabled && ok {
		if days < 0 {
			duePlain, dueOverdue = fmt.Sprintf("‼%dd", -days), true
		} else {
			duePlain = fmt.Sprintf("!%dd", days)
		}
	}

	if width > 0 {
		prefix, check, timePlain, starPlain, migratedPlain, duePlain, title = fitSegmentsToWidth(prefix, check, timePlain, starPlain, migratedPlain, duePlain, title, width)
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
	if migratedPlain != "" {
		b.WriteString(migratedStyle.Copy().Background(bg).Render(migratedPlain))
		b.WriteString(style.Render(" "))
	}
	if duePlain != "" {
		ds := dueStyle
		if dueOverdue {
			ds = dueOverdueStyle
		}
		b.WriteString(ds.Copy().Background(bg).Render(duePlain))
		b.WriteString(style.Render(" "))
	}
	// The title keeps its "#tag" words exactly where they were typed; only
	// their colour changes. Moving them to the end would rewrite the user's
	// sentence — "review #api docs" reads differently from "review docs #api".
	b.WriteString(renderTitleWithTags(title, style, bg))

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
func fitSegmentsToWidth(prefix, check, timePlain, starPlain, migratedPlain, duePlain, title string, w int) (string, string, string, string, string, string, string) {
	assemble := func(title string) string {
		s := prefix + " " + check + " "
		if timePlain != "" {
			s += timePlain + " "
		}
		if starPlain != "" {
			s += starPlain + " "
		}
		if migratedPlain != "" {
			s += migratedPlain + " "
		}
		if duePlain != "" {
			s += duePlain + " "
		}
		return s + title
	}
	if lipgloss.Width(assemble(title)) <= w {
		return prefix, check, timePlain, starPlain, migratedPlain, duePlain, title
	}
	fixedWidth := lipgloss.Width(assemble(""))
	budget := w - fixedWidth
	if budget < 1 {
		// Not even room for the fixed prefix — nothing sensible to show for
		// the title; leave it empty rather than corrupting prefix/check.
		return prefix, check, timePlain, starPlain, migratedPlain, duePlain, ""
	}
	runes := []rune(title)
	for len(runes) > 0 {
		candidate := string(runes) + "…"
		if lipgloss.Width(candidate) <= budget {
			return prefix, check, timePlain, starPlain, migratedPlain, duePlain, candidate
		}
		runes = runes[:len(runes)-1]
	}
	return prefix, check, timePlain, starPlain, migratedPlain, duePlain, ""
}

// renderTitleWithTags styles the "#tag" words inside a title without moving
// them, splitting on spaces and colouring each tag word while the rest keeps
// the row's own style.
//
// Each word is rendered as its own complete span (own fg + the shared bg)
// and concatenated — never wrapped in a second Render() — for the same
// reason the segments above are: lipgloss emits a style's codes once at a
// string's edges, so re-rendering pre-styled text leaves a background hole
// after every inner span's reset.
func renderTitleWithTags(title string, style lipgloss.Style, bg lipgloss.Color) string {
	if !strings.Contains(title, "#") {
		return style.Render(title)
	}
	tag := tagStyle.Copy().Background(bg)
	if style.GetBold() {
		tag = tag.Bold(true)
	}
	var b strings.Builder
	for i, word := range strings.Split(title, " ") {
		if i > 0 {
			b.WriteString(style.Render(" "))
		}
		if len(word) > 1 && strings.HasPrefix(word, "#") {
			b.WriteString(tag.Render(word))
			continue
		}
		b.WriteString(style.Render(word))
	}
	return b.String()
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
