package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// settingsSection is one entry in the modal's left-hand nav column. Each
// section owns an independent content pane on the right, so the modal is a
// master/detail view rather than one flat list of fields.
type settingsSection int

const (
	sectionPomodoro settingsSection = iota
	sectionLayout
	sectionTheme
	sectionAbout

	sectionCount = 4
)

var settingsSectionNames = [sectionCount]string{"Pomodoro", "Layout", "Theme", "About"}

func (s settingsSection) String() string {
	if s >= 0 && int(s) < sectionCount {
		return settingsSectionNames[s]
	}
	return ""
}

// settingsFocus is which of the modal's two columns the cursor is in. The
// nav column chooses the section; the content column drives that section's
// own cursor. Both columns stay visible either way — only the highlight and
// the key bindings move.
type settingsFocus int

const (
	focusSettingsNav settingsFocus = iota
	focusSettingsContent
)

// settingsFieldKind distinguishes how a field's value is edited and
// displayed: a plain minute count, a session count, or a yes/no toggle.
type settingsFieldKind int

const (
	settingsFieldMinutes settingsFieldKind = iota
	settingsFieldCount
	settingsFieldBool
)

// settingsField describes one editable row in the Pomodoro section. min/max
// bound left/right adjustment for the numeric kinds.
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
}

// allLayouts is the ordered list the Layout section navigates. Kept explicit
// rather than derived from the layoutNames map so the order on screen is
// stable (Go map iteration order is not).
var allLayouts = []layout{layoutTasksLeft, layoutTasksRight, layoutStacked, layoutThreeColumn}

// settingsModal is a scratch copy of every setting it edits, so Esc can
// discard in-progress changes without touching the live App state (the
// running Pomodoro keeps ticking on its old values until Enter commits).
//
// Layout and Theme are the exception: both are PREVIEWED live as the cursor
// moves, because the only meaningful way to evaluate them is to see them
// applied. origLayout/origTheme hold what Esc restores — seeded from the
// pre-modal values, then re-seeded each time enter commits a choice, so
// cancelling discards only the previews since the last explicit choice.
type settingsModal struct {
	section settingsSection
	focus   settingsFocus

	// Pomodoro section.
	pomoCursor        int
	workMinutes       int
	shortBreakMinutes int
	longBreakMinutes  int
	longBreakEvery    int
	autoStartNext     bool

	// Layout section. layoutCursor is the previewed layout; layoutChosen is
	// the one actually committed with enter/space (shown as "selected");
	// origLayout is what Esc restores.
	layoutCursor int
	layoutChosen layout
	origLayout   layout

	// Theme section. Mirrors the standalone theme picker's state: a filter
	// input over the full catalog, with a cursor and scroll offset.
	themeFilter  textinput.Model
	themeMatches []Theme
	themeCursor  int
	themeScroll  int
	themeChosen  string
	origTheme    string
}

// settingsThemeVisibleItems is how many theme rows the Theme content pane
// shows at once. It also fixes the modal's content height: every section is
// padded to the same number of lines so the box doesn't resize as you move
// between sections.
const settingsThemeVisibleItems = 9

// settingsContentLines is the fixed height of the right-hand content pane,
// sized to the tallest section (Theme: filter + separator + 9 rows).
const settingsContentLines = settingsThemeVisibleItems + 2

// openSettings enters modeSettings, seeding the modal from the live App
// state so it starts showing exactly what's in effect right now.
func (a *App) openSettings() tea.Cmd {
	m := settingsModal{
		section: sectionPomodoro,
		focus:   focusSettingsNav,

		workMinutes:       a.pomo.workMinutes,
		shortBreakMinutes: a.pomo.shortBreakMinutes,
		longBreakMinutes:  a.pomo.longBreakMinutes,
		longBreakEvery:    a.pomo.longBreakEvery,
		autoStartNext:     a.pomo.autoStartNext,

		layoutChosen: a.layout,
		origLayout:   a.layout,

		themeChosen: currentTheme().Name,
		origTheme:   currentTheme().Name,
	}
	for i, l := range allLayouts {
		if l == a.layout {
			m.layoutCursor = i
			break
		}
	}

	m.themeFilter = textinput.New()
	m.themeFilter.Prompt = "Find: "
	m.themeFilter.Placeholder = "type to filter (e.g. gruvbox, nord)..."
	m.themeFilter.Focus()
	m.themeFilter.TextStyle = lipgloss.NewStyle().Foreground(colorText).Background(colorPaneBg)
	m.themeFilter.PromptStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Background(colorPaneBg)
	m.themeFilter.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorMuted).Background(colorPaneBg)
	m.themeFilter.Cursor.Style = lipgloss.NewStyle().Foreground(colorAccent).Background(colorPaneBg)
	m.filterThemes("")

	a.settings = m
	a.mode = modeSettings
	return textinput.Blink
}

// openSettingsAt opens the modal focused directly on one section's content,
// so the `T` shortcut can land straight in the theme list instead of making
// the user walk the nav column.
func (a *App) openSettingsAt(s settingsSection) tea.Cmd {
	cmd := a.openSettings()
	a.settings.section = s
	a.settings.focus = focusSettingsContent
	return cmd
}

// filterThemes narrows the catalog to the query, matching on the theme's own
// name or on its source (so "curated"/"tint"/"custom" work as category
// filters), and re-homes the cursor on the currently active theme.
func (m *settingsModal) filterThemes(query string) {
	q := strings.ToLower(strings.TrimSpace(query))
	all := allAvailableThemes()
	if q == "" {
		m.themeMatches = all
	} else {
		var matches []Theme
		for _, th := range all {
			nameMatch := strings.Contains(strings.ToLower(th.Name), q)
			sourceMatch := strings.Contains(strings.ToLower(string(th.Source)), q)
			tintMatch := th.Source == SourceBubbletint && strings.Contains("tint", q)
			curatedMatch := th.Source == SourceCurated && strings.Contains("curated", q)
			customMatch := th.Source == SourceCustom && strings.Contains("custom", q)
			if nameMatch || sourceMatch || tintMatch || curatedMatch || customMatch {
				matches = append(matches, th)
			}
		}
		m.themeMatches = matches
	}
	curr := currentTheme().Name
	m.themeCursor = 0
	for i, th := range m.themeMatches {
		if strings.EqualFold(th.Name, curr) {
			m.themeCursor = i
			break
		}
	}
	m.themeScroll = 0
	m.syncThemeScroll()
}

func (m *settingsModal) syncThemeScroll() {
	if m.themeCursor < m.themeScroll {
		m.themeScroll = m.themeCursor
	}
	if m.themeCursor >= m.themeScroll+settingsThemeVisibleItems {
		m.themeScroll = m.themeCursor - settingsThemeVisibleItems + 1
	}
	if m.themeScroll < 0 {
		m.themeScroll = 0
	}
}

// applyAndClose commits the modal's scratch values back onto the running
// Pomodoro, then persists and returns to normal mode. The Pomodoro's
// remaining countdown is only rescaled when its OWN phase's duration
// changed — editing the break lengths mid-focus-session shouldn't touch the
// timer that's actually running.
//
// Layout and theme are already live (previewed as the cursor moved), so
// committing them means pinning the CHOSEN value rather than whatever the
// cursor happens to be resting on.
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

	a.layout = s.layoutChosen
	setThemeByName(s.themeChosen)
	a.clampSelections()

	a.mode = modeNormal
	a.status = "Settings saved"
	a.saveSettings()
}

// cancelSettings discards the modal, restoring the layout and theme that
// were in effect before it opened — both having been mutated live by the
// preview-on-focus behaviour.
func (a *App) cancelSettings() {
	a.layout = a.settings.origLayout
	setThemeByName(a.settings.origTheme)
	a.clampSelections()
	a.mode = modeNormal
	a.status = "Settings cancelled"
}

func (a App) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Esc and ctrl+c always back out, whichever column has focus and even
	// while the theme filter is capturing text.
	switch msg.String() {
	case "esc", "ctrl+c":
		a.cancelSettings()
		return a, nil
	}

	if a.settings.focus == focusSettingsNav {
		return a.updateSettingsNav(msg)
	}
	return a.updateSettingsContent(msg)
}

// updateSettingsNav drives the left column: up/down picks the section, and
// right/l/tab/enter steps into that section's content.
func (a App) updateSettingsNav(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		a.settings.section = (a.settings.section - 1 + sectionCount) % sectionCount
		return a, nil

	case "down", "j":
		a.settings.section = (a.settings.section + 1) % sectionCount
		return a, nil

	case "right", "l", "tab", "enter", " ":
		// About has nothing to navigate, so stepping in would trap the
		// cursor in a pane with no cursor.
		if a.settings.section != sectionAbout {
			a.settings.focus = focusSettingsContent
		}
		return a, nil

	case "ctrl+s":
		a.applyAndClose()
		return a, nil
	}
	return a, nil
}

// updateSettingsContent drives the right column, dispatching to whichever
// section is showing. left/h hands focus back to the nav column — except in
// the Pomodoro and Theme sections, where left/h is already spoken for
// (value adjustment and text editing respectively) and `tab` is the way
// back instead.
func (a App) updateSettingsContent(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch a.settings.section {
	case sectionPomodoro:
		return a.updateSettingsPomodoro(msg)
	case sectionLayout:
		return a.updateSettingsLayout(msg)
	case sectionTheme:
		return a.updateSettingsTheme(msg)
	}
	a.settings.focus = focusSettingsNav
	return a, nil
}

func (a App) updateSettingsPomodoro(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		a.settings.focus = focusSettingsNav
		return a, nil

	case "up", "k":
		a.settings.pomoCursor = (a.settings.pomoCursor - 1 + len(settingsFields)) % len(settingsFields)
		return a, nil

	case "down", "j":
		a.settings.pomoCursor = (a.settings.pomoCursor + 1) % len(settingsFields)
		return a, nil

	case "left", "h", "right", "l":
		delta := 1
		if msg.String() == "left" || msg.String() == "h" {
			delta = -1
		}
		a.settings.adjust(delta)
		return a, nil

	case "enter", "ctrl+s":
		a.applyAndClose()
		return a, nil
	}
	return a, nil
}

// updateSettingsLayout previews the focused layout immediately — the only
// way to judge a layout is to see it — and commits it as the chosen one on
// enter/space.
func (a App) updateSettingsLayout(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left", "h", "tab":
		a.settings.focus = focusSettingsNav
		return a, nil

	case "up", "k":
		a.settings.layoutCursor = (a.settings.layoutCursor - 1 + len(allLayouts)) % len(allLayouts)
		a.previewLayout()
		return a, nil

	case "down", "j":
		a.settings.layoutCursor = (a.settings.layoutCursor + 1) % len(allLayouts)
		a.previewLayout()
		return a, nil

	case "enter", " ":
		a.settings.layoutChosen = allLayouts[a.settings.layoutCursor]
		a.layout = a.settings.layoutChosen
		// Choosing commits: it also becomes the new restore point, so a
		// later Esc backs out only the previews made AFTER this choice
		// rather than undoing the choice itself.
		a.settings.origLayout = a.settings.layoutChosen
		a.clampSelections()
		a.status = "Layout: " + a.settings.layoutChosen.String()
		a.saveSettings()
		return a, nil

	case "ctrl+s":
		a.applyAndClose()
		return a, nil
	}
	return a, nil
}

// previewLayout applies the cursor's layout to the live App so the change is
// visible through the modal's dimmed backdrop. Selections are re-clamped
// because pane heights differ between layouts — a row visible in one may not
// exist in another.
func (a *App) previewLayout() {
	a.layout = allLayouts[a.settings.layoutCursor]
	a.clampSelections()
}

// updateSettingsTheme mirrors the standalone theme picker: arrows move and
// live-preview, enter pins the choice, and everything else is typed into the
// filter. `tab` (not left/h, which belong to the text input) returns to nav.
func (a App) updateSettingsTheme(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		a.settings.focus = focusSettingsNav
		return a, nil

	case "up", "ctrl+p", "ctrl+k":
		if len(a.settings.themeMatches) > 0 {
			a.settings.themeCursor--
			if a.settings.themeCursor < 0 {
				a.settings.themeCursor = len(a.settings.themeMatches) - 1
			}
			a.settings.syncThemeScroll()
			applyTheme(a.settings.themeMatches[a.settings.themeCursor])
		}
		return a, nil

	case "down", "ctrl+n", "ctrl+j":
		if len(a.settings.themeMatches) > 0 {
			a.settings.themeCursor++
			if a.settings.themeCursor >= len(a.settings.themeMatches) {
				a.settings.themeCursor = 0
			}
			a.settings.syncThemeScroll()
			applyTheme(a.settings.themeMatches[a.settings.themeCursor])
		}
		return a, nil

	case "enter":
		if a.settings.themeCursor >= 0 && a.settings.themeCursor < len(a.settings.themeMatches) {
			selected := a.settings.themeMatches[a.settings.themeCursor]
			a.settings.themeChosen = selected.Name
			setThemeByName(selected.Name)
			// Choosing commits: see the Layout section's note — the chosen
			// theme becomes the new Esc restore point.
			a.settings.origTheme = selected.Name
			a.status = "Theme: " + selected.Name
			a.saveSettings()
		}
		return a, nil
	}

	var cmd tea.Cmd
	oldVal := a.settings.themeFilter.Value()
	a.settings.themeFilter, cmd = a.settings.themeFilter.Update(msg)
	if a.settings.themeFilter.Value() != oldVal {
		a.settings.filterThemes(a.settings.themeFilter.Value())
		if len(a.settings.themeMatches) > 0 {
			applyTheme(a.settings.themeMatches[a.settings.themeCursor])
		}
	}
	return a, cmd
}

// adjust changes the selected Pomodoro field by delta steps, clamping
// numeric fields to their [min, max] and toggling the boolean (delta's sign
// is irrelevant to a toggle, only that a key was pressed).
func (m *settingsModal) adjust(delta int) {
	f := settingsFields[m.pomoCursor]
	switch f.kind {
	case settingsFieldMinutes:
		v := m.fieldValue(m.pomoCursor) + delta
		m.setFieldValue(m.pomoCursor, clampInt(v, f.min, f.max))
	case settingsFieldCount:
		v := m.longBreakEvery + delta
		m.longBreakEvery = clampInt(v, f.min, f.max)
	case settingsFieldBool:
		m.autoStartNext = !m.autoStartNext
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

// valueText renders the right-hand value column for one Pomodoro field.
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
	}
	return ""
}

// settingsModalWidth is fixed rather than proportional to the terminal: a
// settings form reads better at a stable width than one that stretches with
// an ultra-wide window. It's wider than the old single-column modal because
// the box now holds a nav column AND a content pane side by side.
const settingsModalWidth = 64

// settingsNavWidth is the display width of the left-hand section list,
// including its one-column leading gutter for the ▸ cursor. Fixed so the
// divider between the columns sits on the same screen column no matter which
// section names are showing.
const settingsNavWidth = 14

// renderSettingsModal draws the modal as a titled, bordered pane (via the
// same renderPane used for every other box in the app) so it matches the
// rest of the UI's visual language rather than introducing a second style.
//
// The body is composed column-wise: the nav list and the content pane are
// each rendered as a slice of exactly-padded lines, then zipped together
// row by row with a vertical divider between them. Building it this way —
// rather than with lipgloss.JoinHorizontal — keeps every cell's background
// explicitly ours, which is what stops the box from showing stripes of the
// terminal's default background between the columns.
func (a App) renderSettingsModal() string {
	contentWidth := settingsModalWidth - 4
	paneWidth := contentWidth - settingsNavWidth - 1 // -1 for the divider column

	nav := a.renderSettingsNav()
	content := a.renderSettingsContent(paneWidth)

	rows := settingsContentLines
	divStyle := lipgloss.NewStyle().Foreground(colorBorder).Background(colorPaneBg)
	blank := lipgloss.NewStyle().Background(colorPaneBg)

	var b strings.Builder
	for i := 0; i < rows; i++ {
		navLine := blank.Render(strings.Repeat(" ", settingsNavWidth))
		if i < len(nav) {
			navLine = nav[i]
		}
		contentLine := blank.Render(strings.Repeat(" ", paneWidth))
		if i < len(content) {
			contentLine = content[i]
		}
		b.WriteString(navLine + divStyle.Render("│") + contentLine)
		b.WriteString("\n")
	}

	b.WriteString(blank.Render(strings.Repeat(" ", contentWidth)))
	b.WriteString("\n")
	b.WriteString(centerLine(a.settingsHint(), contentWidth))

	body := b.String()
	lines := strings.Count(body, "\n") + 1
	return renderPane("Settings", body, true, settingsModalWidth, lines+2)
}

// settingsHint is the key-hint footer, which changes with the focused column
// and section — the keys genuinely differ (the Pomodoro pane uses ←/→ to
// change a value while the Layout pane uses enter to choose), so one static
// hint line would be wrong in most states.
func (a App) settingsHint() string {
	key := func(k, label string) string {
		return paneKeyStyle.Render("["+k+"]") + paneKeyLabelStyle.Render(" "+label+"  ")
	}
	var b strings.Builder
	if a.settings.focus == focusSettingsNav {
		b.WriteString(key("↑/↓", "section"))
		if a.settings.section != sectionAbout {
			b.WriteString(key("→", "open"))
		}
	} else {
		switch a.settings.section {
		case sectionPomodoro:
			b.WriteString(key("↑/↓", "field"))
			b.WriteString(key("←/→", "change"))
			b.WriteString(key("tab", "back"))
		case sectionLayout:
			b.WriteString(key("↑/↓", "preview"))
			b.WriteString(key("enter", "choose"))
			b.WriteString(key("←", "back"))
		case sectionTheme:
			b.WriteString(key("↑/↓", "preview"))
			b.WriteString(key("enter", "choose"))
			b.WriteString(key("tab", "back"))
		}
	}
	b.WriteString(paneKeyStyle.Render("[esc]") + paneKeyLabelStyle.Render(" close"))
	return b.String()
}

// renderSettingsNav builds the left column: one row per section, with a ▸
// marker and a highlighted background on the current one. The highlight is
// dimmed (accent text on the pane background, no panel fill) while focus is
// in the content column, so it's always clear which side the keys act on.
func (a App) renderSettingsNav() []string {
	lines := make([]string, 0, settingsContentLines)
	navFocused := a.settings.focus == focusSettingsNav

	for i := 0; i < sectionCount; i++ {
		sec := settingsSection(i)
		isSel := sec == a.settings.section

		rowBg := colorPaneBg
		prefix := "  "
		nameStyle := lipgloss.NewStyle().Foreground(colorText).Background(rowBg)
		if isSel {
			prefix = "▸ "
			if navFocused {
				rowBg = colorPanel
				nameStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Background(rowBg)
			} else {
				nameStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Background(rowBg)
			}
		}
		prefixStyle := lipgloss.NewStyle().Foreground(colorAccent).Background(rowBg)
		row := prefixStyle.Render(prefix) + nameStyle.Render(sec.String())
		lines = append(lines, padPanelLine(row, settingsNavWidth, rowBg))
	}

	blank := lipgloss.NewStyle().Background(colorPaneBg)
	for len(lines) < settingsContentLines {
		lines = append(lines, blank.Render(strings.Repeat(" ", settingsNavWidth)))
	}
	return lines
}

// renderSettingsContent dispatches to the focused section's own renderer and
// pads the result to the modal's fixed content height, so switching sections
// never resizes the box.
func (a App) renderSettingsContent(width int) []string {
	var lines []string
	switch a.settings.section {
	case sectionPomodoro:
		lines = a.renderSettingsPomodoro(width)
	case sectionLayout:
		lines = a.renderSettingsLayout(width)
	case sectionTheme:
		lines = a.renderSettingsTheme(width)
	case sectionAbout:
		lines = a.renderSettingsAbout(width)
	}

	blank := lipgloss.NewStyle().Background(colorPaneBg)
	for len(lines) < settingsContentLines {
		lines = append(lines, blank.Render(strings.Repeat(" ", width)))
	}
	if len(lines) > settingsContentLines {
		lines = lines[:settingsContentLines]
	}
	return lines
}

// renderSettingsPomodoro draws the label/value rows the modal has always
// shown, now as the Pomodoro section's content rather than the whole modal.
func (a App) renderSettingsPomodoro(width int) []string {
	m := a.settings
	contentFocused := m.focus == focusSettingsContent

	lines := make([]string, 0, len(settingsFields))
	for i, f := range settingsFields {
		isSel := contentFocused && i == m.pomoCursor

		rowBg := colorPaneBg
		labelStyle := lipgloss.NewStyle().Foreground(colorText).Background(rowBg)
		valueStyle := lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Background(rowBg)
		if isSel {
			rowBg = colorPanel
			labelStyle = lipgloss.NewStyle().Bold(true).Foreground(colorText).Background(rowBg)
			valueStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Background(rowBg)
		}

		label := labelStyle.Render(" " + f.label)
		value := valueStyle.Render(m.valueText(i) + " ")

		pad := width - lipgloss.Width(label) - lipgloss.Width(value)
		if pad < 1 {
			pad = 1
		}
		row := label + lipgloss.NewStyle().Background(rowBg).Render(strings.Repeat(" ", pad)) + value
		lines = append(lines, padPanelLine(row, width, rowBg))
	}
	return lines
}

// renderSettingsLayout lists every layout, marking the cursor's row and
// tagging the committed one "selected" — the distinction matters here
// because moving the cursor already previews a layout without choosing it.
func (a App) renderSettingsLayout(width int) []string {
	m := a.settings
	contentFocused := m.focus == focusSettingsContent

	lines := make([]string, 0, len(allLayouts))
	for i, l := range allLayouts {
		isCursor := contentFocused && i == m.layoutCursor

		rowBg := colorPaneBg
		prefix := "   "
		nameStyle := lipgloss.NewStyle().Foreground(colorText).Background(rowBg)
		if isCursor {
			rowBg = colorPanel
			prefix = " ▸ "
			nameStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Background(rowBg)
		}
		prefixStyle := lipgloss.NewStyle().Foreground(colorAccent).Background(rowBg)

		tag := ""
		if l == m.layoutChosen {
			tag = lipgloss.NewStyle().Foreground(colorGreen).Background(rowBg).Render("selected ")
		}

		left := prefixStyle.Render(prefix) + nameStyle.Render(l.String())
		pad := width - lipgloss.Width(left) - lipgloss.Width(tag)
		if pad < 1 {
			pad = 1
		}
		row := left + lipgloss.NewStyle().Background(rowBg).Render(strings.Repeat(" ", pad)) + tag
		lines = append(lines, padPanelLine(row, width, rowBg))
	}
	return lines
}

// renderSettingsTheme draws the filter input, a rule, and the scrolling list
// of matching themes — the standalone theme picker's body, re-homed inside
// the settings modal's content pane.
func (a App) renderSettingsTheme(width int) []string {
	m := a.settings
	contentFocused := m.focus == focusSettingsContent
	blank := lipgloss.NewStyle().Background(colorPaneBg)

	filter := m.themeFilter
	filter.Width = width - 10
	if filter.Width < 5 {
		filter.Width = 5
	}
	lines := []string{
		padPanelLine(" "+filter.View(), width, colorPaneBg),
		padPanelLine(lipgloss.NewStyle().Foreground(colorBorder).Background(colorPaneBg).Render(strings.Repeat("─", width)), width, colorPaneBg),
	}

	if len(m.themeMatches) == 0 {
		lines = append(lines, padPanelLine(statLabelStyle.Background(colorPaneBg).Render("  No matching themes"), width, colorPaneBg))
		for len(lines) < settingsContentLines {
			lines = append(lines, blank.Render(strings.Repeat(" ", width)))
		}
		return lines
	}

	end := m.themeScroll + settingsThemeVisibleItems
	if end > len(m.themeMatches) {
		end = len(m.themeMatches)
	}
	for i := m.themeScroll; i < end; i++ {
		th := m.themeMatches[i]
		isCursor := contentFocused && i == m.themeCursor

		rowBg := colorPaneBg
		prefix := "   "
		nameStyle := lipgloss.NewStyle().Foreground(colorText).Background(rowBg)
		if isCursor {
			rowBg = colorPanel
			prefix = " ▸ "
			nameStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Background(rowBg)
		}
		prefixStyle := lipgloss.NewStyle().Foreground(colorAccent).Background(rowBg)

		// The source tag doubles as the "selected" marker: a theme is either
		// the committed one (✓) or just a catalog entry (its origin).
		var tag string
		if strings.EqualFold(th.Name, m.themeChosen) {
			tag = lipgloss.NewStyle().Foreground(colorGreen).Background(rowBg).Render("✓ selected ")
		} else {
			tagStyle := lipgloss.NewStyle().Foreground(colorMuted).Background(rowBg)
			switch th.Source {
			case SourceCurated:
				tag = tagStyle.Render("★ curated ")
			case SourceCustom:
				tag = tagStyle.Render("✦ custom ")
			default:
				tag = tagStyle.Render("⚙ tint ")
			}
		}

		nameAvail := width - lipgloss.Width(prefix) - lipgloss.Width(tag) - 1
		if nameAvail < 5 {
			nameAvail = 5
		}
		name := th.Name
		if lipgloss.Width(name) > nameAvail {
			name = fitToWidth(name, nameAvail)
		}

		left := prefixStyle.Render(prefix) + nameStyle.Render(name)
		pad := width - lipgloss.Width(left) - lipgloss.Width(tag)
		if pad < 1 {
			pad = 1
		}
		row := left + lipgloss.NewStyle().Background(rowBg).Render(strings.Repeat(" ", pad)) + tag
		lines = append(lines, padPanelLine(row, width, rowBg))
	}
	return lines
}

// renderSettingsAbout shows the TASKII wordmark and the build version,
// vertically centred in the content pane. The banner falls back to plain
// text on a pane too narrow for the block letters, same as the greeting.
func (a App) renderSettingsAbout(width int) []string {
	blank := lipgloss.NewStyle().Background(colorPaneBg)
	logoStyle := lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Background(colorPaneBg)

	var art []string
	if lipgloss.Width(taskiiBanner[0]) <= width {
		for _, row := range taskiiBanner {
			art = append(art, centerLineWidth(logoStyle.Render(row), width))
		}
	} else {
		art = []string{centerLineWidth(logoStyle.Render("TASKII"), width)}
	}

	version := centerLineWidth(statLabelStyle.Background(colorPaneBg).Render("v"+Version), width)

	// Centre the block vertically: banner rows + a blank + the version line.
	block := len(art) + 2
	top := (settingsContentLines - block) / 2
	if top < 0 {
		top = 0
	}

	lines := make([]string, 0, settingsContentLines)
	for i := 0; i < top; i++ {
		lines = append(lines, blank.Render(strings.Repeat(" ", width)))
	}
	lines = append(lines, art...)
	lines = append(lines, blank.Render(strings.Repeat(" ", width)))
	lines = append(lines, version)
	return lines
}

// centerLineWidth centres an already-styled line in `width` cells against
// the pane background. centerLine does the same but is hardcoded to the
// pane-background style and used by the full-width panes; this variant is
// kept separate so the two callers can't drift apart on padding semantics.
func centerLineWidth(s string, width int) string {
	gap := width - lipgloss.Width(s)
	if gap <= 0 {
		return s
	}
	blank := lipgloss.NewStyle().Background(colorPaneBg)
	left := gap / 2
	return blank.Render(strings.Repeat(" ", left)) + s + blank.Render(strings.Repeat(" ", gap-left))
}
