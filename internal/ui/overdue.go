package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderPane(title string, body string, focused bool, width, height int) string {
	borderColor := colorBorder
	if focused {
		borderColor = colorBorderFocus
	}
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)

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
	if title != "" {
		// Prepend via a plain slice, NOT lipgloss.JoinVertical: JoinVertical
		// pads every line up to the widest line's width using its own PLAIN,
		// unstyled spaces. Since the styled title row is normally the widest
		// line, that padding landed on every shorter body line before this
		// function's own styled padding below ever ran (pad computed to 0,
		// because the line was already contentWidth wide) — leaving those
		// cells at the terminal's default background, which reads as solid
		// black rectangles after short content like "(no tasks)".
		lines = append([]string{titleStyle.Width(contentWidth).Render(title)}, lines...)
	}

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
		if pad := contentWidth - lipgloss.Width(l); pad > 0 {
			lines[i] = l + blank.Render(strings.Repeat(" ", pad))
		}
	}
	for len(lines) < contentHeight {
		lines = append(lines, blank.Render(strings.Repeat(" ", contentWidth)))
	}
	if len(lines) > contentHeight {
		lines = lines[:contentHeight]
	}

	pad := blank.Render(" ")
	top := borderStyle.Render("╭"+strings.Repeat("─", width-2)) + borderStyle.Render("╮")
	bottom := borderStyle.Render("╰"+strings.Repeat("─", width-2)) + borderStyle.Render("╯")

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
