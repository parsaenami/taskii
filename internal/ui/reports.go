package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"terminal-dashboard/internal/stats"
)

// blockLevels gives sub-character-resolution fill via partial block glyphs,
// so bars look like a smooth gradient ramp instead of a flat on/off fill.
var blockLevels = []rune{' ', '▏', '▎', '▍', '▌', '▋', '▊', '▉', '█'}

func renderGradientBar(pct float64, width int, color lipgloss.Color) string {
	if width < 1 {
		width = 1
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}

	filledCells := pct / 100 * float64(width)
	full := int(filledCells)
	frac := filledCells - float64(full)

	var b strings.Builder
	// Every glyph here is individually styled and spliced into a larger
	// line as plain text — each one's own SGR reset would otherwise
	// terminate the pane's background wherever it sits, so the background
	// is carried explicitly on each style rather than relying on the outer
	// pane wrap to fill it in.
	filledStyle := lipgloss.NewStyle().Foreground(color).Background(colorPaneBg)
	emptyStyle := lipgloss.NewStyle().Foreground(colorMuted).Background(colorPaneBg)

	for i := 0; i < width; i++ {
		switch {
		case i < full:
			b.WriteString(filledStyle.Render("█"))
		case i == full && frac > 0:
			idx := int(frac * float64(len(blockLevels)-1))
			b.WriteString(filledStyle.Render(string(blockLevels[idx])))
		default:
			b.WriteString(emptyStyle.Render("░"))
		}
	}
	return b.String()
}

// percentColor maps a completion percentage onto the theme's semantic ramp:
// red/muted for low, amber for medium, green for high.
func percentColor(pct float64) lipgloss.Color {
	switch {
	case pct >= 66:
		return colorGreen
	case pct >= 33:
		return colorWarning
	default:
		return colorDanger
	}
}

// renderProgressRow lays out label / bar+pct+counts within an exact total
// row width. The pct+counts suffix's width varies with digit count (e.g.
// "(6/13)" vs "(17/126)"), so the bar is sized from the ACTUAL suffix width
// rather than a guessed constant — a fixed guess undercounts for wider
// counts and the row wraps, silently growing the pane's rendered height.
func renderProgressRow(label string, p stats.Progress, rowWidth int) string {
	color := percentColor(p.Percent())
	pct := fmt.Sprintf("%5.1f%%", p.Percent())
	counts := fmt.Sprintf("(%d/%d)", p.Done, p.Total)
	suffix := " " + pct + " " + counts

	barWidth := rowWidth - lipgloss.Width(suffix)
	if barWidth < 5 {
		barWidth = 5
	}
	bar := renderGradientBar(p.Percent(), barWidth, color)

	return fmt.Sprintf("%s\n%s %s %s",
		statLabelStyle.Render(label),
		bar,
		lipgloss.NewStyle().Bold(true).Foreground(color).Background(colorPaneBg).Render(pct),
		statLabelStyle.Render(counts),
	)
}

const maxHeatmapWeeks = 16

// renderHeatmap draws a GitHub-style contribution grid: one column per week,
// one row per weekday (Sun top .. Sat bottom), shaded by completion count.
// Chosen over keeping the flat 7-day bar chart because it shows the same
// "how active was I" story with much more history in similar screen space,
// and reads immediately from the block-density pattern alone.
func renderHeatmap(cells []stats.HeatmapCell) string {
	if len(cells) == 0 {
		return statLabelStyle.Render("(no data)")
	}
	weeks := len(cells) / 7
	if weeks == 0 {
		return statLabelStyle.Render("(no data)")
	}

	grid := make([][]stats.HeatmapCell, 7) // row = weekday
	for r := 0; r < 7; r++ {
		grid[r] = make([]stats.HeatmapCell, weeks)
	}
	for i, c := range cells {
		week := i / 7
		day := i % 7
		grid[day][week] = c
	}

	weekdayLabels := []string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"}
	var rows []string
	for r := 0; r < 7; r++ {
		var b strings.Builder
		b.WriteString(statLabelStyle.Render(weekdayLabels[r]))
		b.WriteString(" ")
		for _, cell := range grid[r] {
			if cell.Level < 0 {
				b.WriteString(lipgloss.NewStyle().Background(colorPaneBg).Render("  "))
				continue
			}
			color := heatmapRamp[cell.Level]
			b.WriteString(lipgloss.NewStyle().Foreground(color).Background(colorPaneBg).Render("██"))
		}
		// Legend rides on the Saturday row instead of its own trailing lines
		// (label+blank+legend cost 3 lines) so the whole grid fits in
		// realistic terminal heights (~24-30 rows) instead of needing 50+.
		if r == 6 {
			b.WriteString(lipgloss.NewStyle().Background(colorPaneBg).Render("   "))
			b.WriteString(statLabelStyle.Render("less"))
			for _, c := range heatmapRamp {
				b.WriteString(lipgloss.NewStyle().Background(colorPaneBg).Render(" "))
				b.WriteString(lipgloss.NewStyle().Foreground(c).Background(colorPaneBg).Render("██"))
			}
			b.WriteString(statLabelStyle.Render(" more"))
		}
		rows = append(rows, b.String())
	}

	return strings.Join(rows, "\n")
}

// renderReports lays out progress bars, the contribution heatmap, and streak
// within an exact content height so it never overflows renderPane's fixed
// box: lipgloss's .Height() is a floor, not a cap, so content taller than
// the box grows it and pushes the header off the top of the terminal.
func renderReports(r stats.Report, width, height int) string {
	sections := []string{
		titleStyle.Width(width).Render("Reports"),
		"",
		renderProgressRow("Today", r.Today, width),
		"",
		renderProgressRow("This Week", r.Week, width),
		"",
		renderProgressRow("This Month", r.Month, width),
	}

	// Heatmap needs 1 (blank) + 1 (label) + 7 (rows, legend inline on the
	// last row) = 9 lines, and at least 3 week-columns' worth of width
	// (label + 2 cols/week) to be legible. sections elements can each be
	// multi-line (progress rows render as 2 lines), so count actual
	// rendered lines rather than len(sections) or the streak line silently
	// pushes the pane past its height budget.
	usedLines := 0
	for _, s := range sections {
		usedLines += strings.Count(s, "\n") + 1
	}
	remaining := height - usedLines
	const heatmapCost = 9
	if remaining >= heatmapCost+2 && width >= 3+3*2 {
		heatmap := r.Heatmap
		// Fewer weeks of history fit shorter panes just as well width-wise;
		// trim from the front (oldest) so "today" always stays visible on
		// the right, same as GitHub's own graph reads left-to-right in time.
		availableWeeks := (width - 3) / 2
		haveWeeks := len(heatmap) / 7
		if availableWeeks > 0 && availableWeeks < haveWeeks {
			heatmap = heatmap[(haveWeeks-availableWeeks)*7:]
		}
		weeks := len(heatmap) / 7
		sections = append(sections,
			"",
			statLabelStyle.Render(fmt.Sprintf("Activity (last %d weeks)", weeks)),
			renderHeatmap(heatmap),
		)
	}

	sections = append(sections,
		"",
		fmt.Sprintf("%s %s", statLabelStyle.Render("Current Streak:"), statValueStyle.Render(streakText(r.Streak))),
	)

	return strings.Join(sections, "\n")
}

func streakText(n int) string {
	if n == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", n)
}
