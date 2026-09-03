package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"taskii/internal/model"
)

// simpleEntry is one row of the combined list: a task (today's or overdue) or
// a note. Purely a view-level projection — the stored data structures are
// untouched, the two collections are just merged for display.
type simpleEntry struct {
	created time.Time
	isNote  bool
	task    model.Task
	overdue bool
	note    model.Note
	// noteIndex/taskID map a row back to its source so selection can act on
	// the right item without re-deriving it from the row's position.
	noteIndex int
}

// simpleEntries merges tasks and notes into one list ordered by creation time,
// oldest first. Today's tasks, overdue tasks and notes all appear together.
func (a App) simpleEntries() []simpleEntry {
	if a.upcoming {
		var out []simpleEntry
		for _, t := range a.upcomingTasks() {
			out = append(out, simpleEntry{created: t.CreatedAt, task: t})
		}
		return out
	}
	today := a.now().Format(dateFormat)
	var out []simpleEntry

	for _, t := range a.applyFilters(a.tasks) {
		// Today's items plus anything still undone from an earlier day —
		// exactly the union of what the Today and Overdue panes show.
		if t.Date != today && (t.Done || t.Date > today) {
			continue
		}
		out = append(out, simpleEntry{
			created: t.CreatedAt,
			task:    t,
			overdue: t.Date < today,
		})
	}
	for i, n := range a.notes {
		out = append(out, simpleEntry{
			created:   n.CreatedAt,
			isNote:    true,
			note:      n,
			noteIndex: i,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].created.Before(out[j].created)
	})
	return out
}

// simpleDisplayLine is one rendered row, tagged with the entry it belongs to
// so a wrapped note highlights as a block and scrolling counts display lines.
type simpleDisplayLine struct {
	entryIndex int
	text       string
	first      bool
}

// simpleGutter is the fixed lead-in every row shares, so notes and tasks line
// up: a 1-cell selection indicator, a space, a 3-cell marker field ("[ ]",
// "{x}" or " • "), and a space. Note bodies and task titles therefore start at
// the same column.
const simpleGutter = 1 + 1 + 3 + 1

// simpleNoteMarker is the note's stand-in for a task's checkbox, sized to the
// same 3 cells so the dot sits centred under the brackets.
const simpleNoteMarker = " • "

// simpleLines flattens entries into display rows at the given width. Tasks are
// one row each (truncated); notes wrap, since they have no length limit.
func (a App) simpleLines(entries []simpleEntry, width int) []simpleDisplayLine {
	var out []simpleDisplayLine
	for i, e := range entries {
		if !e.isNote {
			out = append(out, simpleDisplayLine{entryIndex: i, first: true})
			continue
		}
		// Wrapped rows hang under the body, not under the marker.
		bodyWidth := width - simpleGutter
		if bodyWidth < 1 {
			bodyWidth = 1
		}
		for j, l := range wrapPlain(e.note.Body, bodyWidth) {
			out = append(out, simpleDisplayLine{
				entryIndex: i,
				text:       l,
				first:      j == 0,
			})
		}
	}
	return out
}

// renderSimpleList draws the merged list with the same selection/scroll
// behaviour as the notes board: selection is per ENTRY, scrolling is per
// DISPLAY LINE, and the scroll indicator is always emitted so the list's
// height never changes with scroll state.
func (a App) renderSimpleList(entries []simpleEntry, visibleRows, width int) string {
	if len(entries) == 0 && a.mode != modeAdding && a.mode != modeNoteEditing {
		// colorBg, not hintStyle's colorPaneBg — see the scroll indicator below.
		return lipgloss.NewStyle().Foreground(colorMuted).Background(colorBg).
			Render("(nothing yet)") + "\n"
	}
	if visibleRows < 1 {
		visibleRows = 1
	}

	all := a.simpleLines(entries, width)
	scroll := a.simpleScroll
	if scroll > len(all) {
		scroll = len(all)
	}
	end := scroll + visibleRows
	if end > len(all) {
		end = len(all)
	}

	var lines []string
	for i := scroll; i < end; i++ {
		dl := all[i]
		e := entries[dl.entryIndex]
		selected := dl.entryIndex == a.simpleSelected

		if e.isNote {
			bg := colorBg
			if selected {
				bg = colorPanel
			}
			style := lipgloss.NewStyle().Foreground(colorText).Background(bg)
			if selected {
				style = style.Bold(true)
			}

			// Same gutter shape as renderTaskLine — indicator, space, 3-cell
			// marker, space — so the dot lines up under the brackets and the
			// body under the titles. Continuation rows keep the gutter blank.
			// Indicator only on the note's FIRST row, like the marker — a
			// wrapped note is one item, so repeating ">" down its
			// continuation rows read as several selected entries.
			prefix := " "
			if selected && dl.first {
				prefix = ">"
			}
			marker := strings.Repeat(" ", lipgloss.Width(simpleNoteMarker))
			markerStyled := style.Render(marker)
			if dl.first {
				marker = simpleNoteMarker
				markerStyled = lipgloss.NewStyle().Foreground(colorAccent).Background(bg).Render(marker)
			}

			text := dl.text
			if avail := width - simpleGutter; avail > 0 && lipgloss.Width(text) > avail {
				text = fitToWidth(text, avail)
			}

			line := style.Render(prefix+" ") + markerStyled + style.Render(" "+text)
			if pad := width - lipgloss.Width(line); pad > 0 {
				line += lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", pad))
			}
			lines = append(lines, line)
			continue
		}
		lines = append(lines, renderTaskLine(e.task, selected, e.overdue, width, colorBg, a.upcoming))
	}

	above := scroll
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
	// Styled with colorBg, NOT hintStyle: hintStyle carries colorPaneBg, the
	// shade used inside a bordered pane. Simple mode has no panes, so that
	// shade rendered as a stray highlighted strip across the row under the
	// last item — most visible when the indicator is empty and the whole row
	// is just padding.
	lines = append(lines,
		lipgloss.NewStyle().Foreground(colorMuted).Background(colorBg).
			Render(fitToWidth(indicator, width)))

	return strings.Join(lines, "\n")
}

// simplePadding is the blank margin on each side of the simple-mode page.
// There's no pane border here to hold the content off the terminal edge, so
// the padding does that job.
const simplePadding = 2

// simpleTopPadding is a blank row above the content, so the banner isn't
// jammed against the top edge. Taken out of the height budget by
// simpleBodyHeight rather than added on top, or the page would overflow.
const simpleTopPadding = 1

// simpleContentWidth is the usable width once both side margins are taken
// out. Every other width in simple mode is derived from this rather than from
// a.width, so the margins can't be spent twice and overflow the terminal.
func (a App) simpleContentWidth() int {
	w := a.width - 2*simplePadding
	if w < 1 {
		w = 1
	}
	return w
}

// simpleGreetingWidth is the fixed left column: wide enough for the banner
// plus a little breathing room, and never more than a third of the content.
func (a App) simpleGreetingWidth() int {
	w := lipgloss.Width(taskiiBanner[0]) + 2
	if max := a.simpleContentWidth() / 3; w > max {
		w = max
	}
	if w < 8 {
		w = 8
	}
	return w
}

// simpleListWidth is what's left for the list after the greeting column and
// the divider (" │ ", 3 cells).
func (a App) simpleListWidth() int {
	w := a.simpleContentWidth() - a.simpleGreetingWidth() - 3
	if w < 1 {
		w = 1
	}
	return w
}

// simpleBodyHeight is the content area's height: the terminal less the fixed
// chrome and the top margin. Single source for both the row budget and the
// column padding, so they can't disagree about how tall the body is.
func (a App) simpleBodyHeight() int {
	h := a.height - a.chromeLines() - simpleTopPadding
	if h < 1 {
		h = 1
	}
	return h
}

// simpleVisibleRows is how many list rows fit, reserving the always-emitted
// scroll indicator and, while adding, the input line.
func (a App) simpleVisibleRows() int {
	// -1 for the scroll indicator, -simpleTabsHeight for the TASK/NOTE tabs.
	rows := a.simpleBodyHeight() - 1 - simpleTabsHeight
	if a.mode == modeAdding {
		rows--
	}
	if a.mode == modeNoteEditing {
		rows -= a.noteEditorHeight()
	}
	if rows < 1 {
		rows = 1
	}
	return rows
}

// simpleTabsHeight is what renderSimpleTabs occupies: the tab row plus the
// rule under it.
const simpleTabsHeight = 2

// renderSimpleTabs draws the TASK / NOTE selector above the list. The active
// tab is filled with the accent colour and marked with a caret, so which mode
// `a` will add in is visible at a glance rather than only inferable from the
// help bar at the bottom of the screen.
func (a App) renderSimpleTabs(width int) string {
	activeStyle := lipgloss.NewStyle().Bold(true).
		Foreground(colorBg).Background(colorAccent)
	idleStyle := lipgloss.NewStyle().Foreground(colorMuted).Background(colorBg)
	gap := lipgloss.NewStyle().Background(colorBg)

	tab := func(label string, active bool) string {
		if active {
			return activeStyle.Render(" ▸ " + label + " ")
		}
		return idleStyle.Render("   " + label + " ")
	}

	row := tab("TASK", !a.simpleNoteMode) + gap.Render(" ") + tab("NOTE", a.simpleNoteMode)
	if a.upcoming {
		row = tab("Upcoming", true) + idleStyle.Render(filterLabel(a.filterImportant, a.filterUndone))
	}
	if pad := width - lipgloss.Width(row); pad > 0 {
		row += gap.Render(strings.Repeat(" ", pad))
	} else if pad < 0 {
		row = truncateANSI(row, width)
	}

	rule := lipgloss.NewStyle().Foreground(colorBorder).Background(colorBg).
		Render(strings.Repeat("─", max(width, 0)))
	return row + "\n" + rule
}

// renderSimple builds the whole simple-mode body: greeting column, vertical
// divider, list.
func (a App) renderSimple() string {
	gw := a.simpleGreetingWidth()
	lw := a.simpleListWidth()
	bodyHeight := a.simpleBodyHeight()

	entries := a.simpleEntries()
	list := a.renderSimpleTabs(lw) + "\n" +
		a.renderSimpleList(entries, a.simpleVisibleRows(), lw)

	if a.mode == modeAdding {
		a.input.TextStyle = lipgloss.NewStyle().Foreground(colorText).Background(colorBg)
		a.input.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorMuted).Background(colorBg)
		a.input.PromptStyle = lipgloss.NewStyle().Foreground(colorAccent).Background(colorBg)
		a.input.Cursor.Style = lipgloss.NewStyle().Foreground(colorText).Background(colorBg)
		// inputPromptStyle bakes in colorPaneBg; re-base it on the page
		// surface, since simple mode has no panes.
		promptStyle := inputPromptStyle.Copy().Background(colorBg)
		if field := lw - lipgloss.Width(promptStyle.Render("+ ")) - lipgloss.Width(a.input.Prompt) - 1; field > 0 &&
			lipgloss.Width(a.input.Placeholder) > field {
			a.input.Placeholder = fitToWidth(a.input.Placeholder, field)
		}
		a.input.Width = 0
		line := promptStyle.Render("+ ") + a.input.View()
		if pad := lw - lipgloss.Width(line); pad > 0 {
			line += lipgloss.NewStyle().Background(colorBg).Render(strings.Repeat(" ", pad))
		}
		list += "\n" + line
	}
	if a.mode == modeNoteEditing {
		list += "\n" + a.renderNoteEditor(lw)
	}

	greet := renderSimpleGreeting(a.now(), a.username, gw)

	// Pad both columns to the body height so the divider runs the full way
	// down and JoinHorizontal has nothing of its own to pad with.
	greetLines := strings.Split(greet, "\n")
	listLines := strings.Split(list, "\n")
	blankGreet := lipgloss.NewStyle().Background(colorBg).Render(strings.Repeat(" ", gw))
	blankList := lipgloss.NewStyle().Background(colorBg).Render(strings.Repeat(" ", lw))
	for len(greetLines) < bodyHeight {
		greetLines = append(greetLines, blankGreet)
	}
	for len(listLines) < bodyHeight {
		listLines = append(listLines, blankList)
	}
	greetLines = greetLines[:bodyHeight]
	listLines = listLines[:bodyHeight]

	bg := lipgloss.NewStyle().Background(colorBg)
	divStyle := lipgloss.NewStyle().Foreground(colorBorder).Background(colorBg)
	pad := bg.Render(" ")
	// Side margins carry the page background explicitly, like every other
	// blank run in this codebase — a raw space would print as a stripe of the
	// terminal's default background down each edge.
	margin := bg.Render(strings.Repeat(" ", simplePadding))

	var b strings.Builder
	// Top margin: full-width blank rows above everything, so the divider
	// starts level with the content rather than running up to the edge.
	// padLines in assemblePage fills these out to the terminal width.
	for i := 0; i < simpleTopPadding; i++ {
		b.WriteString(bg.Render(" "))
		b.WriteString("\n")
	}
	for i := 0; i < bodyHeight; i++ {
		if i > 0 {
			b.WriteString("\n")
		}
		// Both columns are padded AND clamped here: anything wider than its
		// budget would shift the divider and consume the side margin, so the
		// column widths stay authoritative over their contents.
		g := greetLines[i]
		switch p := gw - lipgloss.Width(g); {
		case p > 0:
			g += bg.Render(strings.Repeat(" ", p))
		case p < 0:
			g = truncateANSI(g, gw)
		}
		l := listLines[i]
		switch p := lw - lipgloss.Width(l); {
		case p > 0:
			l += bg.Render(strings.Repeat(" ", p))
		case p < 0:
			l = truncateANSI(l, lw)
		}
		b.WriteString(margin)
		b.WriteString(g)
		b.WriteString(pad)
		b.WriteString(divStyle.Render("│"))
		b.WriteString(pad)
		b.WriteString(l)
		b.WriteString(margin)
	}
	return b.String()
}

// renderSimpleGreeting is the greeting block LEFT-aligned, for the fixed
// column at the left of simple mode. renderGreeting centres its content for
// the boxed pane in the other layouts, which would look wrong against a
// divider line.
func renderSimpleGreeting(now time.Time, username string, width int) string {
	greetLine := timeGreeting(now)
	if username != "" {
		greetLine += ", " + username
	}

	logoStyle := lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Background(colorBg)
	blank := lipgloss.NewStyle().Background(colorBg)
	label := lipgloss.NewStyle().Foreground(colorMuted).Background(colorBg)

	var lines []string
	if lipgloss.Width(taskiiBanner[0]) <= width {
		for _, row := range taskiiBanner {
			lines = append(lines, logoStyle.Render(row))
		}
	} else {
		lines = append(lines, logoStyle.Render("TASKII"))
	}
	lines = append(lines,
		blank.Render(""),
		label.Render(now.Format("Monday, January 2, 2006")),
		label.Render("v"+Version),
		blank.Render(""),
		label.Render(greetLine),
	)

	// Clamp as well as pad: the date and greeting lines are longer than the
	// banner, so on a narrow terminal they overflowed this column, pushed the
	// divider right and ate the page's right margin.
	for i, l := range lines {
		switch pad := width - lipgloss.Width(l); {
		case pad > 0:
			lines[i] = l + blank.Render(strings.Repeat(" ", pad))
		case pad < 0:
			lines[i] = truncateANSI(l, width)
		}
	}
	return strings.Join(lines, "\n")
}
