package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// shortcutItem is intentionally separate from helpKey. The bottom help bar
// is contextual and short; this overlay is the complete reference, including
// bindings that are only active in a particular pane or modal.
type shortcutItem struct {
	key   string
	label string
}

type shortcutSection struct {
	name  string
	items []shortcutItem
}

// shortcutSections is kept as a single inventory so the reference can be
// audited against updateNormal, updateSimple, updateSettings, and the editor
// handlers without depending on whatever happens to be focused.
var shortcutSections = []shortcutSection{
	{
		name: "Global",
		items: []shortcutItem{
			{"?", "open this shortcuts reference"},
			{"q", "quit in normal and simple mode"},
			{"ctrl+c", "quit normally; cancel Settings"},
			{"S", "open Settings"},
			{"R", "manage routines (normal/simple mode)"},
			{"tab / shift+tab", "switch pane in normal mode"},
		},
	},
	{
		name: "Tasks / Today / Upcoming",
		items: []shortcutItem{
			{"a", "add a task (Today)"},
			{"enter", "edit the selected task"},
			{"space", "mark done / undone"},
			{"space / enter", "toggle today's routine"},
			{"s", "skip selected routine today"},
			{"d", "delete after confirmation"},
			{"i", "toggle important"},
			{"I", "show important only"},
			{"U", "show undone only"},
			{"C", "switch Today and Upcoming"},
			{"up/down, j/k", "move through tasks"},
		},
	},
	{
		name: "Overdue",
		items: []shortcutItem{
			{"space", "move the selected task to Today"},
			{"d", "delete after confirmation"},
			{"i", "toggle important"},
			{"I / U", "important / undone filter"},
			{"up/down, j/k", "move through overdue tasks"},
		},
	},
	{
		name: "Reports",
		items: []shortcutItem{
			{"left/right, h/l", "switch chart"},
			{"up/down, j/k", "scroll routine rows on Routines"},
			{"tab / shift+tab", "switch pane"},
		},
	},
	{
		name: "Notes",
		items: []shortcutItem{
			{"a", "add a note"},
			{"enter", "edit the selected note"},
			{"d", "delete after confirmation"},
			{"C", "clear the board after confirmation"},
			{"e", "expand / shrink the board"},
			{"up/down, j/k", "move through notes"},
		},
	},
	{
		name: "Pomodoro",
		items: []shortcutItem{
			{"p", "start / pause"},
			{"r", "reset the current phase"},
			{"n", "skip to the next phase"},
		},
	},
	{
		name: "Simple mode",
		items: []shortcutItem{
			{"tab", "switch Task and Note input"},
			{"a", "add the selected input type"},
			{"space", "toggle the selected task"},
			{"space / enter", "toggle a routine; s skips it"},
			{"enter", "edit the selected entry"},
			{"d", "delete after confirmation"},
			{"i", "toggle important on a task"},
			{"I / U", "important / undone filter"},
			{"C", "switch Today and Upcoming"},
			{"up/down, j/k", "move through entries"},
		},
	},
	{
		name: "Editors / Modals",
		items: []shortcutItem{
			{"enter", "save task or note"},
			{"esc", "cancel or leave the editor"},
			{"ctrl+j / ctrl+n", "insert a note newline"},
			{"alt/opt+enter", "insert a note newline"},
			{"shift+enter", "newline where the terminal supports it"},
			{"#tag, MM-DD, HH:MM", "task entry annotations"},
			{"!Nd", "task deadline annotation"},
			{"y / Y / enter", "confirm a deletion or clear"},
			{"any other key", "cancel a confirmation"},
			{"up/down, j/k", "navigate Settings sections"},
			{"left/right, h/l", "adjust or navigate Settings"},
			{"ctrl+s", "save Settings"},
			{"a / enter / e", "add / edit routine in manager"},
			{"left/right / space", "schedule / custom weekdays in routine editor"},
			{"ctrl+s", "save routine editor"},
			{"ctrl+p/k, ctrl+n/j", "move in the theme browser"},
		},
	},
}

const (
	shortcutsMaxWidth = 82
	shortcutsMinWidth = 30
)

func (a App) shortcutsModalWidth() int {
	w := a.width - 4
	if w > shortcutsMaxWidth {
		w = shortcutsMaxWidth
	}
	if w < shortcutsMinWidth {
		w = a.width - 2
	}
	if w < 4 {
		w = 4
	}
	if w > a.width && a.width > 0 {
		w = a.width
	}
	return w
}

func (a App) shortcutsModalHeight() int {
	if a.height < 3 {
		return 3
	}
	return a.height
}

func (a App) shortcutRows(width int) []string {
	if width < 1 {
		width = 1
	}
	var rows []string
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Background(colorPaneBg)
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(colorText).Background(colorPaneBg)
	labelStyle := lipgloss.NewStyle().Foreground(colorMuted).Background(colorPaneBg)

	for _, section := range shortcutSections {
		rows = append(rows, padPanelLine(sectionStyle.Render("  "+section.name), width, colorPaneBg))
		for _, item := range section.items {
			prefix := "    " + item.key + "  "
			available := width - lipgloss.Width(prefix)
			label := item.label
			if available < 1 {
				available = 1
			}
			if lipgloss.Width(label) > available {
				label = fitToWidth(label, available)
			}
			line := keyStyle.Render(prefix) + labelStyle.Render(label)
			rows = append(rows, padPanelLine(line, width, colorPaneBg))
		}
	}
	return rows
}

func (a App) shortcutsViewportRows() int {
	// The body has room for a header and footer at normal sizes. At very short
	// heights those optional lines yield before the actual shortcut rows do.
	capacity := a.shortcutsModalHeight() - 2
	if capacity < 1 {
		return 1
	}
	if capacity == 1 {
		return 1
	}
	if capacity >= 3 {
		return capacity - 2
	}
	return capacity - 1
}

func (a App) shortcutsMaxScroll() int {
	width := a.shortcutsModalWidth() - 4
	if width < 1 {
		width = 1
	}
	rows := a.shortcutRows(width)
	max := len(rows) - a.shortcutsViewportRows()
	if max < 0 {
		return 0
	}
	return max
}

func (a App) updateShortcuts(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "?", "esc", "q":
		a.shortcutsOpen = false
		a.shortcutsScroll = 0
		return a, nil
	case "ctrl+c":
		return a, tea.Quit
	case "up", "k":
		a.shortcutsScroll--
	case "down", "j":
		a.shortcutsScroll++
	case "pgup":
		a.shortcutsScroll -= a.shortcutsViewportRows()
	case "pgdown":
		a.shortcutsScroll += a.shortcutsViewportRows()
	case "home":
		a.shortcutsScroll = 0
	case "end":
		a.shortcutsScroll = a.shortcutsMaxScroll()
	default:
		return a, nil
	}
	if a.shortcutsScroll < 0 {
		a.shortcutsScroll = 0
	}
	if max := a.shortcutsMaxScroll(); a.shortcutsScroll > max {
		a.shortcutsScroll = max
	}
	return a, nil
}

func (a App) renderShortcutsModal() string {
	modalWidth := a.shortcutsModalWidth()
	innerWidth := modalWidth - 4
	if innerWidth < 1 {
		innerWidth = 1
	}
	allRows := a.shortcutRows(innerWidth)
	viewport := a.shortcutsViewportRows()
	maxScroll := len(allRows) - viewport
	if maxScroll < 0 {
		maxScroll = 0
	}
	scroll := a.shortcutsScroll
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	end := scroll + viewport
	if end > len(allRows) {
		end = len(allRows)
	}

	muted := lipgloss.NewStyle().Foreground(colorMuted).Background(colorPaneBg)
	blank := lipgloss.NewStyle().Background(colorPaneBg)
	capacity := a.shortcutsModalHeight() - 2
	if capacity < 1 {
		capacity = 1
	}
	showHeader := capacity >= 3
	showFooter := capacity >= 2
	lines := make([]string, 0, capacity)
	if showHeader {
		lines = append(lines, padPanelLine(muted.Render("  All shortcuts · ↑/↓ or PgUp/PgDn to scroll"), innerWidth, colorPaneBg))
	}
	lines = append(lines, allRows[scroll:end]...)
	if showFooter {
		footer := "  [? / esc / q] close"
		if len(allRows) > viewport {
			footer = fmt.Sprintf("  %s · %d-%d of %d · [? / esc / q] close",
				map[bool]string{true: "↑/↓ scroll", false: "↓ more"}[scroll > 0],
				scroll+1, end, len(allRows))
		}
		lines = append(lines, padPanelLine(muted.Render(footer), innerWidth, colorPaneBg))
	}
	for len(lines) < capacity {
		lines = append(lines, blank.Render(strings.Repeat(" ", innerWidth)))
	}

	return renderPane("Shortcuts", strings.Join(lines, "\n"), true, modalWidth, len(lines)+2)
}
