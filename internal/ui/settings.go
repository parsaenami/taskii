package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// settingsFieldKind distinguishes how a field's value is edited and
// displayed: a plain minute count, a session count, a yes/no toggle, or a
// cycled enum (currently just layout).
type settingsFieldKind int

const (
	settingsFieldMinutes settingsFieldKind = iota
	settingsFieldCount
	settingsFieldBool
	settingsFieldLayout
)

// settingsField describes one editable row in the modal. min/max bound
// left/right adjustment for the numeric kinds; layout wraps via layout.next().
type settingsField struct {
	label string
	kind  settingsFieldKind
	min   int
	max   int
}

var settingsFields = []settingsField{
	{label: "Focus session", kind: settingsFieldMinutes, min: 1, max: 180},
	{label: "Short break", kind: settingsFieldMinutes, min: 1, max: 60},
	{label: "Long break", kind: settingsFieldMinutes, min: 1, max: 120},
	{label: "Sessions until long break", kind: settingsFieldCount, min: 1, max: 12},
	{label: "Auto-start next session", kind: settingsFieldBool},
	{label: "Layout", kind: settingsFieldLayout},
}

// settingsModal is a scratch copy of every setting it edits, so Esc can
// discard in-progress changes without touching the live App state (the
// running Pomodoro keeps ticking on its old values until Enter commits).
type settingsModal struct {
	selected int

	workMinutes       int
	shortBreakMinutes int
	longBreakMinutes  int
	longBreakEvery    int
	autoStartNext     bool
	layout            layout
}

// openSettings enters modeSettings, seeding the modal from the live App
// state so it starts showing exactly what's in effect right now.
func (a *App) openSettings() {
	a.settings = settingsModal{
		workMinutes:       a.pomo.workMinutes,
		shortBreakMinutes: a.pomo.shortBreakMinutes,
		longBreakMinutes:  a.pomo.longBreakMinutes,
		longBreakEvery:    a.pomo.longBreakEvery,
		autoStartNext:     a.pomo.autoStartNext,
		layout:            a.layout,
	}
	a.mode = modeSettings
}

// applyAndClose commits the modal's scratch values back onto the App and
// running Pomodoro, then persists and returns to normal mode. The Pomodoro's
// remaining countdown is only rescaled when its OWN phase's duration
// changed — editing the break lengths mid-focus-session shouldn't touch the
// timer that's actually running.
func (a *App) applyAndClose() {
	s := a.settings
	oldPhaseDuration := a.pomo.phaseDuration()

	a.pomo.workMinutes = s.workMinutes
	a.pomo.shortBreakMinutes = s.shortBreakMinutes
	a.pomo.longBreakMinutes = s.longBreakMinutes
	a.pomo.longBreakEvery = s.longBreakEvery
	a.pomo.autoStartNext = s.autoStartNext

	if newDuration := a.pomo.phaseDuration(); newDuration != oldPhaseDuration {
		a.pomo.remaining = newDuration
	}

	a.layout = s.layout
	a.clampSelections()

	a.mode = modeNormal
	a.status = "Settings saved"
	a.saveSettings()
}

func (a App) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.mode = modeNormal
		a.status = "Settings cancelled"
		return a, nil

	case "enter":
		a.applyAndClose()
		return a, nil

	case "up", "k":
		a.settings.selected = (a.settings.selected - 1 + len(settingsFields)) % len(settingsFields)
		return a, nil

	case "down", "j", "tab":
		a.settings.selected = (a.settings.selected + 1) % len(settingsFields)
		return a, nil

	case "left", "h", "right", "l":
		delta := 1
		if msg.String() == "left" || msg.String() == "h" {
			delta = -1
		}
		a.settings.adjust(delta)
		return a, nil
	}
	return a, nil
}

// adjust changes the selected field by delta steps, clamping numeric fields
// to their [min, max] and toggling/cycling the others (delta's sign is
// irrelevant to a toggle or a two-way layout cycle, only that a key was
// pressed).
func (m *settingsModal) adjust(delta int) {
	f := settingsFields[m.selected]
	switch f.kind {
	case settingsFieldMinutes:
		v := m.fieldValue(m.selected) + delta
		m.setFieldValue(m.selected, clampInt(v, f.min, f.max))
	case settingsFieldCount:
		v := m.longBreakEvery + delta
		m.longBreakEvery = clampInt(v, f.min, f.max)
	case settingsFieldBool:
		m.autoStartNext = !m.autoStartNext
	case settingsFieldLayout:
		if delta < 0 {
			// layout only exposes next(); step backward by cycling forward
			// through the rest of the list.
			for i := 0; i < 3; i++ {
				m.layout = m.layout.next()
			}
			return
		}
		m.layout = m.layout.next()
	}
}

func (m *settingsModal) fieldValue(i int) int {
	switch i {
	case 0:
		return m.workMinutes
	case 1:
		return m.shortBreakMinutes
	case 2:
		return m.longBreakMinutes
	}
	return 0
}

func (m *settingsModal) setFieldValue(i, v int) {
	switch i {
	case 0:
		m.workMinutes = v
	case 1:
		m.shortBreakMinutes = v
	case 2:
		m.longBreakMinutes = v
	}
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// settingsValueText renders the right-hand value column for one field.
func (m settingsModal) valueText(i int) string {
	f := settingsFields[i]
	switch f.kind {
	case settingsFieldMinutes:
		return fmt.Sprintf("%d min", m.fieldValue(i))
	case settingsFieldCount:
		return strconv.Itoa(m.longBreakEvery)
	case settingsFieldBool:
		if m.autoStartNext {
			return "On"
		}
		return "Off"
	case settingsFieldLayout:
		return m.layout.String()
	}
	return ""
}

// settingsModalWidth is fixed rather than proportional to the terminal: a
// settings form reads better at a stable width than one that stretches with
// an ultra-wide window.
const settingsModalWidth = 46

// renderSettingsModal draws the modal as a titled, bordered pane (via the
// same renderPane used for every other box in the app) so it matches the
// rest of the UI's visual language rather than introducing a second style.
func renderSettingsModal(m settingsModal) string {
	var b strings.Builder
	contentWidth := settingsModalWidth - 4

	labelStyle := lipgloss.NewStyle().Foreground(colorText).Background(colorPaneBg)
	valueStyle := lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Background(colorPaneBg)
	selValueStyle := lipgloss.NewStyle().Bold(true).Foreground(colorText).Background(colorPanel)
	selLabelStyle := lipgloss.NewStyle().Bold(true).Foreground(colorText).Background(colorPanel)
	blank := lipgloss.NewStyle().Background(colorPaneBg)

	for i, f := range settingsFields {
		label := f.label
		value := m.valueText(i)

		ls, vs := labelStyle, valueStyle
		rowBg := colorPaneBg
		if i == m.selected {
			ls, vs = selLabelStyle, selValueStyle
			rowBg = colorPanel
		}

		labelRendered := ls.Render(" " + label)
		valueRendered := vs.Render(value + " ")

		pad := contentWidth - lipgloss.Width(labelRendered) - lipgloss.Width(valueRendered)
		if pad < 1 {
			pad = 1
		}
		row := labelRendered + lipgloss.NewStyle().Background(rowBg).Render(strings.Repeat(" ", pad)) + valueRendered
		b.WriteString(row)
		b.WriteString("\n")
	}

	b.WriteString(blank.Render(strings.Repeat(" ", contentWidth)))
	b.WriteString("\n")

	hint := paneKeyStyle.Render("[↑/↓]") + paneKeyLabelStyle.Render(" select  ") +
		paneKeyStyle.Render("[←/→]") + paneKeyLabelStyle.Render(" change  ") +
		paneKeyStyle.Render("[enter]") + paneKeyLabelStyle.Render(" save  ") +
		paneKeyStyle.Render("[esc]") + paneKeyLabelStyle.Render(" cancel")
	b.WriteString(centerLine(hint, contentWidth))

	body := strings.TrimRight(b.String(), "\n")
	lines := strings.Count(body, "\n") + 1
	return renderPane("Settings", body, true, settingsModalWidth, lines+2)
}

// overlaySettingsModal places the settings box centered over the already
// fully-rendered page, with the page dimmed underneath so the app is still
// visible behind the modal rather than replaced by a solid backdrop.
// Building it this way (compose over a finished frame) rather than
// threading modal state through View()'s per-pane rendering keeps every
// other pane's layout math untouched by whether the modal is open — only
// the top-level composite changes.
func overlaySettingsModal(page string, m settingsModal, width, height int) string {
	modal := renderSettingsModal(m)
	dimmed := dimANSI(page, 0.45)
	return compositeOver(dimmed, modal, width, height)
}
