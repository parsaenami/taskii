package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type helpKey struct {
	key   string
	label string
}

// name groups keys for readability in app.go's helpGroups() call sites, but
// is no longer rendered — per user request the bottom bar shows flat
// "[key] label" entries with no category prefix.
type helpGroup struct {
	name string
	keys []helpKey
}

// helpKeyStyle, helpLabelStyle, helpSepStyle, and helpStyle live in theme.go
// (rebuilt by applyTheme on every theme change, not just once at package
// load) — see the comment there for why that matters.

// renderHelpBar lays out every group's keys as flat "[key] label" entries —
// group names are dropped per user request; the bracketed key alone reads
// clearly enough without a category prefix — separated by a dim "│", and
// wraps to additional lines if the full bar doesn't fit width.
func renderHelpBar(groups []helpGroup, width int) string {
	var entries []string
	for _, g := range groups {
		for _, k := range g.keys {
			// An empty key means a plain note rather than a "press X" entry
			// (e.g. a usage hint) — render as a bare label, no brackets.
			if k.key == "" {
				entries = append(entries, helpLabelStyle.Render(k.label))
				continue
			}
			entries = append(entries, helpKeyStyle.Render("["+k.key+"]")+" "+helpLabelStyle.Render(k.label))
		}
	}

	sep := helpSepStyle.Render("  │  ")
	margin := helpStyle.Render(" ")

	full := margin + strings.Join(entries, sep)
	if width <= 0 || lipgloss.Width(full) <= width {
		return full
	}

	// Doesn't fit on one line: wrap entry-by-entry so no single "[key] label"
	// pair ever splits across lines.
	var lineText string
	var lines []string
	for _, e := range entries {
		candidate := e
		if lineText != "" {
			candidate = lineText + sep + e
		}
		if lineText != "" && lipgloss.Width(candidate) > width {
			lines = append(lines, margin+lineText)
			lineText = e
			continue
		}
		lineText = candidate
	}
	if lineText != "" {
		lines = append(lines, margin+lineText)
	}
	return strings.Join(lines, "\n")
}
