package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/parsaenami/taskii/internal/stats"
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
	// The partial cell is a LEFT-aligned glyph: only its leading fraction is
	// painted in the fill colour, and the rest of the cell falls through to
	// its background. With the pane background there, that remainder read as
	// a gap punched between the filled bar and the track it should meet — the
	// same severed-segment effect the stacked bar chart had.
	//
	// The background has to be the track's EFFECTIVE colour, not colorMuted.
	// A track cell is "░" — muted ink covering only about a quarter of the
	// cell over the pane background — so it reads as a dark blend, while solid
	// colorMuted is far lighter (in Ember, #8a7268 against a ~#3c302b track).
	// Using colorMuted directly put a conspicuously pale slab in the seam.
	partialStyle := lipgloss.NewStyle().Foreground(color).Background(trackColor())

	for i := 0; i < width; i++ {
		switch {
		case i < full:
			b.WriteString(filledStyle.Render("█"))
		case i == full && frac > 0:
			idx := int(frac * float64(len(blockLevels)-1))
			b.WriteString(partialStyle.Render(string(blockLevels[idx])))
		default:
			b.WriteString(emptyStyle.Render("░"))
		}
	}
	return b.String()
}

// shadeCoverage is roughly how much of a cell "░" (U+2591 LIGHT SHADE) inks
// in. The Unicode shade blocks step 25/50/75%, and fonts render them close
// enough to that for a colour match.
const shadeCoverage = 0.25

// trackColor is the colour a "░" track cell actually appears to be: muted ink
// at shadeCoverage over the pane background. Computed from the live theme
// colours so it stays correct across every theme rather than being pinned to
// one palette.
func trackColor() lipgloss.Color {
	return blendColor(colorPaneBg, colorMuted, shadeCoverage)
}

// blendColor mixes two hex colours, returning base with mix blended in at the
// given ratio (0 = base, 1 = mix). Falls back to base if either colour isn't
// parseable hex, so a malformed theme degrades to the old look rather than
// rendering something arbitrary.
func blendColor(base, mix lipgloss.Color, ratio float64) lipgloss.Color {
	br, bg, bb, ok1 := parseHexColor(string(base))
	mr, mg, mb, ok2 := parseHexColor(string(mix))
	if !ok1 || !ok2 {
		return base
	}
	lerp := func(a, b int) int {
		return int(float64(a) + (float64(b)-float64(a))*ratio + 0.5)
	}
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x",
		lerp(br, mr), lerp(bg, mg), lerp(bb, mb)))
}

// parseHexColor reads "#rrggbb". Themes are authored in that form throughout,
// so the short "#rgb" spelling isn't handled.
func parseHexColor(s string) (r, g, b int, ok bool) {
	if len(s) != 7 || s[0] != '#' {
		return 0, 0, 0, false
	}
	v, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return int(v>>16) & 0xff, int(v>>8) & 0xff, int(v) & 0xff, true
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
// The count rides on the LABEL line ("Today  6/17") rather than after the
// bar, so the number you actually want — how many of today's tasks are done —
// reads first. The percentage sits after the bar, where its width is known
// and can be carved out of the bar's own width.
func renderProgressRow(label string, p stats.Progress, rowWidth int) string {
	color := percentColor(p.Percent())
	count := fmt.Sprintf("  %d/%d", p.Done, p.Total)

	head := statLabelStyle.Render(label) +
		lipgloss.NewStyle().Bold(true).Foreground(colorText).Background(colorPaneBg).Render(count)

	pct := fmt.Sprintf("%5.1f%%", p.Percent())
	suffix := " " + pct

	barWidth := rowWidth - lipgloss.Width(suffix)
	if barWidth < 5 {
		barWidth = 5
	}
	// The separating space is folded into the percentage's own Render: a bare
	// " " between two styled spans is outside both and falls through to the
	// terminal's default background.
	bar := renderGradientBar(p.Percent(), barWidth, color) +
		lipgloss.NewStyle().Bold(true).Foreground(color).Background(colorPaneBg).Render(suffix)

	return head + "\n" + bar
}

const maxHeatmapWeeks = 16

// renderHeatmap draws a GitHub-style contribution grid: one column per week,
// one row per weekday (Sun top .. Sat bottom), shaded by completion count.
// Chosen over keeping the flat 7-day bar chart because it shows the same
// "how active was I" story with much more history in similar screen space,
// and reads immediately from the block-density pattern alone.
// heatmapLegendWidth is the colour key's own width:
// "less" + 5x(space + 2-cell swatch) + " more".
const heatmapLegendWidth = 4 + 5*3 + 5

// legendPlacement says whether the heatmap's less..more colour key is drawn.
// It always sits on its own row under the grid — an inline variant riding the
// Saturday row used to exist, but it cost ~27 cells of grid width and made the
// key's position depend on the column, so the placement is now unconditional.
type legendPlacement int

const (
	// legendNone omits it (the column is too narrow for the key itself).
	legendNone legendPlacement = iota
	// legendBelow puts it on its own row under the grid.
	legendBelow
)

// heatmapWidth is the rendered width of a grid of n weeks: a 2-cell weekday
// label, a space, then 2 cells per week. The legend sits below the grid, so it
// never widens a row — but it can be the widest line in the block on its own.
func heatmapWidth(weeks int, legend legendPlacement) int {
	w := 3 + 2*weeks
	if legend == legendBelow && heatmapLegendWidth > w {
		w = heatmapLegendWidth
	}
	return w
}

// heatmapHeight is the block's line count: 7 weekday rows, plus the spacer
// and legend rows when the key is drawn.
func heatmapHeight(legend legendPlacement) int {
	h := heatmapGridLines
	if legend == legendBelow {
		h += heatmapLegendCost
	}
	return h
}

// renderHeatmapLegend draws the "less ██ ██ ██ ██ ██ more" colour key. Every
// segment carries the pane background explicitly — the separating spaces
// included — since a raw space between styled spans belongs to neither.
func renderHeatmapLegend() string {
	var b strings.Builder
	b.WriteString(statLabelStyle.Render("less"))
	for _, c := range heatmapRamp {
		b.WriteString(lipgloss.NewStyle().Background(colorPaneBg).Render(" "))
		b.WriteString(lipgloss.NewStyle().Foreground(c).Background(colorPaneBg).Render("██"))
	}
	b.WriteString(statLabelStyle.Render(" more"))
	return b.String()
}

func renderHeatmap(cells []stats.HeatmapCell, legend legendPlacement) string {
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
		// Trailing space folded into the label's own styled span; a raw
		// b.WriteString(" ") here would sit outside every SGR span and
		// render with the terminal's default background.
		b.WriteString(statLabelStyle.Render(weekdayLabels[r] + " "))
		for _, cell := range grid[r] {
			if cell.Level < 0 {
				b.WriteString(lipgloss.NewStyle().Background(colorPaneBg).Render("  "))
				continue
			}
			color := heatmapRamp[cell.Level]
			b.WriteString(lipgloss.NewStyle().Foreground(color).Background(colorPaneBg).Render("██"))
		}
		rows = append(rows, b.String())
	}

	if legend == legendBelow {
		// A blank spacer separates the key from the grid so the two don't read
		// as one block. It carries the pane background explicitly — an empty
		// string here would render as a default-background gap through the
		// pane, the same way an unstyled space between spans does.
		rows = append(rows,
			lipgloss.NewStyle().Background(colorPaneBg).Render(strings.Repeat(" ", 3+2*weeks)))

		// Centre the key under the grid. centerBlock later shifts the whole
		// block as one unit, so without this the key would sit flush against
		// the grid's left edge rather than under its middle.
		key := renderHeatmapLegend()
		gridWidth := 3 + 2*weeks
		if pad := (gridWidth - lipgloss.Width(key)) / 2; pad > 0 {
			key = lipgloss.NewStyle().Background(colorPaneBg).Render(strings.Repeat(" ", pad)) + key
		}
		rows = append(rows, key)
	}

	return strings.Join(rows, "\n")
}

// renderReports lays out progress bars, the contribution heatmap, and streak
// within an exact content height so it never overflows renderPane's fixed
// box: lipgloss's .Height() is a floor, not a cap, so content taller than
// the box grows it and pushes the header off the top of the terminal.
// reportsHeaderLines is everything above the chart: the 2-line progress row
// (label + bar), a blank, the chart selector, and the blank before the chart.
const reportsHeaderLines = 2 + 1 + 1 + 1

// reportsFullContentLines is the content height at which renderReports can
// show everything, sized to the tallest chart (the heatmap). Layouts that
// want a complete Reports pane size it from this rather than guessing.
const reportsFullContentLines = reportsHeaderLines + heatmapCost

// heatmapGridLines is the grid alone: one row per weekday, Sun..Sat.
const heatmapGridLines = 7

// heatmapCost is the whole heatmap block: the grid, a blank spacer, and the
// legend row beneath it. Sizing the pane from the grid alone would leave the
// legend short and renderPane would truncate it away silently.
const heatmapCost = heatmapGridLines + heatmapLegendCost

// heatmapLegendCost is what the colour key costs in rows: a blank spacer so
// the key doesn't sit flush against the grid, plus the key itself.
const heatmapLegendCost = 2

// reportsMinContentLines is the header alone — what Reports keeps when the
// column is too tight for any chart at all.
const reportsMinContentLines = reportsHeaderLines

// renderReports lays out today's progress, a chart selector and the selected
// chart within an exact content height so it never overflows renderPane's
// fixed box: lipgloss's .Height() is a floor, not a cap.
//
// chart selects which chart is shown; focused draws the selector highlighted
// so it's clear the arrow keys will move it.
func renderReports(r stats.Report, width, height int, chart reportChart, focused bool) string {
	// No title row here — renderPane draws "Reports" on the top border.
	sections := []string{
		renderProgressRow("Today", r.Today, width),
		"",
		renderChartTabs(chart, focused, width),
	}

	usedLines := 0
	for _, s := range sections {
		usedLines += strings.Count(s, "\n") + 1
	}

	if body := renderSelectedChart(r, chart, width, height-usedLines-1); body != "" {
		sections = append(sections, "", centerBlock(body, width))
	}

	return strings.Join(sections, "\n")
}

// centerBlock centres a multi-line block horizontally within width, padding
// each line by the SAME amount so the block's internal alignment (bars over
// their axis labels) is preserved. Padding is applied by hand with a
// background-carrying style — these lines already carry their own ANSI, and
// re-rendering pre-styled content leaves the padding unbackgrounded.
func centerBlock(s string, width int) string {
	lines := strings.Split(s, "\n")
	blockWidth := 0
	for _, l := range lines {
		if w := lipgloss.Width(l); w > blockWidth {
			blockWidth = w
		}
	}
	gap := width - blockWidth
	if gap <= 0 {
		return s
	}
	left := lipgloss.NewStyle().Background(colorPaneBg).Render(strings.Repeat(" ", gap/2))
	for i, l := range lines {
		line := left + l
		if pad := width - lipgloss.Width(line); pad > 0 {
			line += lipgloss.NewStyle().Background(colorPaneBg).Render(strings.Repeat(" ", pad))
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

// renderSelectedChart draws whichever chart is active, or "" when there isn't
// the height/width for it.
func renderSelectedChart(r stats.Report, chart reportChart, width, height int) string {
	if height < 4 || width < 12 {
		return ""
	}

	switch chart {
	case chartWeek:
		// Every day of the week gets a label; there are only seven.
		return renderBarChart(r.WeekBars, width, height, 1)

	case chartMonth:
		// Thin the labels so ~30 days don't overprint each other.
		every := 5
		if width >= 60 {
			every = 2
		} else if width >= 40 {
			every = 3
		}
		return renderBarChart(r.MonthBars, width, height, every)

	default:
		if width < 3+3*2 {
			return ""
		}
		// The legend always sits below the grid, so it costs a line and no
		// width. It's dropped only when the key itself won't fit the column,
		// or when that extra line won't fit the height.
		legend := legendNone
		if heatmapLegendWidth <= width && height >= heatmapHeight(legendBelow) {
			legend = legendBelow
		}
		if height < heatmapHeight(legend) {
			return ""
		}

		heatmap := r.Heatmap
		haveWeeks := len(heatmap) / 7
		availableWeeks := (width - 3) / 2
		// Trim from the front (oldest) so "today" stays visible on the right,
		// same as GitHub's own graph reads left-to-right in time.
		if availableWeeks > 0 && availableWeeks < haveWeeks {
			heatmap = heatmap[(haveWeeks-availableWeeks)*7:]
		}
		return renderHeatmap(heatmap, legend)
	}
}

// renderChartTabs draws the Week / Month / Contribution selector. When the
// pane is focused the active tab is filled with the accent colour, so it's
// obvious the arrow keys act here; unfocused it's a quieter underline.
func renderChartTabs(chart reportChart, focused bool, width int) string {
	// Full labels need ~29 cells; below that they'd be truncated mid-word and
	// the selector stops being readable exactly where switching still matters,
	// so fall back to initials.
	short := width < 29

	var b strings.Builder
	for i := chartWeek; i <= chartContribution; i++ {
		if i > chartWeek {
			b.WriteString(statLabelStyle.Render(" "))
		}
		name := i.String()
		if short {
			name = name[:1]
		}
		switch {
		case i == chart && focused:
			b.WriteString(lipgloss.NewStyle().Bold(true).
				Foreground(colorPaneBg).Background(colorAccent).Render(" " + name + " "))
		case i == chart:
			b.WriteString(lipgloss.NewStyle().Bold(true).Underline(true).
				Foreground(colorAccent).Background(colorPaneBg).Render(" " + name + " "))
		default:
			b.WriteString(statLabelStyle.Render(" " + name + " "))
		}
	}
	row := b.String()
	if pad := width - lipgloss.Width(row); pad > 0 {
		row += lipgloss.NewStyle().Background(colorPaneBg).Render(strings.Repeat(" ", pad))
	} else if pad < 0 {
		row = truncateANSI(row, width)
	}
	return row
}
