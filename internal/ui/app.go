package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"terminal-dashboard/internal/model"
	"terminal-dashboard/internal/stats"
)

type focusedPane int

const (
	focusToday focusedPane = iota
	focusOverdue
)

type mode int

const (
	modeNormal mode = iota
	modeAdding
)

const dateFormat = "2006-01-02"

type App struct {
	tasks []model.Task
	now   func() time.Time

	width, height int

	focus focusedPane
	mode  mode

	todaySelected int
	todayScroll   int

	overdueSelected int
	overdueScroll   int

	filterImportant bool
	filterUndone    bool

	pomo pomodoro

	input     textinput.Model
	err       string
	status    string
	noPersist bool

	username string
}

// Options configures NewApp for non-default startup modes.
type Options struct {
	// Mock runs the app against generated sample data: no read from or write
	// to data/tasks.json, so a demo/screenshot run never touches real data.
	Mock bool
}

func NewApp(opts Options) App {
	var tasks []model.Task
	errMsg := ""

	if opts.Mock {
		tasks = mockTasks(time.Now())
	} else {
		var err error
		tasks, err = model.Load()
		if err != nil {
			errMsg = "failed to load tasks: " + err.Error()
			tasks = []model.Task{}
		}
	}

	if !opts.Mock {
		settings, _ := model.LoadSettings()
		if settings.Theme != "" {
			setThemeByName(settings.Theme)
		}
	}

	ti := textinput.New()
	ti.Placeholder = "Title, or end with HH:MM to add it as an appointment"
	ti.CharLimit = 120

	return App{
		tasks:     tasks,
		now:       time.Now,
		focus:     focusToday,
		mode:      modeNormal,
		input:     ti,
		err:       errMsg,
		pomo:      newPomodoro(),
		noPersist: opts.Mock,
		username:  currentUsername(),
	}
}

func (a App) Init() tea.Cmd {
	return pomodoroTick()
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	case pomodoroTickMsg:
		if a.pomo.tick() {
			return a, tea.Batch(pomodoroTick(), notifyPhaseChange(a.pomo.phase))
		}
		return a, pomodoroTick()

	case tea.KeyMsg:
		if a.mode == modeAdding {
			return a.updateAdding(msg)
		}
		return a.updateNormal(msg)
	}

	return a, nil
}

func (a App) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return a, tea.Quit

	case "tab":
		if a.focus == focusToday {
			a.focus = focusOverdue
		} else {
			a.focus = focusToday
		}
		return a, nil

	case "up", "k":
		a.moveSelection(-1)
		return a, nil

	case "down", "j":
		a.moveSelection(1)
		return a, nil

	case "a":
		if a.focus == focusToday {
			a.mode = modeAdding
			a.input.SetValue("")
			a.input.Focus()
			return a, textinput.Blink
		}
		return a, nil

	case " ", "enter":
		a.toggleSelected()
		return a, nil

	case "d":
		a.deleteSelected()
		return a, nil

	case "i":
		a.toggleImportantSelected()
		return a, nil

	case "I":
		a.filterImportant = !a.filterImportant
		a.clampSelections()
		return a, nil

	case "U":
		a.filterUndone = !a.filterUndone
		a.clampSelections()
		return a, nil

	case "p":
		a.pomo.running = !a.pomo.running
		return a, nil

	case "r":
		a.pomo.reset()
		a.pomo.running = false
		return a, nil

	case "n":
		a.pomo.advance()
		a.pomo.running = false
		return a, notifyPhaseChange(a.pomo.phase)

	case "t":
		name := cycleTheme()
		a.status = "Theme: " + name
		if !a.noPersist {
			_ = model.SaveSettings(model.Settings{Theme: name})
		}
		return a, nil
	}

	return a, nil
}

func (a App) updateAdding(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.mode = modeNormal
		a.input.Blur()
		return a, nil
	case "enter":
		a.addTask(a.input.Value())
		a.mode = modeNormal
		a.input.Blur()
		return a, nil
	}

	var cmd tea.Cmd
	a.input, cmd = a.input.Update(msg)
	return a, cmd
}

func (a *App) addTask(raw string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}

	title := raw
	taskTime := ""
	kind := model.KindTask
	fields := strings.Fields(raw)
	if len(fields) > 0 {
		last := fields[len(fields)-1]
		if isTimeLike(last) {
			taskTime = last
			kind = model.KindAppointment
			title = strings.TrimSpace(strings.TrimSuffix(raw, last))
		}
	}
	if title == "" {
		return
	}

	t := model.Task{
		ID:        strconv.FormatInt(a.now().UnixNano(), 36),
		Title:     title,
		Done:      false,
		Kind:      kind,
		Date:      a.now().Format(dateFormat),
		Time:      taskTime,
		CreatedAt: a.now(),
	}

	a.tasks = append(a.tasks, t)
	a.persist()
}

func isTimeLike(s string) bool {
	if len(s) != 5 || s[2] != ':' {
		return false
	}
	h, err1 := strconv.Atoi(s[0:2])
	m, err2 := strconv.Atoi(s[3:5])
	if err1 != nil || err2 != nil {
		return false
	}
	return h >= 0 && h < 24 && m >= 0 && m < 60
}

func (a *App) moveSelection(delta int) {
	list := a.currentList()
	if len(list) == 0 {
		return
	}
	sel := a.currentSelected()
	sel += delta
	if sel < 0 {
		sel = 0
	}
	if sel >= len(list) {
		sel = len(list) - 1
	}
	a.setCurrentSelected(sel)
	a.syncScroll()
}

// syncScroll keeps the current pane's scroll offset such that the selected
// row stays within the visible viewport, scrolling the minimal amount needed.
func (a *App) syncScroll() {
	sel := a.currentSelected()
	scroll := a.currentScroll()
	visible := a.visibleRowsFor(a.focus)

	if sel < scroll {
		scroll = sel
	} else if sel >= scroll+visible {
		scroll = sel - visible + 1
	}
	if scroll < 0 {
		scroll = 0
	}
	a.setCurrentScroll(scroll)
}

func (a *App) clampSelections() {
	todayLen := len(a.todayTasks())
	if a.todaySelected >= todayLen {
		a.todaySelected = todayLen - 1
	}
	if a.todaySelected < 0 {
		a.todaySelected = 0
	}
	overdueLen := len(a.overdueTasks())
	if a.overdueSelected >= overdueLen {
		a.overdueSelected = overdueLen - 1
	}
	if a.overdueSelected < 0 {
		a.overdueSelected = 0
	}
	a.syncScroll()
}

func (a *App) toggleSelected() {
	list := a.currentList()
	sel := a.currentSelected()
	if sel < 0 || sel >= len(list) {
		return
	}
	id := list[sel].ID
	for i := range a.tasks {
		if a.tasks[i].ID == id {
			a.tasks[i].Done = !a.tasks[i].Done
			if a.tasks[i].Done {
				now := a.now()
				a.tasks[i].DoneAt = &now
			} else {
				a.tasks[i].DoneAt = nil
			}
			break
		}
	}
	a.persist()
}

func (a *App) toggleImportantSelected() {
	list := a.currentList()
	sel := a.currentSelected()
	if sel < 0 || sel >= len(list) {
		return
	}
	id := list[sel].ID
	for i := range a.tasks {
		if a.tasks[i].ID == id {
			a.tasks[i].Important = !a.tasks[i].Important
			break
		}
	}
	a.persist()
}

func (a *App) deleteSelected() {
	list := a.currentList()
	sel := a.currentSelected()
	if sel < 0 || sel >= len(list) {
		return
	}
	id := list[sel].ID
	filtered := a.tasks[:0]
	for _, t := range a.tasks {
		if t.ID != id {
			filtered = append(filtered, t)
		}
	}
	a.tasks = filtered

	newLen := len(a.currentList())
	if sel >= newLen {
		sel = newLen - 1
	}
	a.setCurrentSelected(sel)
	a.syncScroll()
	a.persist()
}

func (a *App) persist() {
	if a.noPersist {
		return
	}
	if err := model.Save(a.tasks); err != nil {
		a.err = "failed to save tasks: " + err.Error()
	}
}

func (a App) currentList() []model.Task {
	if a.focus == focusToday {
		return a.todayTasks()
	}
	return a.overdueTasks()
}

func (a App) currentSelected() int {
	if a.focus == focusToday {
		return a.todaySelected
	}
	return a.overdueSelected
}

func (a *App) setCurrentSelected(v int) {
	if a.focus == focusToday {
		a.todaySelected = v
	} else {
		a.overdueSelected = v
	}
}

func (a App) currentScroll() int {
	if a.focus == focusToday {
		return a.todayScroll
	}
	return a.overdueScroll
}

func (a *App) setCurrentScroll(v int) {
	if a.focus == focusToday {
		a.todayScroll = v
	} else {
		a.overdueScroll = v
	}
}

func (a App) applyFilters(tasks []model.Task) []model.Task {
	if !a.filterImportant && !a.filterUndone {
		return tasks
	}
	out := make([]model.Task, 0, len(tasks))
	for _, t := range tasks {
		if a.filterImportant && !t.Important {
			continue
		}
		if a.filterUndone && t.Done {
			continue
		}
		out = append(out, t)
	}
	return out
}

func (a App) todayTasks() []model.Task {
	today := a.now().Format(dateFormat)
	var out []model.Task
	for _, t := range a.tasks {
		if t.Date == today {
			out = append(out, t)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Time == out[j].Time {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		if out[i].Time == "" {
			return false
		}
		if out[j].Time == "" {
			return true
		}
		return out[i].Time < out[j].Time
	})
	return a.applyFilters(out)
}

func (a App) overdueTasks() []model.Task {
	today := a.now().Format(dateFormat)
	var out []model.Task
	for _, t := range a.tasks {
		if t.Date < today && !t.Done {
			out = append(out, t)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Date < out[j].Date
	})
	return a.applyFilters(out)
}

func (a App) helpGroups() []helpGroup {
	if a.mode == modeAdding {
		return []helpGroup{
			{"", []helpKey{
				{"enter", "confirm"}, {"esc", "cancel"},
				{"", "end with HH:MM to add it as an appointment"},
			}},
		}
	}
	return []helpGroup{
		{"Task", []helpKey{
			{"a", "add"}, {"space/enter", "toggle"}, {"d", "delete"}, {"i", "important"},
		}},
		{"View", []helpKey{
			{"tab", "switch pane"}, {"↑/↓ j/k", "navigate"}, {"I/U", "filters"},
		}},
		{"Pomodoro", []helpKey{
			{"p", "pause/resume"}, {"r", "reset"}, {"n", "skip"},
		}},
		{"App", []helpKey{
			{"t", "theme"}, {"q", "quit"},
		}},
	}
}

// chromeLines returns how many lines of fixed UI (error line, help bar)
// surround the body panes, so both visibleRowsFor and View agree on how much
// height the panes actually get. The top header/date bar was removed (the
// date now lives in the greeting pane, which is part of the body, not fixed
// chrome). The help bar's line count varies with terminal width (it wraps to
// a second line when narrow), so a flat constant here would let the last
// help line get clipped off-screen or mis-budget the body panes' height by
// one row whenever wrapping kicks in.
func (a App) chromeLines() int {
	const errLine = 1
	help := lipgloss.Height(renderHelpBar(a.helpGroups(), a.width))
	return errLine + help
}

// visibleRowsFor computes how many task rows fit in the given pane's content
// height. Mirrors the height math used when actually laying out the panes in
// View(), so scroll math and rendering never disagree about viewport size.
func (a App) visibleRowsFor(focus focusedPane) int {
	bodyHeight := a.height - a.chromeLines()
	if bodyHeight < 10 {
		bodyHeight = 10
	}
	todayHeight := bodyHeight / 2
	overdueHeight := bodyHeight - todayHeight

	paneHeight := todayHeight
	if focus == focusOverdue {
		paneHeight = overdueHeight
	}
	contentHeight := paneHeight - 2 // border top+bottom
	titleLines := 1
	// renderTaskList always emits a scroll-indicator line (blank when there's
	// nothing to scroll), so its line is reserved unconditionally here too —
	// otherwise the pane grows by a line whenever "N more" starts appearing.
	indicatorLines := 1
	rows := contentHeight - titleLines - indicatorLines
	if focus == focusToday && a.mode == modeAdding {
		rows-- // reserve a line for the inline add-task input
	}
	if rows < 1 {
		rows = 1
	}
	return rows
}

func filterLabel(important, undone bool) string {
	var tags []string
	if important {
		tags = append(tags, "Important")
	}
	if undone {
		tags = append(tags, "Undone")
	}
	if len(tags) == 0 {
		return ""
	}
	return " [" + strings.Join(tags, ", ") + "]"
}

func (a App) View() string {
	if a.width == 0 {
		return "loading..."
	}

	helpLine := renderHelpBar(a.helpGroups(), a.width)

	leftWidth := a.width * 3 / 5
	rightWidth := a.width - leftWidth - 1
	if rightWidth < 20 {
		rightWidth = 20
	}

	bodyHeight := a.height - a.chromeLines()
	if bodyHeight < 10 {
		bodyHeight = 10
	}
	todayHeight := bodyHeight / 2
	overdueHeight := bodyHeight - todayHeight

	filters := filterLabel(a.filterImportant, a.filterUndone)

	today := a.todayTasks()
	todayVisible := a.visibleRowsFor(focusToday)
	todayBody := renderTaskList(today, a.todaySelected, a.todayScroll, todayVisible, a.focus == focusToday, false, leftWidth-4)
	if a.mode == modeAdding {
		// The textinput sizes itself from its placeholder unless given an
		// explicit width, which overflowed narrow panes and dragged the
		// whole layout wider than the terminal. Fit it to what's actually
		// left after the "+ " prompt. View has a value receiver, so this
		// only touches the local copy used for this frame.
		inputWidth := leftWidth - 4 - lipgloss.Width(inputPromptStyle.Render("+ "))
		if inputWidth < 1 {
			inputWidth = 1
		}
		a.input.Width = inputWidth
		// Set here rather than once in NewApp so these follow theme changes.
		a.input.TextStyle = lipgloss.NewStyle().Foreground(colorText).Background(colorPaneBg)
		a.input.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorMuted).Background(colorPaneBg)
		a.input.PromptStyle = lipgloss.NewStyle().Foreground(colorAccent).Background(colorPaneBg)
		a.input.Cursor.Style = lipgloss.NewStyle().Foreground(colorText).Background(colorPaneBg)

		// The widget pads out to its own Width with unstyled spaces, which
		// leaves an unbackgrounded gap. Trim that trailing filler off and
		// re-pad it ourselves with the pane background.
		inputLine := inputPromptStyle.Render("+ ") + strings.TrimRight(a.input.View(), " ")
		if pad := (leftWidth - 4) - lipgloss.Width(inputLine); pad > 0 {
			inputLine += lipgloss.NewStyle().Background(colorPaneBg).Render(strings.Repeat(" ", pad))
		}
		todayBody += "\n" + inputLine
	}
	todayPane := renderPane(fmt.Sprintf("Today (%d)%s", len(today), filters), todayBody, a.focus == focusToday, leftWidth, todayHeight)

	overdue := a.overdueTasks()
	overdueVisible := a.visibleRowsFor(focusOverdue)
	overdueBody := renderTaskList(overdue, a.overdueSelected, a.overdueScroll, overdueVisible, a.focus == focusOverdue, true, leftWidth-4)
	overduePane := renderPane(fmt.Sprintf("Overdue (%d)%s", len(overdue), filters), overdueBody, a.focus == focusOverdue, leftWidth, overdueHeight)

	left := lipgloss.JoinVertical(lipgloss.Left, todayPane, overduePane)

	// Pomodoro's content is a fixed 7 lines regardless of pane size, so give
	// it just enough room (+ border) and let Reports use the rest — a 3:2
	// split starved Reports of the height its progress bars + heatmap need
	// on realistic terminal sizes, hiding the heatmap almost everywhere.
	const pomoContentLines = 7
	pomoHeight := pomoContentLines + 2
	if pomoHeight > bodyHeight/2 {
		pomoHeight = bodyHeight / 2
	}
	// Greeting is likewise a small fixed-content pane (logo/date/greeting),
	// carved out of the same right-column budget before Reports claims the
	// remainder, same pattern as Pomodoro above.
	greetHeight := greetingContentLines + 2
	if greetHeight > bodyHeight/3 {
		greetHeight = bodyHeight / 3
	}
	reportsHeight := bodyHeight - pomoHeight - greetHeight

	greetBody := renderGreeting(a.now(), a.username, rightWidth-4, greetHeight-2)
	greetPane := renderPane("", greetBody, false, rightWidth, greetHeight)

	report := stats.Compute(a.tasks, a.now())
	reportsBody := renderReports(report, rightWidth-4, reportsHeight-2)
	reportsPane := renderPane("", reportsBody, false, rightWidth, reportsHeight)

	pomoBody := renderPomodoro(a.pomo, rightWidth-4)
	pomoPane := renderPane("", pomoBody, false, rightWidth, pomoHeight)

	right := lipgloss.JoinVertical(lipgloss.Left, greetPane, reportsPane, pomoPane)

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	pageBg := lipgloss.NewStyle().Background(colorBg)

	errLine := ""
	if a.err != "" {
		errLine = lipgloss.NewStyle().Foreground(colorDanger).Background(colorBg).Render(a.err)
	} else if a.status != "" {
		errLine = lipgloss.NewStyle().Foreground(colorMuted).Background(colorBg).Render(a.status)
	}

	// Every line from every source below is padded to a.width with its own
	// styled (colorBg) trailing spaces BEFORE joining — not after. Two
	// distinct bugs made that necessary: (1) any line already carrying its
	// own ANSI styling can't have background backfilled past its own
	// embedded reset by a later outer Render() call (documented repeatedly
	// elsewhere in this codebase); and (2) lipgloss.JoinVertical pads
	// shorter lines up to the widest line's width using its OWN plain,
	// unstyled spaces — so even a line that already had its trailing
	// padding correctly backgrounded gets MORE, unstyled padding appended
	// on top by JoinVertical itself if a sibling line (here, body) is
	// wider. Pre-padding every line to the same final width so none of
	// them differ removes JoinVertical's padding from the equation
	// entirely — it never has anything left to pad.
	padLines := func(s string) string {
		lines := strings.Split(s, "\n")
		for i, l := range lines {
			if pad := a.width - lipgloss.Width(l); pad > 0 {
				lines[i] = l + pageBg.Render(strings.Repeat(" ", pad))
			}
		}
		return strings.Join(lines, "\n")
	}

	full := lipgloss.JoinVertical(lipgloss.Left,
		padLines(body),
		padLines(errLine),
		padLines(helpLine),
	)
	return full
}
