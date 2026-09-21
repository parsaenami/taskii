package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/parsaenami/taskii/internal/model"
)

// todayEntry is a view-only projection. Selection indexes entries, while
// scrolling indexes display lines (the two optional section headings cost rows).
type todayEntry struct {
	routine *model.Routine
	task    model.Task
}

func (a App) dueRoutines() []model.Routine {
	if a.upcoming || a.timeline {
		return nil
	}
	now := a.now()
	day := now.Format(dateFormat)
	var out []model.Routine
	for _, r := range a.routines {
		if r.CreatedAt.In(now.Location()).Format(dateFormat) > day || !model.ScheduledOnDate(r, now, a.workdays) || r.History[day] == model.RoutineSkipped {
			continue
		}
		out = append(out, r)
	}
	return out
}

func (a App) todayEntries() []todayEntry {
	var out []todayEntry
	for _, r := range a.dueRoutines() {
		r := r
		out = append(out, todayEntry{routine: &r})
	}
	for _, t := range a.activeDayTasks() {
		out = append(out, todayEntry{task: t})
	}
	return out
}

func (a App) selectedTodayRoutine() *model.Routine {
	if a.simple || a.focus != focusToday {
		return nil
	}
	e := a.todayEntries()
	if a.todaySelected < 0 || a.todaySelected >= len(e) {
		return nil
	}
	return e[a.todaySelected].routine
}

func (a App) todayLineSpan(index int) (int, int) {
	entries := a.todayEntries()
	if index < 0 || index >= len(entries) {
		return -1, -1
	}
	if len(a.dueRoutines()) == 0 {
		return index, index
	}
	line := index + 1
	if entries[index].routine == nil {
		line++
	}
	return line, line
}

func renderRoutineLine(r model.Routine, selected bool, width int, bg lipgloss.Color, today string) string {
	if selected {
		bg = colorPanel
	}
	mark := "[ ]"
	style := lipgloss.NewStyle().Foreground(colorAccent).Background(bg)
	if r.History[today] == model.RoutineCompleted {
		mark = "[x]"
		style = lipgloss.NewStyle().Foreground(colorMuted).Background(bg)
	}
	prefix := "  "
	if selected {
		prefix = "> "
	}
	line := style.Render(prefix + mark + " " + fitToWidth(r.Title, max(0, width-6)))
	return padPanelLine(line, width, bg)
}

func (a App) renderTodayList(visible, width int) string {
	entries := a.todayEntries()
	if len(a.dueRoutines()) == 0 {
		return renderTaskList(a.activeDayTasks(), a.todaySelected, a.todayScroll, visible, a.focus == focusToday, false, width, a.now(), a.upcoming)
	}
	day := a.now().Format(dateFormat)
	var rows []string
	heading := lipgloss.NewStyle().Bold(true).Foreground(colorMuted).Background(colorPaneBg)
	rows = append(rows, padPanelLine(heading.Render("ROUTINES"), width, colorPaneBg))
	for i, e := range entries {
		if i == len(a.dueRoutines()) {
			rows = append(rows, padPanelLine(heading.Render("TASKS"), width, colorPaneBg))
		}
		if e.routine != nil {
			rows = append(rows, renderRoutineLine(*e.routine, i == a.todaySelected && a.focus == focusToday, width, colorPaneBg, day))
		} else {
			rows = append(rows, renderTaskLine(e.task, i == a.todaySelected && a.focus == focusToday, false, width, colorPaneBg, a.now(), a.upcoming))
		}
	}
	if len(entries) == len(a.dueRoutines()) {
		rows = append(rows, padPanelLine(heading.Render("TASKS"), width, colorPaneBg))
	}
	scroll := max(0, min(a.todayScroll, len(rows)-1))
	// visibleRowsFor reserves both headings, but headings can scroll out of
	// view. Still render no more than the pane's fixed physical row budget.
	end := min(len(rows), scroll+max(1, visible+2))
	indicator := ""
	if end < len(rows) {
		indicator = fmt.Sprintf("↓ %d more", len(rows)-end)
	}
	if scroll > 0 {
		indicator = fmt.Sprintf("↑ %d more", scroll)
		if end < len(rows) {
			indicator += fmt.Sprintf(" / ↓ %d more", len(rows)-end)
		}
	}
	return strings.Join(append(rows[scroll:end], hintStyle.Render(fitToWidth(indicator, width))), "\n")
}

func (a *App) saveRoutineSnapshot(snapshot []model.Routine) bool {
	if !a.noPersist {
		if err := model.SaveRoutines(snapshot); err != nil {
			a.err = "failed to save routines: " + err.Error()
			return false
		}
	}
	a.routines = snapshot
	a.routineReportScroll = 0
	a.clampRoutineReportScroll()
	a.err = ""
	return true
}

func (a *App) changeRoutine(id string, status model.RoutineStatus) bool {
	if a.routinesLoadErr != nil {
		a.err = "routines unavailable: " + a.routinesLoadErr.Error()
		return false
	}
	copyOf := append([]model.Routine(nil), a.routines...)
	day := a.now().Format(dateFormat)
	for i := range copyOf {
		if copyOf[i].ID != id {
			continue
		}
		if !model.ScheduledOnDate(copyOf[i], a.now(), a.workdays) || copyOf[i].CreatedAt.In(a.now().Location()).Format(dateFormat) > day {
			return false
		}
		copyOf[i].History = make(map[string]model.RoutineStatus, len(a.routines[i].History)+1)
		for k, v := range a.routines[i].History {
			copyOf[i].History[k] = v
		}
		if status == "" {
			delete(copyOf[i].History, day)
		} else {
			copyOf[i].History[day] = status
		}
		return a.saveRoutineSnapshot(copyOf)
	}
	return false
}

func (a *App) toggleRoutine(id string) bool {
	day := a.now().Format(dateFormat)
	for _, r := range a.routines {
		if r.ID == id {
			if r.History[day] == model.RoutineCompleted {
				return a.changeRoutine(id, "")
			}
			return a.changeRoutine(id, model.RoutineCompleted)
		}
	}
	return false
}

func (a *App) skipRoutine(id string) bool {
	day := a.now().Format(dateFormat)
	for _, r := range a.routines {
		if r.ID != id {
			continue
		}
		// Completion is an explicit decision for today. Do not let the skip
		// shortcut overwrite it; the user must first mark the routine undone.
		if r.History[day] == model.RoutineCompleted {
			a.err = "Completed routines cannot be skipped; mark it undone first"
			return false
		}
		return a.changeRoutine(id, model.RoutineSkipped)
	}
	return false
}

// Only a date transition causes reconciliation. An unsuccessful write leaves
// the cursor unchanged so the next tick can retry without losing history.
func (a *App) advanceRoutineDate() {
	day := a.now().Format(dateFormat)
	if day == a.routineDate {
		return
	}
	if a.routinesLoadErr != nil {
		a.routineDate = day
		return
	}
	copyOf, changed, err := reconcileRoutineSnapshot(a.routines, a.now(), a.workdays)
	if err != nil {
		a.err = "failed to reconcile routines: " + err.Error()
		return
	}
	if changed && !a.saveRoutineSnapshot(copyOf) {
		return
	}
	a.routineDate = day
	// A routine can disappear at midnight. Reset the selection as well as
	// scroll so the old numeric index cannot silently target another item.
	a.todaySelected = 0
	a.simpleSelected = 0
	a.todayScroll = 0
	a.simpleScroll = 0
}
