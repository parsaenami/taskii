package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"taskii/internal/stats"
)

// reportChart selects which chart the Reports pane shows.
type reportChart int

const (
	chartWeek reportChart = iota
	chartMonth
	chartContribution
)

var reportChartNames = map[reportChart]string{
	chartWeek:         "Week",
	chartMonth:        "Month",
	chartContribution: "Contribution",
}

func (c reportChart) String() string { return reportChartNames[c] }

func (c reportChart) next() reportChart {
	if c >= chartContribution {
		return chartWeek
	}
	return c + 1
}

func (c reportChart) prev() reportChart {
	if c <= chartWeek {
		return chartContribution
	}
	return c - 1
}

// Half-block glyphs let one terminal row represent two units of bar height,
// doubling the chart's effective resolution without costing extra lines.
//
// blockLower fills only the bottom half of its cell. Where a half-block's
// empty half sits *inside* the bar (a done/open boundary landing mid-row, or a
// bar whose top edge falls mid-row), the cell is drawn with the neighbouring
// segment's colour as its BACKGROUND instead of the pane's, so both halves are
// painted and the column reads as one solid bar. Leaving the pane background
// showing through is what made bars look severed.
const (
	blockFull  = "█"
	blockLower = "▄"
	blockUpper = "▀"
)

// renderBarChart draws a stacked daily bar chart: each column is one day, its
// full height is that day's task count, and the bottom portion (in the "done"
// colour) is how many were completed.
//
// labelEvery thins the x-axis labels — every day for a week, every few days
// for a month, so the axis doesn't overprint itself.
func renderBarChart(bars []stats.DayBar, width, height int, labelEvery int) string {
	if len(bars) == 0 || height < 3 {
		return statLabelStyle.Render("(no data)")
	}

	// Reserve rows below the plot: the axis rule, the x-axis labels, and the
	// legend when there's room for it. Counting only the first two overran
	// the caller's budget by a line, and renderPane truncated it — silently
	// eating the legend rather than shrinking the plot to make room.
	const axisRows = 2
	withLegend := height >= axisRows+3
	reserved := axisRows
	if withLegend {
		reserved++
	}
	plotHeight := height - reserved
	if plotHeight < 1 {
		plotHeight = 1
	}

	maxTotal := 0
	for _, b := range bars {
		if b.Total > maxTotal {
			maxTotal = b.Total
		}
	}
	if maxTotal == 0 {
		maxTotal = 1
	}

	yLabelWidth := len(fmt.Sprintf("%d", maxTotal)) + 1
	plotWidth := width - yLabelWidth
	if plotWidth < 1 {
		plotWidth = 1
	}

	// Column width: at least 1 cell per bar, plus a gap when there's room.
	barW, gap := 1, 0
	if plotWidth >= len(bars)*3 {
		barW, gap = 2, 1
	} else if plotWidth >= len(bars)*2 {
		barW, gap = 1, 1
	}
	// Drop the oldest days if even the minimum doesn't fit.
	perCol := barW + gap
	if maxCols := plotWidth / perCol; maxCols >= 1 && maxCols < len(bars) {
		bars = bars[len(bars)-maxCols:]
	}

	// Each bar stacks three bands, bottom to top: tasks carried forward from
	// an earlier day and finished today, tasks scheduled for today and
	// finished, then whatever is still open. Migrated work sits at the bottom
	// because it's the day's oldest debt — the foundation the rest is built
	// on — and a band anchored to the axis is the one the eye measures most
	// reliably.
	migratedColor := colorPurple
	doneColor := colorGreen
	todoColor := colorMuted

	blank := lipgloss.NewStyle().Background(colorPaneBg)
	axisStyle := lipgloss.NewStyle().Foreground(colorBorder).Background(colorPaneBg)
	migratedStyleBar := lipgloss.NewStyle().Foreground(migratedColor).Background(colorPaneBg)
	doneStyleBar := lipgloss.NewStyle().Foreground(doneColor).Background(colorPaneBg)
	todoStyleBar := lipgloss.NewStyle().Foreground(todoColor).Background(colorPaneBg)

	// Each row is half a unit taller than the one below, so a row can be
	// full, half, or empty for a given bar.
	unitsPerRow := float64(maxTotal) / float64(plotHeight)
	tickStep := yTickStep(maxTotal, plotHeight)

	var rows []string
	for row := 0; row < plotHeight; row++ {
		// Value at the top and bottom of this row, counting from the axis up.
		rowTop := float64(plotHeight-row) * unitsPerRow
		rowBottom := float64(plotHeight-row-1) * unitsPerRow

		// y-axis label: the max on the top row, then every tick value that
		// falls inside this row's span. Labelling only the top and the axis
		// left the middle of the plot unscaled — a bar's height could only be
		// eyeballed against two reference points.
		label := strings.Repeat(" ", yLabelWidth)
		if row == 0 {
			label = fmt.Sprintf("%*d ", yLabelWidth-1, maxTotal)
		} else if v, ok := tickInRow(rowBottom, rowTop, tickStep, maxTotal); ok {
			label = fmt.Sprintf("%*d ", yLabelWidth-1, v)
		}
		line := statLabelStyle.Render(label)

		for i, b := range bars {
			if i > 0 && gap > 0 {
				line += blank.Render(strings.Repeat(" ", gap))
			}
			glyph, style := barCell(b, rowTop, rowBottom,
				migratedColor, doneColor, todoColor, blank)
			line += style.Render(strings.Repeat(glyph, barW))
		}
		rows = append(rows, line)
	}

	// Axis rule.
	axis := statLabelStyle.Render(fmt.Sprintf("%*d ", yLabelWidth-1, 0))
	axisWidth := len(bars)*barW + max(len(bars)-1, 0)*gap
	axis += axisStyle.Render(strings.Repeat("─", axisWidth))
	rows = append(rows, axis)

	// x-axis labels, thinned so they don't collide. Labels are written into a
	// plain character buffer positioned under their column and only styled at
	// the end — a multi-digit day number is wider than its 1-2 cell column, so
	// per-column padding would truncate "17" to "." Overflow is allowed to
	// run into the following (unlabelled) columns instead.
	buf := []rune(strings.Repeat(" ", axisWidth))
	for i, b := range bars {
		if labelEvery <= 0 || i%labelEvery != 0 {
			continue
		}
		text := fmt.Sprintf("%d", b.Date.Day())
		if labelEvery == 1 {
			// Week view: weekday initial fits a narrow column.
			text = b.Date.Format("Mon")[:1]
		}
		at := i * perCol
		// Skip a label that would run past the axis rather than printing a
		// clipped fragment ("22" rendering as "2" reads as day 2).
		if at+len([]rune(text)) > len(buf) {
			continue
		}
		for j, r := range []rune(text) {
			buf[at+j] = r
		}
	}
	rows = append(rows,
		blank.Render(strings.Repeat(" ", yLabelWidth))+statLabelStyle.Render(string(buf)))

	// Legend, so the two colours are self-explanatory. Its row was reserved
	// above, so appending it here can't push the chart past its budget.
	if withLegend {
		pad := blank.Render(strings.Repeat(" ", yLabelWidth))
		full := pad +
			migratedStyleBar.Render("█") + statLabelStyle.Render(" carried  ") +
			doneStyleBar.Render("█") + statLabelStyle.Render(" done  ") +
			todoStyleBar.Render("█") + statLabelStyle.Render(" open")
		// Fall back to the original two-swatch legend on a pane too narrow
		// for three, rather than dropping the legend altogether: "done" and
		// "open" are the two most people read most often.
		short := pad +
			doneStyleBar.Render("█") + statLabelStyle.Render(" done  ") +
			todoStyleBar.Render("█") + statLabelStyle.Render(" open")
		switch {
		case lipgloss.Width(full) <= width:
			rows = append(rows, full)
		case lipgloss.Width(short) <= width:
			rows = append(rows, short)
		}
	}

	return strings.Join(rows, "\n")
}

// yTickStep picks the increment between y-axis labels: the smallest "nice"
// value (1, 2, 5, 10, 20, 50, …) that keeps labels at least two rows apart, so
// the axis stays readable instead of printing a number on every row of a tall
// plot. Returns 0 when the plot is too short for any intermediate label.
func yTickStep(maxTotal, plotHeight int) int {
	if plotHeight < 3 || maxTotal < 2 {
		return 0
	}
	// Aim for a label roughly every other row, and never more than 5 ticks.
	wanted := plotHeight / 2
	if wanted > 5 {
		wanted = 5
	}
	if wanted < 1 {
		return 0
	}
	rough := float64(maxTotal) / float64(wanted)
	// Walk the nice values in ascending order and take the first that is at
	// least as coarse as the rough target.
	step := 0
	for scale := 1; scale <= 1000000 && step == 0; scale *= 10 {
		for _, mult := range []int{1, 2, 5} {
			if cand := mult * scale; float64(cand) >= rough {
				step = cand
				break
			}
		}
	}
	if step < 1 {
		step = 1
	}
	if step >= maxTotal {
		return 0
	}
	return step
}

// tickInRow reports the tick value to print against a row spanning
// (bottom, top]. The largest multiple of step in that half-open interval wins,
// and the max is skipped — the top row already carries it, and repeating it a
// row lower would misread as a second, lower gridline at the same value.
func tickInRow(bottom, top float64, step, maxTotal int) (int, bool) {
	if step <= 0 {
		return 0, false
	}
	// Largest multiple of step that is <= top.
	v := (int(top) / step) * step
	for v > 0 {
		if float64(v) > bottom && float64(v) <= top && v != maxTotal {
			return v, true
		}
		v -= step
	}
	return 0, false
}

// barCell picks the glyph and style for one cell of one bar, given the value
// range [rowBottom, rowTop) that cell spans.
//
// The bar is a stack of coloured bands. Walking them bottom-up, the cell is
// painted by the first band whose top edge reaches into this row; a band
// ending mid-row is drawn as a half-block composited over whichever colour
// sits above it, so the boundary fills the whole cell instead of letting the
// pane background show through as a gap.
//
// Written as a loop over a band list rather than a switch over every
// pair of adjacent bands: with three bands the case analysis would be nine
// branches, and each new band would square it again.
func barCell(b stats.DayBar, rowTop, rowBottom float64,
	migratedColor, doneColor, todoColor lipgloss.Color,
	blank lipgloss.Style) (string, lipgloss.Style) {

	// Cumulative top edge of each band, bottom to top. DoneMigrated is a
	// SUBSET of Done, so the plain-done band is the remainder.
	migrated := float64(b.DoneMigrated)
	done := float64(b.Done)
	total := float64(b.Total)

	bands := []struct {
		top   float64
		color lipgloss.Color
	}{
		{migrated, migratedColor},
		{done, doneColor},
		{total, todoColor},
	}

	for i, band := range bands {
		if band.top <= rowBottom {
			continue // this band ends below the row entirely
		}
		// The colour directly above this band, if the bar continues past it.
		var above lipgloss.Color
		hasAbove := false
		for _, next := range bands[i+1:] {
			if next.top > band.top {
				above, hasAbove = next.color, true
				break
			}
		}

		switch {
		case band.top >= rowTop:
			// Band fills this row outright.
			return blockFull, lipgloss.NewStyle().Foreground(band.color).Background(colorPaneBg)
		case hasAbove:
			// Band ends inside the row and something continues above it:
			// composite so both halves are painted.
			return blockLower, lipgloss.NewStyle().Foreground(band.color).Background(above)
		default:
			// Top edge of the whole bar; the pane background is correct above.
			return blockLower, lipgloss.NewStyle().Foreground(band.color).Background(colorPaneBg)
		}
	}
	return " ", blank
}
