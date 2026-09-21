package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/parsaenami/taskii/internal/model"
)

type routineModal struct {
	selected, scroll int
	editID           string
	field            int // title, schedule, then seven weekdays
	title            textinput.Model
	schedule         model.RoutineSchedule
	days             [7]bool
	errorText        string
}

func (a *App) openRoutineManager() {
	a.mode = modeRoutineManager
	a.routineUI.errorText = ""
	a.clampRoutineSelection()
}

func (a *App) clampRoutineSelection() {
	a.routineUI.selected = max(0, min(a.routineUI.selected, len(a.routines)-1))
	visible := a.routineManagerVisibleRows()
	a.routineUI.scroll = max(0, min(a.routineUI.scroll, a.routineUI.selected))
	if a.routineUI.selected >= a.routineUI.scroll+visible {
		a.routineUI.scroll = a.routineUI.selected - visible + 1
	}
}

// The manager pins two key-hint rows to the modal footer, keeps one blank row
// above them, and reserves one content row for errors. Keep the cursor and the
// renderer on the same viewport budget.
func (a App) routineManagerVisibleRows() int {
	return max(1, a.routineModalHeight()-6)
}

func (a App) managerSelectedID() string {
	if a.routineUI.selected < 0 || a.routineUI.selected >= len(a.routines) {
		return ""
	}
	return a.routines[a.routineUI.selected].ID
}

func (a App) managerSelectedTitle() string {
	if a.managerSelectedID() == "" {
		return ""
	}
	return a.routines[a.routineUI.selected].Title
}

func (a App) updateRoutineManager(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.mode = modeNormal
	case "up", "k":
		a.routineUI.selected--
		a.clampRoutineSelection()
	case "down", "j":
		a.routineUI.selected++
		a.clampRoutineSelection()
	case "a":
		return a.startRoutineEdit(-1)
	case "enter", "e":
		if a.managerSelectedID() != "" {
			return a.startRoutineEdit(a.routineUI.selected)
		}
	case "s":
		if id := a.managerSelectedID(); id != "" && a.routines[a.routineUI.selected].History[a.now().Format(dateFormat)] == model.RoutineSkipped {
			a.changeRoutine(id, "")
		}
	case "d":
		if id := a.managerSelectedID(); id != "" {
			a.deleteItemID = id
			a.deleteReturn = modeRoutineManager
			a.mode = modeConfirmDelete
		}
	}
	return a, nil
}

func (a App) startRoutineEdit(index int) (tea.Model, tea.Cmd) {
	a.routineUI.editID = ""
	a.routineUI.field = 0
	a.routineUI.errorText = ""
	a.routineUI.title = textinput.New()
	a.routineUI.title.Prompt = ""
	a.routineUI.title.CharLimit = 120
	a.routineUI.schedule = model.ScheduleEveryDay
	a.routineUI.days = [7]bool{}
	if index >= 0 && index < len(a.routines) {
		r := a.routines[index]
		a.routineUI.editID = r.ID
		a.routineUI.title.SetValue(r.Title)
		a.routineUI.schedule = r.Schedule
		for _, day := range r.CustomWeekdays {
			a.routineUI.days[day] = true
		}
	}
	a.routineUI.title.Focus()
	a.mode = modeRoutineEditor
	return a, textinput.Blink
}

func (a App) updateRoutineEditor(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m := &a.routineUI
	switch msg.String() {
	case "esc":
		a.mode = modeRoutineManager
		m.title.Blur()
		return a, nil
	case "ctrl+s":
		if a.commitRoutineEdit() {
			a.mode = modeRoutineManager
			m.title.Blur()
		}
		return a, nil
	case "tab", "down":
		m.field++
		limit := 1
		if m.schedule == model.ScheduleCustom {
			limit = 8
		}
		if m.field > limit {
			m.field = 0
		}
	case "shift+tab", "up":
		m.field--
		if m.field < 0 {
			if m.schedule == model.ScheduleCustom {
				m.field = 8
			} else {
				m.field = 1
			}
		}
	case "left", "right":
		if m.field == 0 {
			var cmd tea.Cmd
			m.title, cmd = m.title.Update(msg)
			return a, cmd
		}
		if m.field == 1 {
			options := []model.RoutineSchedule{model.ScheduleEveryDay, model.ScheduleWorkdays, model.ScheduleCustom}
			for i, s := range options {
				if s == m.schedule {
					step := 1
					if msg.String() == "left" {
						step = 2
					}
					m.schedule = options[(i+step)%len(options)]
					break
				}
			}
			if m.schedule != model.ScheduleCustom && m.field > 1 {
				m.field = 1
			}
		}
	case " ":
		if m.schedule == model.ScheduleCustom && m.field >= 2 && m.field <= 8 {
			day := (int(a.weekStart) + m.field - 2) % 7
			m.days[day] = !m.days[day]
		} else if m.field == 0 {
			var cmd tea.Cmd
			m.title, cmd = m.title.Update(msg)
			return a, cmd
		}
	default:
		if m.field == 0 {
			var cmd tea.Cmd
			m.title, cmd = m.title.Update(msg)
			return a, cmd
		}
	}
	m.errorText = ""
	if m.field == 0 {
		m.title.Focus()
	} else {
		m.title.Blur()
	}
	return a, nil
}

func (a *App) commitRoutineEdit() bool {
	m := &a.routineUI
	title := strings.TrimSpace(m.title.Value())
	if title == "" {
		m.errorText = "Title cannot be empty"
		return false
	}
	var days []time.Weekday
	if m.schedule == model.ScheduleCustom {
		for n := 0; n < 7; n++ {
			day := time.Weekday((int(a.weekStart) + n) % 7)
			if m.days[day] {
				days = append(days, day)
			}
		}
		if len(days) == 0 {
			m.errorText = "Select at least one weekday"
			return false
		}
	}
	if a.routinesLoadErr != nil {
		m.errorText = "Routines unavailable: " + a.routinesLoadErr.Error()
		return false
	}
	copyOf := append([]model.Routine(nil), a.routines...)
	if m.editID == "" {
		base := strconv.FormatInt(a.now().UnixNano(), 36)
		id := base
		for suffix := 1; ; suffix++ {
			collision := false
			for _, r := range copyOf {
				if r.ID == id {
					collision = true
					break
				}
			}
			if !collision {
				break
			}
			id = fmt.Sprintf("%s-%d", base, suffix)
		}
		copyOf = append(copyOf, model.Routine{ID: id, Title: title, CreatedAt: a.now(), Schedule: m.schedule, CustomWeekdays: days})
	} else {
		found := false
		for i := range copyOf {
			if copyOf[i].ID == m.editID {
				// Settle the OLD recurrence before changing its schedule. The cursor
				// and history of previous days are retained on the replacement.
				copyOf[i].History = make(map[string]model.RoutineStatus, len(a.routines[i].History))
				for k, v := range a.routines[i].History {
					copyOf[i].History[k] = v
				}
				if _, err := model.ReconcileRoutine(&copyOf[i], a.now(), a.workdays); err != nil {
					m.errorText = err.Error()
					return false
				}
				copyOf[i].Title, copyOf[i].Schedule, copyOf[i].CustomWeekdays = title, m.schedule, days
				found = true
				break
			}
		}
		if !found {
			m.errorText = "Routine changed; edit cancelled"
			return false
		}
	}
	if !a.saveRoutineSnapshot(copyOf) {
		m.errorText = a.err
		return false
	}
	a.routineUI.selected = len(copyOf) - 1
	for i, r := range copyOf {
		if r.ID == m.editID {
			a.routineUI.selected = i
			break
		}
	}
	a.clampRoutineSelection()
	a.clampSelections()
	return true
}

func (a *App) deleteRoutine(id string) {
	if a.routinesLoadErr != nil {
		a.routineUI.errorText = "Routines unavailable"
		return
	}
	copyOf := make([]model.Routine, 0, len(a.routines))
	for _, r := range a.routines {
		if r.ID != id {
			copyOf = append(copyOf, r)
		}
	}
	if len(copyOf) == len(a.routines) {
		a.routineUI.errorText = "Routine changed; delete cancelled"
		return
	}
	if !a.saveRoutineSnapshot(copyOf) {
		a.routineUI.errorText = a.err
		return
	}
	a.clampRoutineSelection()
	a.clampSelections()
}

func (a App) routineModalWidth() int  { return max(4, min(58, a.width-4)) }
func (a App) routineModalHeight() int { return max(3, min(19, a.height-2)) }

func routineModalHintLine(keys ...helpKey) string {
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts,
			paneKeyStyle.Render("["+key.key+"]")+
				paneKeyLabelStyle.Render(" "+key.label))
	}
	return strings.Join(parts, paneKeySepStyle.Render("  │  "))
}

func routineManagerHints() []string {
	return []string{
		routineModalHintLine(helpKey{"a", "add"}, helpKey{"enter/e", "edit"}, helpKey{"d", "delete"}),
		routineModalHintLine(helpKey{"s", "restore skip"}, helpKey{"↑/↓", "move"}, helpKey{"esc", "close"}),
	}
}

func routineEditorHints() []string {
	return []string{
		routineModalHintLine(helpKey{"tab/↑/↓", "field"}, helpKey{"←/→", "schedule"}, helpKey{"space", "day"}),
		routineModalHintLine(helpKey{"ctrl+s", "save"}, helpKey{"esc", "cancel"}),
	}
}

// pinRoutineModalFooter pads the content before appending the key hints, so
// list length and editor schedule never move the controls away from the
// modal's bottom edge. Every row is also fitted to the pane surface here.
func pinRoutineModalFooter(lines, footer []string, width, capacity int) []string {
	bodyRows := max(0, capacity-len(footer))
	if len(lines) > bodyRows {
		lines = lines[:bodyRows]
	}
	blank := lipgloss.NewStyle().Background(colorPaneBg).Render(strings.Repeat(" ", width))
	for len(lines) < bodyRows {
		lines = append(lines, blank)
	}
	for _, line := range footer {
		lines = append(lines, padPanelLine(line, width, colorPaneBg))
	}
	return lines
}

func (a App) renderRoutineModal() string {
	w, h := a.routineModalWidth(), a.routineModalHeight()
	inside := max(1, w-4)
	capacity := max(1, h-2)
	label := lipgloss.NewStyle().Foreground(colorMuted).Background(colorPaneBg)
	selected := lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Background(colorPanel)
	var lines []string
	add := func(s string) { lines = append(lines, padPanelLine(s, inside, colorPaneBg)) }
	if a.mode == modeRoutineEditor {
		m := a.routineUI
		add(label.Render("Title"))
		title := m.title.Value()
		if m.field == 0 {
			title = "▸ " + title + "▌"
		} else {
			title = "  " + title
		}
		add(label.Render(fitToWidth(title, inside)))
		style := label
		if m.field == 1 {
			style = selected
		}
		add(style.Render(fitToWidth(fmt.Sprintf("Schedule: ◂ %s ▸", m.schedule), inside)))
		if m.schedule == model.ScheduleCustom {
			for n := 0; n < 7; n++ {
				day := time.Weekday((int(a.weekStart) + n) % 7)
				mark := "[ ]"
				if m.days[day] {
					mark = "[x]"
				}
				st := label
				if m.field == n+2 {
					st = selected
				}
				add(st.Render(fitToWidth(mark+" "+day.String(), inside)))
			}
		}
		if m.errorText != "" {
			add(lipgloss.NewStyle().Foreground(colorDanger).Background(colorPaneBg).Render(fitToWidth(m.errorText, inside)))
		}
		lines = pinRoutineModalFooter(lines, routineEditorHints(), inside, capacity)
	} else {
		if a.mode == modeConfirmDelete && a.deleteReturn == modeRoutineManager {
			// The status line owns the one confirmation prompt and its hints.
			// Keep the manager list visible, but never ask the same question twice.
			add(label.Render(fitToWidth("Confirm below to remove this routine and its history", inside)))
		}
		if len(a.routines) == 0 {
			add(label.Render("No routines yet"))
		}
		viewport := a.routineManagerVisibleRows()
		if a.routineUI.errorText == "" {
			viewport++
		}
		start := max(0, min(a.routineUI.scroll, len(a.routines)-1))
		for i := start; i < min(len(a.routines), start+viewport); i++ {
			r := a.routines[i]
			state := "not scheduled"
			if model.ScheduledOnDate(r, a.now(), a.workdays) && r.CreatedAt.In(a.now().Location()).Format(dateFormat) <= a.now().Format(dateFormat) {
				state = "pending"
				if s := r.History[a.now().Format(dateFormat)]; s != "" {
					state = string(s)
				}
			}
			st := label
			marker := "  "
			if i == a.routineUI.selected {
				st = selected
				marker = "▸ "
			}
			add(st.Render(fitToWidth(marker+r.Title+" · "+state+" · "+string(r.Schedule), inside)))
		}
		if a.routineUI.errorText != "" {
			add(lipgloss.NewStyle().Foreground(colorDanger).Background(colorPaneBg).Render(fitToWidth(a.routineUI.errorText, inside)))
		}
		if a.mode == modeConfirmDelete && a.deleteReturn == modeRoutineManager {
			lines = pinRoutineModalFooter(lines, nil, inside, capacity)
		} else {
			lines = pinRoutineModalFooter(lines, routineManagerHints(), inside, capacity)
		}
	}
	if len(lines) > capacity {
		lines = lines[:capacity]
	}
	return renderPane("Routines", strings.Join(lines, "\n"), true, w, h)
}
