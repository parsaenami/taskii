package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// layout selects how the two groups of panes — the task column (Today +
// Overdue) and the info column (Greeting + Reports + Pomodoro) — are arranged
// on screen.
type layout int

const (
	// layoutTasksLeft is two columns with tasks on the left, info on the right.
	layoutTasksLeft layout = iota
	// layoutTasksRight is the mirror image: info left, tasks right.
	layoutTasksRight
	// layoutStacked is two rows: a short info row on top (Greeting, Reports
	// and Pomodoro side by side), with the task panes filling the row below.
	layoutStacked
	// layoutThreeColumn is info | tasks | notes at 1/4 : 2/4 : 1/4, giving the
	// task lists the middle half and Notes a full-height column of its own.
	layoutThreeColumn
	// layoutDashboardGrid places a horizontal header and horizontal Pomodoro
	// above Reports | Today and Notes | Overdue.
	layoutDashboardGrid
	// layoutHeaderColumns places a full-width header above info | tasks | notes.
	layoutHeaderColumns
)

var layoutNames = map[layout]string{
	layoutTasksLeft:     "Layout 1",
	layoutTasksRight:    "Layout 2",
	layoutStacked:       "Layout 3",
	layoutThreeColumn:   "Layout 4",
	layoutDashboardGrid: "Layout 5",
	layoutHeaderColumns: "Layout 6",
}

// Preserve preferences written before the layouts received numbered names.
var legacyLayoutNames = map[string]layout{
	"Tasks Left": layoutTasksLeft, "Tasks Right": layoutTasksRight,
	"Stacked": layoutStacked, "Three Column": layoutThreeColumn,
	"Dashboard Grid": layoutDashboardGrid, "Header Columns": layoutHeaderColumns,
}

func (l layout) String() string {
	if n, ok := layoutNames[l]; ok {
		return n
	}
	return layoutNames[layoutTasksLeft]
}

func layoutByName(name string) layout {
	for l, n := range layoutNames {
		if n == name {
			return l
		}
	}
	if l, ok := legacyLayoutNames[name]; ok {
		return l
	}
	return layoutTasksLeft
}

func (l layout) next() layout {
	if l >= layoutHeaderColumns {
		return layoutTasksLeft
	}
	return l + 1
}

// geometry is every size decision for one frame, computed in ONE place so
// View() and the scroll math in visibleRowsFor() can never disagree about how
// big a pane is — a past source of off-by-one scrolling bugs. All fields are
// outer pane dimensions (borders included) unless named otherwise.
type geometry struct {
	// taskWidth is the outer width of the Today/Overdue panes; infoWidth is
	// the outer width of the Greeting/Reports/Pomodoro panes. In the stacked
	// layout the three info panes sit side by side, so infoWidth is only a
	// third of the screen while taskWidth is the full width.
	taskWidth int
	infoWidth int

	todayHeight   int
	overdueHeight int

	greetHeight   int
	reportsHeight int
	pomoHeight    int

	// Borderless headers have page-background dimensions, not pane content
	// dimensions. They are used only by the two header-based layouts.
	headerWidth  int
	headerHeight int
	headerGap    int // page-background rows below the header
	rowGap       int // page-background rows between upper/lower panes

	// notesWidth/notesHeight are the Notes pane's own box. In the column
	// layouts it sits under Pomodoro in the info column; in the stacked
	// layout it shares the task row with Today and Overdue.
	notesWidth  int
	notesHeight int
}

// notesMinContentLines is the smallest useful Notes board: two rows of bullets
// plus the scroll-indicator line the renderer always emits. Below this the
// pane is dropped entirely rather than rendered unusably small.
const notesMinContentLines = 3

// Fixed content heights for the two panes whose content doesn't grow with the
// pane: Pomodoro is phase/timer/bar/blank/sessions, and the greeting is its
// banner plus blank/date/greeting. Both are carved out of the shared budget
// before Reports claims whatever is left, so Reports gets the height its
// progress bars and heatmap actually need.
// header, blank, 3 rows of block digits, blank, bar, blank, session summary,
// and the pinned key-hint row.
const pomoContentLines = 10

// pomoMinContentLines is what must survive when the column is tight: the
// phase header, the 3 rows of block digits, the session summary, and the
// pinned key-hint row.
// renderPomodoro sheds its blank spacers first and then the progress bar
// (the big countdown already conveys progress) to reach this.
//
// Stated as an explicit count rather than pomoContentLines minus the droppable
// rows: derived that way, adding any new line silently raised the floor too,
// which is how a since-removed status line pushed Reports' heatmap out of
// reach on 40-row terminals.
const pomoMinContentLines = 6

func (a App) geometry() geometry {
	bodyHeight := a.height - a.chromeLines()
	if bodyHeight < 10 {
		bodyHeight = 10
	}

	var g geometry

	// Expanded Notes takes the whole body, whatever the layout. Every other
	// pane keeps a zero height and is skipped by View().
	if a.notesExpanded && a.focus == focusNotes {
		g.notesWidth = a.width
		g.notesHeight = bodyHeight
		return g
	}

	if a.layout == layoutDashboardGrid {
		g.infoWidth = (a.width - 1) * 2 / 5
		g.taskWidth = a.width - g.infoWidth - 1
		g.notesWidth = g.infoWidth
		g.headerWidth = g.infoWidth
		// Preserve Today's eight-row input budget and Notes' seven-row editor
		// budget independent of mode. On short terminals header context and
		// page gaps yield first; five outer header rows retain all three digit rows.
		g.headerHeight = min(7, max(5, bodyHeight-17))
		gapBudget := max(0, bodyHeight-g.headerHeight-15)
		g.headerGap = min(1, gapBudget)
		g.rowGap = min(1, max(0, gapBudget-g.headerGap))
		g.pomoHeight = g.headerHeight
		remaining := bodyHeight - g.headerHeight - g.headerGap - g.rowGap
		// Tabs, routine/task headings, one selected row, the scroll hint and
		// inline input need eight outer rows. Reserve them in normal mode too
		// so opening the input never changes the pane positions.
		g.reportsHeight = min(remaining-3, max(8, remaining/2))
		g.todayHeight = g.reportsHeight
		g.notesHeight = remaining - g.reportsHeight
		g.overdueHeight = g.notesHeight
		return g
	}

	if a.layout == layoutHeaderColumns {
		g.headerWidth = a.width
		// Yield the header's vertical margins before crowding the task input
		// and the full Pomodoro's eight-row minimum. The three banner rows stay.
		g.headerHeight = min(5, max(3, bodyHeight-18))
		g.headerGap, g.rowGap = 1, 1
		g.infoWidth = a.width / 4
		g.notesWidth = a.width / 4
		g.taskWidth = a.width - g.infoWidth - g.notesWidth - 2
		remaining := bodyHeight - g.headerHeight - g.headerGap
		g.reportsHeight = min(remaining-g.rowGap-3, max(8, (remaining-g.rowGap)/2))
		g.todayHeight = g.reportsHeight
		g.pomoHeight = remaining - g.reportsHeight - g.rowGap
		g.overdueHeight = g.pomoHeight
		g.notesHeight = remaining
		return g
	}

	if a.layout == layoutStacked {
		// One full-width row of info panes on top, tasks filling the rest.
		// The info row is sized so Reports can show everything it has
		// (progress rows + heatmap) rather than to the greeting's
		// smaller fixed content — otherwise the heatmap and streak silently
		// drop out, which is what this row is mostly there to show.
		infoHeight := reportsFullContentLines + 2
		// Still bounded, but the task panes sit SIDE BY SIDE in this layout
		// and so need far fewer rows than when stacked — a bodyHeight/2 cap
		// clipped the info row 3 lines short of the heatmap on a 40-row
		// terminal for no benefit. Reserve a fixed minimum for the task row
		// instead, which lets the info row reach full size whenever the
		// terminal can afford it.
		const minTaskRows = 8
		if max := bodyHeight - minTaskRows; infoHeight > max {
			infoHeight = max
		}
		if infoHeight < 4 {
			infoHeight = 4
		}
		g.greetHeight = infoHeight
		g.reportsHeight = infoHeight
		g.pomoHeight = infoHeight

		// Three panes share the width; the last absorbs the rounding remainder
		// so the row always sums to exactly a.width.
		g.infoWidth = a.width / 3

		// Today, Overdue and Notes sit side by side in this layout, each a
		// third of the width (Notes absorbs the rounding remainder) and each
		// getting the FULL task-row height rather than a share of it.
		g.taskWidth = a.width / 3

		taskRows := bodyHeight - infoHeight
		if taskRows < 4 {
			taskRows = 4
		}
		g.todayHeight = taskRows
		g.overdueHeight = taskRows
		g.notesWidth = a.width - 2*g.taskWidth
		g.notesHeight = taskRows
		return g
	}

	if a.layout == layoutThreeColumn {
		// info | tasks | notes at 1/4 : 2/4 : 1/4, with two single-column
		// gutters between them. Tasks take the rounding remainder so the
		// three columns plus gutters always sum to exactly a.width.
		g.infoWidth = a.width / 4
		g.notesWidth = a.width / 4
		g.taskWidth = a.width - g.infoWidth - g.notesWidth - 2
		if g.taskWidth < 20 {
			g.taskWidth = 20
		}

		g.todayHeight = bodyHeight / 2
		g.overdueHeight = bodyHeight - g.todayHeight

		// Notes owns its column outright, full height.
		g.notesHeight = bodyHeight

		// Greeting, Reports and Pomodoro split the info column into equal
		// thirds. Reports takes the rounding remainder (it's the one whose
		// content actually scales with height — the other two are fixed-size
		// and just centre themselves in whatever they get), so the three
		// always sum to exactly bodyHeight.
		third := bodyHeight / 3
		g.greetHeight = third
		g.pomoHeight = third
		g.reportsHeight = bodyHeight - 2*third
		return g
	}

	// Two-column layouts. The 3:5 split favours the task column, which holds
	// the variable-length content; the -1 leaves a one-column gutter.
	g.taskWidth = a.width * 3 / 5
	g.infoWidth = a.width - g.taskWidth - 1
	if g.infoWidth < 20 {
		g.infoWidth = 20
	}

	g.todayHeight = bodyHeight / 2
	g.overdueHeight = bodyHeight - g.todayHeight

	g.greetHeight = greetingContentLines + 2
	if max := bodyHeight / 3; g.greetHeight > max {
		g.greetHeight = max
	}

	// Pomodoro is sized AFTER the greeting and yields to Reports when the
	// column is tight. Reports takes whatever the other two leave, so a
	// fixed-height Pomodoro spends the column's slack before Reports can see
	// it — which is what pushed the heatmap out of reach on 40-row terminals
	// when the timer grew from 5 content lines to 9. renderPomodoro drops its
	// blank spacers first (see pomoMinContentLines), so shrinking it here
	// costs whitespace rather than information.
	g.pomoHeight = pomoContentLines + 2
	if slack := bodyHeight - g.greetHeight - (reportsFullContentLines + 2); slack < g.pomoHeight {
		if slack < pomoMinContentLines+2 {
			slack = pomoMinContentLines + 2
		}
		g.pomoHeight = slack
	}
	if max := bodyHeight / 2; g.pomoHeight > max {
		g.pomoHeight = max
	}

	// Reports is FIXED at the height it needs to show everything (progress
	// rows + heatmap); Notes is the flexible pane and absorbs all
	// remaining space. Reports' content is a known, bounded set, so extra
	// height beyond reportsFullContentLines just becomes blank filler —
	// whereas Notes is unbounded and every extra row shows another bullet.
	//
	// On short columns Reports still has to give: it shrinks toward
	// reportsMinContentLines (renderReports drops its chart body) so Notes
	// keeps a usable minimum.
	g.notesWidth = g.infoWidth
	remaining := bodyHeight - g.greetHeight - g.pomoHeight

	// Reports snaps between two sizes rather than taking anything in between:
	// its content is a fixed set of blocks, and an intermediate height buys
	// nothing — 14 content lines renders exactly the same as 8 (the heatmap
	// needs 11 more on top of the base rows, all or nothing) and the surplus
	// becomes blank filler while Notes sits at its minimum.
	reportsFull := reportsFullContentLines + 2
	reportsMin := reportsMinContentLines + 2
	if remaining-reportsFull >= notesMinContentLines+2 {
		g.reportsHeight = reportsFull
	} else {
		g.reportsHeight = reportsMin
	}
	if g.reportsHeight > remaining {
		g.reportsHeight = remaining
	}

	g.notesHeight = remaining - g.reportsHeight
	if g.notesHeight < notesMinContentLines+2 {
		// Not enough left for a usable board: drop it and give the height
		// back to Reports rather than rendering an unusable sliver.
		g.reportsHeight += g.notesHeight
		g.notesHeight = 0
	}
	return g
}

// gutterColumn builds the one-column gap between two side-by-side columns as
// a stack of background-carrying spaces. The two-column widths deliberately
// leave this column free (taskWidth + infoWidth == width-1); filling it with
// styled spaces rather than letting JoinHorizontal pad with its own plain
// ones keeps it from rendering as a full-height stripe of the terminal's
// default background.
func gutterColumn(height int) string {
	if height < 1 {
		height = 1
	}
	cell := lipgloss.NewStyle().Background(colorBg).Render(" ")
	rows := make([]string, height)
	for i := range rows {
		rows[i] = cell
	}
	return strings.Join(rows, "\n")
}

// spacerRow fills a complete horizontal gap without relying on JoinVertical's
// unstyled padding. Its width is the same budget as the rows on either side.
func spacerRow(width int) string {
	return lipgloss.NewStyle().Background(colorBg).Render(strings.Repeat(" ", max(0, width)))
}

// joinPaneRows uses explicit, geometry-budgeted page gaps. Empty gaps append no
// rows at all, letting short screens preserve functional pane minima.
func joinPaneRows(top, bottom string, width, gap int) string {
	rows := []string{top}
	for i := 0; i < gap; i++ {
		rows = append(rows, spacerRow(width))
	}
	return strings.Join(append(rows, bottom), "\n")
}
