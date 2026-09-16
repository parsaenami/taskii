package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"taskii/internal/model"
	"taskii/internal/stats"
)

type focusedPane int

const (
	focusToday focusedPane = iota
	focusOverdue
	focusNotes
	focusReports

	focusCount = 4
)

type mode int

const (
	modeNormal mode = iota
	modeAdding
	modeConfirmDelete
	modeNoteEditing
	modeConfirmClearNotes
	modeSettings
)

// dateFormat is an alias for the model package's canonical layout, kept so
// the many call sites in this package stay short.
const dateFormat = model.DateFormat

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

	notes         []model.Note
	notesSelected int
	notesScroll   int
	noteInput     textarea.Model
	noteEditIndex int // -1 when adding, otherwise the note being edited
	// notesExpanded blows the Notes board up to the whole body area, hiding
	// every other pane. Deliberately NOT persisted: it's a transient working
	// state, not a preference like theme or layout.
	notesExpanded bool

	// reportChart is which chart the Reports pane shows; navigable with the
	// arrow keys while that pane is focused.
	reportChart reportChart

	// Simple mode: one pane, one merged list. simpleNoteInput selects which
	// input tab is active (tab toggles between adding a task and a note).
	simple         bool
	simpleSelected int
	simpleScroll   int
	simpleNoteMode bool

	input textinput.Model
	// taskEditID is the task the inline input is editing, or "" when the
	// input is adding a new one. Held as an ID rather than a list index
	// because both task lists are sorted and re-derived every frame — an
	// index would point at a different row the moment a title changes.
	taskEditID string
	err        string
	status     string
	noPersist  bool

	username string
	layout   layout

	settings settingsModal
}

// Options configures NewApp for non-default startup modes.
type Options struct {
	// Mock runs the app against generated sample data: no read from or write
	// to data/tasks.json, so a demo/screenshot run never touches real data.
	Mock bool
	// Simple renders a single full-screen pane: the greeting in a fixed left
	// column, and one list merging today's tasks, overdue tasks and notes in
	// creation order beside it.
	Simple bool
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

	lay := layoutTasksLeft
	pomo := newPomodoro()
	if !opts.Mock {
		settings, _ := model.LoadSettings()
		if settings.Theme != "" {
			setThemeByName(settings.Theme)
		}
		if settings.Layout != "" {
			lay = layoutByName(settings.Layout)
		}
		if settings.PomodoroFocusMinutes > 0 {
			pomo.workMinutes = settings.PomodoroFocusMinutes
		}
		if settings.PomodoroShortBreakMinutes > 0 {
			pomo.shortBreakMinutes = settings.PomodoroShortBreakMinutes
		}
		if settings.PomodoroLongBreakMinutes > 0 {
			pomo.longBreakMinutes = settings.PomodoroLongBreakMinutes
		}
		if settings.PomodoroLongBreakEvery > 0 {
			pomo.longBreakEvery = settings.PomodoroLongBreakEvery
		}
		pomo.autoStartNext = settings.PomodoroAutoStartNext
		pomo.remaining = pomo.phaseDuration()
	}

	var notes []model.Note
	if opts.Mock {
		notes = mockNotes()
	} else if n, err := model.LoadNotes(); err == nil {
		notes = n
	}

	ti := textinput.New()
	ti.Placeholder = "Title  ·  #tag  ·  HH:MM[-HH:MM]"
	ti.CharLimit = 120

	ta := textarea.New()
	ta.Placeholder = "Note — ctrl+j or opt+enter for a new line"
	ta.ShowLineNumbers = false
	// No CharLimit: notes are explicitly unbounded in length.
	ta.CharLimit = 0
	// The textarea binds enter to "insert newline" by default; here enter
	// SAVES and shift/alt+enter inserts the newline, so that binding is
	// cleared and handled in updateNoteEditing instead.
	ta.KeyMap.InsertNewline.SetEnabled(false)

	return App{
		tasks:         tasks,
		now:           time.Now,
		focus:         focusToday,
		mode:          modeNormal,
		input:         ti,
		notes:         notes,
		noteInput:     ta,
		noteEditIndex: -1,
		simple:        opts.Simple,
		err:           errMsg,
		pomo:          pomo,
		noPersist:     opts.Mock,
		username:      currentUsername(),
		layout:        lay,
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
		switch a.mode {
		case modeAdding:
			return a.updateAdding(msg)
		case modeConfirmDelete:
			return a.updateConfirmDelete(msg)
		case modeNoteEditing:
			return a.updateNoteEditing(msg)
		case modeConfirmClearNotes:
			return a.updateConfirmClearNotes(msg)
		case modeSettings:
			return a.updateSettings(msg)
		}
		return a.updateNormal(msg)
	}

	return a, nil
}

// expandedAllowedKeys are the only bindings that stay live while the Notes
// board is expanded: the notes keys themselves plus the app-wide ones. Every
// other pane is off-screen, so its keys would act invisibly — pressing `p`
// would start a Pomodoro nobody can see.
var expandedAllowedKeys = map[string]bool{
	// Notes.
	"a": true, "enter": true, "d": true, "C": true, "e": true,
	"up": true, "k": true, "down": true, "j": true,
	// App-wide.
	"q": true, "ctrl+c": true, "S": true,
}

// updateSimple is the whole key map for --simple: one list, one selection,
// and tab choosing which kind of item `a` will add. The other panes don't
// exist here, so their bindings (pomodoro, filters, focus switching, layout)
// are simply absent rather than being gated off.
func (a App) updateSimple(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	entries := a.simpleEntries()

	switch msg.String() {
	case "q", "ctrl+c":
		return a, tea.Quit

	case "tab":
		a.simpleNoteMode = !a.simpleNoteMode
		return a, nil

	case "up", "k":
		a.moveSimpleSelection(-1, entries)
		return a, nil

	case "down", "j":
		a.moveSimpleSelection(1, entries)
		return a, nil

	case "a":
		if a.simpleNoteMode {
			return a.startNoteEdit(-1)
		}
		a.mode = modeAdding
		a.taskEditID = ""
		a.input.SetValue("")
		a.input.Focus()
		return a, textinput.Blink

	case " ", "enter":
		if a.simpleSelected < 0 || a.simpleSelected >= len(entries) {
			return a, nil
		}
		e := entries[a.simpleSelected]
		if e.isNote {
			if msg.String() == "enter" {
				return a.startNoteEdit(e.noteIndex)
			}
			return a, nil
		}
		// Enter edits, space toggles — same split as the full layout's task
		// lists, so the two modes don't teach different habits. Overdue rows
		// ARE editable here, unlike the full layout: simple mode has one
		// merged list and one input line, so there's no second pane for the
		// cursor to land in by mistake.
		if msg.String() == "enter" {
			return a.startTaskEdit(&e.task)
		}
		a.toggleTaskByID(e.task.ID)
		return a, nil

	case "d":
		if a.simpleSelected >= 0 && a.simpleSelected < len(entries) {
			a.mode = modeConfirmDelete
		}
		return a, nil

	case "i":
		if a.simpleSelected >= 0 && a.simpleSelected < len(entries) {
			if e := entries[a.simpleSelected]; !e.isNote {
				a.toggleImportantByID(e.task.ID)
			}
		}
		return a, nil

	case "S":
		return a, a.openSettings()
	}
	return a, nil
}

func (a *App) moveSimpleSelection(delta int, entries []simpleEntry) {
	if len(entries) == 0 {
		return
	}
	sel := a.simpleSelected + delta
	if sel < 0 {
		sel = 0
	}
	if sel >= len(entries) {
		sel = len(entries) - 1
	}
	a.simpleSelected = sel
	a.syncSimpleScroll(entries)
}

// syncSimpleScroll keeps the selected entry's display rows in view. Same
// note-vs-line distinction as the notes board: selection counts entries,
// the scroll offset counts display lines.
func (a *App) syncSimpleScroll(entries []simpleEntry) {
	visible := a.simpleVisibleRows()
	all := a.simpleLines(entries, a.simpleListWidth())
	first, last := -1, -1
	for i, l := range all {
		if l.entryIndex == a.simpleSelected {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 0 {
		return
	}
	scroll := a.simpleScroll
	if first < scroll {
		scroll = first
	} else if last >= scroll+visible {
		scroll = last - visible + 1
		if scroll > first {
			scroll = first
		}
	}
	if scroll < 0 {
		scroll = 0
	}
	a.simpleScroll = scroll
}

func (a *App) toggleTaskByID(id string) {
	for i := range a.tasks {
		if a.tasks[i].ID == id {
			a.tasks[i].Done = !a.tasks[i].Done
			if a.tasks[i].Done {
				t := a.now()
				a.tasks[i].DoneAt = &t
			} else {
				a.tasks[i].DoneAt = nil
			}
			a.persist()
			return
		}
	}
}

func (a *App) toggleImportantByID(id string) {
	for i := range a.tasks {
		if a.tasks[i].ID == id {
			a.tasks[i].Important = !a.tasks[i].Important
			a.persist()
			return
		}
	}
}

func (a *App) deleteSimpleSelected() {
	entries := a.simpleEntries()
	if a.simpleSelected < 0 || a.simpleSelected >= len(entries) {
		return
	}
	e := entries[a.simpleSelected]
	if e.isNote {
		if e.noteIndex >= 0 && e.noteIndex < len(a.notes) {
			a.notes = append(a.notes[:e.noteIndex], a.notes[e.noteIndex+1:]...)
			a.persistNotes()
		}
	} else {
		for i := range a.tasks {
			if a.tasks[i].ID == e.task.ID {
				a.tasks = append(a.tasks[:i], a.tasks[i+1:]...)
				a.persist()
				break
			}
		}
	}
	if a.simpleSelected >= len(a.simpleEntries()) {
		a.simpleSelected = len(a.simpleEntries()) - 1
	}
	if a.simpleSelected < 0 {
		a.simpleSelected = 0
	}
}

func (a App) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.simple {
		return a.updateSimple(msg)
	}
	if a.notesExpanded && !expandedAllowedKeys[msg.String()] {
		return a, nil
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return a, tea.Quit

	case "tab":
		a.focus = (a.focus + 1) % focusCount
		a.clampSelections()
		return a, nil

	case "shift+tab":
		a.focus = (a.focus + focusCount - 1) % focusCount
		a.clampSelections()
		return a, nil

	case "up", "k", "left", "h":
		// On the Reports pane the navigation keys move between charts
		// rather than between rows — that pane has no list.
		if a.focus == focusReports {
			a.reportChart = a.reportChart.prev()
			return a, nil
		}
		a.moveSelection(-1)
		return a, nil

	case "down", "j", "right", "l":
		if a.focus == focusReports {
			a.reportChart = a.reportChart.next()
			return a, nil
		}
		a.moveSelection(1)
		return a, nil

	case "a":
		switch a.focus {
		case focusReports:
			// No items on this pane.
			return a, nil
		case focusToday:
			a.mode = modeAdding
			a.taskEditID = ""
			a.input.SetValue("")
			a.input.Focus()
			return a, textinput.Blink
		case focusNotes:
			return a.startNoteEdit(-1)
		}
		return a, nil

	case " ", "enter":
		if a.focus == focusReports {
			return a, nil
		}
		// Enter opens the selected item for editing; space toggles a task
		// done. Editing is Today-only: the inline input renders inside the
		// Today pane (the same line modeAdding uses), so an edit started
		// from Overdue would drop the cursor into a different box than the
		// row being edited. Carry the task forward with space first, then
		// edit it in Today.
		if a.focus == focusNotes {
			if msg.String() == "enter" && len(a.notes) > 0 {
				return a.startNoteEdit(a.notesSelected)
			}
			return a, nil
		}
		if msg.String() == "enter" {
			if a.focus != focusToday {
				return a, nil
			}
			return a.startTaskEdit(a.selectedTask())
		}
		// Space means "done" in Today, but "carry this forward to today" in
		// Overdue — there's nothing useful about ticking off a task on a day
		// that has already passed, whereas pulling it onto today's plate is
		// the action that pane is actually for.
		if a.focus == focusOverdue {
			a.migrateSelected()
			return a, nil
		}
		a.toggleSelected()
		return a, nil

	case "d":
		// Deletion is irreversible (there's no undo), so it takes a second
		// keystroke. Only enter the mode if something is actually selected,
		// otherwise the prompt would ask about nothing.
		if a.focus == focusReports {
			return a, nil
		}
		if a.focus == focusNotes {
			if len(a.notes) > 0 {
				a.mode = modeConfirmDelete
			}
			return a, nil
		}
		if a.selectedTask() != nil {
			a.mode = modeConfirmDelete
		}
		return a, nil

	case "C":
		// Clear the whole board. Capitalised and confirmed, since it discards
		// everything at once.
		if a.focus == focusNotes && len(a.notes) > 0 {
			a.mode = modeConfirmClearNotes
		}
		return a, nil

	case "e":
		// Notes-only: expand the board to fill the screen, and back.
		if a.focus == focusNotes {
			a.notesExpanded = !a.notesExpanded
			// The viewport changes size, so the scroll offset may now leave
			// the selected note off-screen.
			a.syncScroll()
		}
		return a, nil

	case "i":
		// Task-list only: neither the notes board nor Reports has items
		// with a notion of importance.
		if a.focus != focusNotes && a.focus != focusReports {
			a.toggleImportantSelected()
		}
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

	case "S":
		return a, a.openSettings()
	}

	return a, nil
}

// startTaskEdit opens the inline input pre-filled with the selected task's
// title, reusing modeAdding's widget and render path rather than splicing a
// live input into the middle of an already-styled list row.
func (a App) startTaskEdit(t *model.Task) (tea.Model, tea.Cmd) {
	if t == nil {
		return a, nil
	}
	a.mode = modeAdding
	a.taskEditID = t.ID
	a.input.SetValue(taskInputText(*t, a.now()))
	a.input.Focus()
	a.input.CursorEnd()
	return a, textinput.Blink
}

// taskInputText renders a task back into the annotation syntax parseTaskInput
// reads, so opening the editor and pressing enter unchanged is a no-op. Every
// annotation has to appear here: one left out would be silently dropped by
// the re-parse on save, since applyTaskEdit assigns the whole set.
// The due date is re-derived from `now` rather than echoed back verbatim, so
// the editor shows what the task means TODAY: a "!3d" entered last week opens
// as "!1d", and saving it unchanged keeps the same absolute deadline instead
// of silently pushing it three days out.
func taskInputText(t model.Task, now time.Time) string {
	// Tags need no reconstruction: they were never removed from the title,
	// so they're already in it, in the position they were typed.
	val := t.Title
	// The end-anchored annotations follow, since that's the only position
	// parseTaskInput recognises them in.
	if t.Time != "" {
		val += " " + t.Time
		if t.EndTime != "" {
			val += "-" + t.EndTime
		}
	}
	if days, ok := t.DaysUntilDue(now); dueDatesEnabled && ok {
		if days < 0 {
			// Echoed in the same "‼Nd" form the row shows, which
			// parseDueField accepts — so an overdue task saved unchanged
			// keeps its actual deadline instead of being rescheduled to
			// today.
			val += fmt.Sprintf(" ‼%dd", -days)
		} else {
			val += fmt.Sprintf(" !%dd", days)
		}
	}
	return val
}

func (a App) updateAdding(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.mode = modeNormal
		a.taskEditID = ""
		a.input.Blur()
		return a, nil
	case "enter":
		if a.taskEditID != "" {
			a.applyTaskEdit(a.input.Value())
			a.mode = modeNormal
			a.taskEditID = ""
			a.input.Blur()
			return a, nil
		}
		a.addTask(a.input.Value())
		a.input.SetValue("")
		return a, nil
	}

	// Bound the value to the visible field so long titles scroll horizontally
	// instead of overflowing the pane. This has to happen before Update: the
	// widget recomputes its scroll offsets from Width inside handleOverflow,
	// which View then just reads.
	a.input.Width = a.inputFieldWidth()

	var cmd tea.Cmd
	a.input, cmd = a.input.Update(msg)
	return a, cmd
}

// confirmHint is the "[y] yes / [any other key] cancel" suffix on a
// confirmation prompt. It reuses the help bar's own key/label styles so the
// prompt matches the rest of the app's key hints — the confirmation lives on
// the status line ONLY, and the help bar is left showing the normal bindings,
// so the same question isn't asked twice in two different styles.
func confirmHint() string {
	return helpLabelStyle.Render("   ") +
		helpKeyStyle.Render("[y]") + helpLabelStyle.Render(" yes") +
		helpLabelStyle.Render("   ") +
		helpKeyStyle.Render("[any other key]") + helpLabelStyle.Render(" cancel")
}

// noteEditorHeight is how many rows the inline note editor occupies. Kept
// small so the board stays visible while typing; the textarea scrolls
// internally for longer notes.
func (a App) noteEditorHeight() int {
	const h = 3
	return h
}

// renderNotesPane draws the Notes board, including the inline editor when one
// is open. Returns "" when the layout gave Notes no room, so callers can omit
// it entirely rather than render a zero-height box.
func (a App) renderNotesPane(g geometry) string {
	if g.notesHeight < 3 || g.notesWidth < 6 {
		return ""
	}
	contentWidth := g.notesWidth - 4
	if contentWidth < 1 {
		contentWidth = 1
	}

	body := renderNotes(a.notes, a.notesSelected, a.notesScroll,
		a.visibleRowsFor(focusNotes), a.focus == focusNotes,
		a.mode == modeNoteEditing, contentWidth)

	if a.mode == modeNoteEditing {
		body += "\n" + a.renderNoteEditor(contentWidth)
	}

	title := fmt.Sprintf("Notes (%d)", len(a.notes))
	return renderPane(title, body, a.focus == focusNotes, g.notesWidth, g.notesHeight)
}

// renderNoteEditor draws the multi-line note editor at the given width,
// returning exactly noteEditorHeight() lines. Shared by the Notes pane and
// simple mode.
//
// The widget's own styles go through Inline(true), which strips backgrounds,
// and it pads its lines with unstyled spaces — so its output can't be made to
// carry a background from the outside by styling alone. Strip the ANSI it
// produced and re-render each line as background-carrying spans instead. The
// editor is plain text (no per-token colors to preserve), so nothing is lost.
func (a App) renderNoteEditor(width int) string {
	bg := colorPaneBg
	if a.simple {
		// Simple mode has no panes, so the editor sits on the page surface.
		bg = colorBg
	}

	// Value receiver: these only affect the frame being rendered. Set here
	// rather than at construction so they follow theme changes.
	a.noteInput.SetWidth(width)
	a.noteInput.SetHeight(a.noteEditorHeight())
	a.noteInput.FocusedStyle.Base = lipgloss.NewStyle().Background(bg)
	a.noteInput.FocusedStyle.Text = lipgloss.NewStyle().Foreground(colorText).Background(bg)
	a.noteInput.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(colorMuted).Background(bg)
	a.noteInput.FocusedStyle.CursorLine = lipgloss.NewStyle().Background(bg)
	a.noteInput.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(colorAccent).Background(bg)

	editStyle := lipgloss.NewStyle().Foreground(colorText).Background(bg)
	promptStyle := lipgloss.NewStyle().Foreground(colorAccent).Background(bg)
	cursorStyle := lipgloss.NewStyle().Foreground(bg).Background(colorAccent)

	// Stripping the widget's ANSI also removes the cursor's inverse video, so
	// the caret is drawn from the model's own row/column instead.
	curRow, curCol := a.noteInput.Line(), a.noteInput.LineInfo().ColumnOffset

	var out []string
	for row, l := range strings.Split(a.noteInput.View(), "\n") {
		plain := strings.TrimRight(ansiRe.ReplaceAllString(l, ""), " ")
		rest, hasPrompt := strings.CutPrefix(plain, a.noteInput.Prompt)
		if !hasPrompt {
			rest = plain
		}

		var text string
		if row == curRow {
			r := []rune(rest)
			for len(r) <= curCol {
				r = append(r, ' ')
			}
			text = editStyle.Render(string(r[:curCol])) +
				cursorStyle.Render(string(r[curCol])) +
				editStyle.Render(string(r[curCol+1:]))
		} else {
			text = editStyle.Render(rest)
		}

		line := text
		if hasPrompt {
			line = promptStyle.Render(a.noteInput.Prompt) + text
		}
		if pad := width - lipgloss.Width(line); pad > 0 {
			line += lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", pad))
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// startNoteEdit opens the note editor. index -1 adds a new note, otherwise
// the existing note is loaded for editing.
func (a App) startNoteEdit(index int) (tea.Model, tea.Cmd) {
	a.mode = modeNoteEditing
	a.noteEditIndex = index
	if index >= 0 && index < len(a.notes) {
		a.noteInput.SetValue(a.notes[index].Body)
	} else {
		a.noteInput.SetValue("")
	}
	a.noteInput.Focus()
	a.noteInput.CursorEnd()
	return a, textarea.Blink
}

// updateNoteEditing handles the multi-line note editor. Enter saves;
// shift+enter and alt+enter insert a newline. Shift+enter is what the user
// asked for, but many terminals send an identical sequence for enter and
// shift+enter (they're indistinguishable without kitty-protocol or similar
// support), so alt+enter is bound alongside it as a portable fallback.
func (a App) updateNoteEditing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.mode = modeNormal
		a.noteInput.Blur()
		return a, nil

	// Newline bindings, and what each is actually worth:
	//
	//   ctrl+j    — a real control character (0x0A). Universal.
	//   ctrl+n    — mnemonic alias ("new line"). Universal, same reason.
	//   alt+enter — ESC-prefixed; this is what macOS Option+Enter sends when
	//               the terminal treats Option as Meta. Widely supported.
	//   ctrl+j    — also what Option+Enter sends when it does NOT. (Both
	//               encodings verified with a key probe.)
	//   shift+enter — bound, but only reaches the app in terminals speaking
	//               the kitty keyboard protocol (kitty, WezTerm, Ghostty,
	//               foot) or iTerm2 with a manual mapping. Legacy terminal
	//               encoding has no representation for modifier+Enter, so
	//               Terminal.app sends bytes identical to plain Enter and NO
	//               binding can separate them. Kept for terminals that can
	//               send it; deliberately not advertised as the primary
	//               route, since a hint that silently does nothing is worse
	//               than no hint.
	case "shift+enter", "alt+enter", "ctrl+j", "ctrl+n":
		a.noteInput.InsertString("\n")
		return a, nil

	case "enter":
		adding := a.noteEditIndex < 0
		a.saveNote(a.noteInput.Value())
		if adding {
			// Stay in the editor for the next note, the same way modeAdding
			// keeps the task input open — a board is usually filled several
			// bullets at a time, and esc is the deliberate way out.
			a.noteEditIndex = -1
			a.noteInput.SetValue("")
			return a, nil
		}
		a.mode = modeNormal
		a.noteInput.Blur()
		return a, nil
	}

	var cmd tea.Cmd
	a.noteInput, cmd = a.noteInput.Update(msg)
	return a, cmd
}

// updateConfirmClearNotes handles the whole-board clear prompt. Same
// anything-but-yes-cancels rule as the delete prompt.
func (a App) updateConfirmClearNotes(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	a.mode = modeNormal
	switch msg.String() {
	case "y", "Y", "enter":
		a.notes = nil
		a.notesSelected = 0
		a.notesScroll = 0
		a.persistNotes()
		a.status = "Notes cleared"
	default:
		a.status = "Clear cancelled"
	}
	return a, nil
}

// saveNote commits the editor's contents, either appending a new note or
// replacing the one being edited. An empty body deletes rather than storing a
// blank bullet.
func (a *App) saveNote(body string) {
	body = strings.TrimRight(body, "\n \t")
	editing := a.noteEditIndex
	a.noteEditIndex = -1

	if strings.TrimSpace(body) == "" {
		if editing >= 0 && editing < len(a.notes) {
			a.notes = append(a.notes[:editing], a.notes[editing+1:]...)
			a.clampSelections()
			a.persistNotes()
		}
		return
	}

	if editing >= 0 && editing < len(a.notes) {
		a.notes[editing].Body = body
	} else {
		a.notes = append(a.notes, model.Note{
			ID:        strconv.FormatInt(a.now().UnixNano(), 36),
			Body:      body,
			CreatedAt: a.now(),
		})
		a.notesSelected = len(a.notes) - 1
	}
	if editing >= 0 && editing < len(a.notes) {
		a.notesSelected = editing
	}
	a.persistNotes()
	// Leave the cursor on the note just written and scroll it into view, so
	// the ">" indicator lands on it rather than on wherever it was before.
	a.selectNote(a.notesSelected)
}

// selectNote puts the cursor on the given note index in whichever list is
// showing — the merged simple-mode list or the Notes board — and scrolls it
// into view.
func (a *App) selectNote(index int) {
	if index < 0 || index >= len(a.notes) {
		return
	}
	if a.simple {
		entries := a.simpleEntries()
		for i, e := range entries {
			if e.isNote && e.noteIndex == index {
				a.simpleSelected = i
				a.syncSimpleScroll(entries)
				return
			}
		}
		return
	}
	a.notesSelected = index
	a.focus = focusNotes
	a.syncScroll()
}

func (a *App) deleteSelectedNote() {
	if a.notesSelected < 0 || a.notesSelected >= len(a.notes) {
		return
	}
	a.notes = append(a.notes[:a.notesSelected], a.notes[a.notesSelected+1:]...)
	a.clampSelections()
	a.persistNotes()
}

func (a *App) persistNotes() {
	if a.noPersist {
		return
	}
	if err := model.SaveNotes(a.notes); err != nil {
		a.err = "failed to save notes: " + err.Error()
	}
}

// leftPaneWidth is the outer width of the Today/Overdue panes. Delegates to
// geometry() so it tracks the active layout — in the stacked layout the task
// panes span the full width rather than a 3:5 column.
func (a *App) leftPaneWidth() int { return a.geometry().taskWidth }

// inputFieldWidth is how many cells the add-task value itself may occupy:
// the pane interior (outer width less border and padding) minus the "+ "
// prompt, the widget's own one-cell prompt, and the cursor block it always
// appends past the value. Under-reserving here doesn't wrap the line, it
// pushes it a cell past the pane border, since renderPane clamps height but
// not width.
func (a *App) inputFieldWidth() int {
	w := a.leftPaneWidth() - 4 -
		lipgloss.Width(inputPromptStyle.Render("+ ")) -
		lipgloss.Width(a.input.Prompt) - 1
	if w < 1 {
		w = 1
	}
	return w
}

// applyTaskEdit writes the inline input back onto the task being edited,
// re-parsing the trailing "HH:MM" the same way addTask does so an edit can
// promote a task to an appointment or demote it back. An emptied title is
// treated as a no-op rather than a delete: `d` is the deliberate way to
// remove a task, and silently destroying one on a stray ctrl+u would be a
// nasty surprise.
func (a *App) applyTaskEdit(raw string) {
	p, ok := parseTaskInput(raw)
	if !ok {
		return
	}
	for i := range a.tasks {
		if a.tasks[i].ID == a.taskEditID {
			// Every annotation is assigned unconditionally, so deleting one
			// from the text removes it from the task — an edit shows the
			// whole annotation set, so what's on screen is what's stored.
			a.tasks[i].Title = p.title
			a.tasks[i].Time = p.time
			a.tasks[i].EndTime = p.endTime
			a.tasks[i].Tags = p.tags
			// While deadlines are shelved, an edit must LEAVE an existing
			// DueDate alone: the editor can't show it, so re-assigning from
			// the parsed text would silently erase a deadline the user set
			// while the feature was on.
			if dueDatesEnabled {
				a.tasks[i].DueDate = a.dueDateFrom(p)
			}
			a.tasks[i].Kind = p.kind
			a.persist()
			a.selectTaskByID(a.taskEditID)
			return
		}
	}
}

// parseTaskInput splits a raw entry into its title and, when the last field
// looks like a clock time, an appointment time. Shared by addTask and
// applyTaskEdit so the two can't disagree about what "ends with HH:MM" means.
// dueDatesEnabled gates the whole "!Nd" deadline feature, which is shelved
// for now rather than removed: the parser, the list/sort behaviour and the
// chip renderer are all still here and still tested, and flipping this to
// true turns them back on in one edit.
//
// While it's false, "!2d" is ordinary text in a title, no task is held in
// Today's list by a deadline, and no chip is drawn. Any DueDate already in a
// saved tasks.json is left untouched rather than erased, so pausing the
// feature doesn't destroy data that was entered while it was on.
const dueDatesEnabled = false

// parsedTask is everything the annotation syntax can pull out of one line of
// input. Grouped into a struct rather than returned as five loose values so
// adding the next annotation doesn't churn every call site's signature.
type parsedTask struct {
	title   string
	time    string
	endTime string
	kind    model.Kind
	tags    []string
	// dueInDays is the "!Nd" offset, and dueSet distinguishes "!0d" (due
	// today) from no deadline at all — 0 is a meaningful value here.
	dueInDays int
	dueSet    bool
}

// parseTaskInput splits a raw entry into its title and annotations:
//
//	"standup 09:00"        → appointment at 09:00
//	"standup 09:00-09:15"  → appointment from 09:00 to 09:15
//	"fix login #api"       → task tagged "api", titled "fix login"
//
// The two annotation kinds are positional in different ways, which is why
// they're handled separately rather than in one pass. A time is only
// recognised as the LAST field — "meet 3 people at 09:00" shouldn't have its
// "3" eaten — whereas "#tag" is self-delimiting and so is lifted from
// anywhere in the line, which is what every tool with hashtags does.
//
// Returns ok=false when nothing but annotations was entered: a task has to
// have a title to be worth storing.
func parseTaskInput(raw string) (parsedTask, bool) {
	out := parsedTask{kind: model.KindTask}

	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return out, false
	}

	// The end-anchored annotations are consumed first, since tag handling
	// below doesn't remove words but the loop here does shift what "last"
	// means. Both are accepted in either order ("... 09:00 !2d" and
	// "... !2d 09:00"), because requiring a fixed order between two
	// independent annotations is a rule with nothing behind it.
	for len(fields) > 0 {
		last := fields[len(fields)-1]
		if days, ok := parseDueField(last); dueDatesEnabled && ok && !out.dueSet {
			out.dueInDays = days
			out.dueSet = true
			fields = fields[:len(fields)-1]
			continue
		}
		if start, end, isRange := parseTimeField(last); start != "" && out.time == "" {
			out.time = start
			out.kind = model.KindAppointment
			if isRange {
				out.endTime = end
			}
			fields = fields[:len(fields)-1]
			continue
		}
		break
	}

	// Tags are RECORDED but left in place: "review #api docs" keeps reading
	// as the sentence it was typed as, with the tag word merely coloured
	// differently. Lifting them to the end would rewrite the user's phrasing
	// on save, and an edit would then show text they never wrote.
	seen := map[string]bool{}
	for _, f := range fields {
		// A bare "#" is punctuation, not a tag, and duplicates are collapsed
		// so "#api fix #api" records one tag, not two.
		if len(f) > 1 && strings.HasPrefix(f, "#") {
			tag := strings.TrimPrefix(f, "#")
			if key := strings.ToLower(tag); !seen[key] {
				seen[key] = true
				out.tags = append(out.tags, tag)
			}
		}
	}

	out.title = strings.Join(fields, " ")
	if out.title == "" {
		return parsedTask{kind: model.KindTask}, false
	}
	return out, true
}

// parseDueField recognises a trailing "!Nd" deadline: !0d is today, !1d
// tomorrow, !Nd N days out. Any other shape returns false so the field stays
// part of the title.
//
// A negative offset is rejected rather than treated as "already overdue":
// there's no way to type a deadline in the past that isn't a typo, and an
// overdue task arrives at that state by the clock moving, not by being
// entered that way.
func parseDueField(f string) (days int, ok bool) {
	if f == "" || (f[len(f)-1] != 'd' && f[len(f)-1] != 'D') {
		return 0, false
	}
	body := f[:len(f)-1]

	// "‼Nd" is the overdue spelling the renderer produces, accepted on input
	// so that opening an overdue task and saving it unchanged preserves its
	// real deadline. Without this the editor would show a deadline the
	// syntax couldn't express and quietly reschedule it on every save.
	sign := 1
	switch {
	case strings.HasPrefix(body, "‼"):
		sign, body = -1, strings.TrimPrefix(body, "‼")
	case strings.HasPrefix(body, "!"):
		body = strings.TrimPrefix(body, "!")
	default:
		return 0, false
	}

	n, err := strconv.Atoi(body)
	// A negative offset typed by hand ("!-2d") is rejected: there's no way
	// to mean a deadline in the past that isn't a typo, and a task reaches
	// that state by the clock moving rather than by being entered that way.
	if err != nil || n < 0 {
		return 0, false
	}
	return sign * n, true
}

// parseTimeField recognises a trailing "HH:MM" or "HH:MM-HH:MM". The range
// form returns both ends; a bare time returns the start only. Anything else
// returns "" so the caller keeps the field as part of the title.
//
// A range whose end is not after its start is rejected outright rather than
// silently kept: "09:00-09:00" is a zero-length event and "10:00-09:00" runs
// backwards, and in both cases treating the text as a title is likelier to
// match what the user meant than storing a nonsense duration.
func parseTimeField(f string) (start, end string, isRange bool) {
	if isTimeLike(f) {
		return f, "", false
	}
	a, b, found := strings.Cut(f, "-")
	if !found || !isTimeLike(a) || !isTimeLike(b) || b <= a {
		return "", "", false
	}
	return a, b, true
}

// dueDateFrom resolves a parsed "!Nd" offset into the absolute date it means,
// anchored to today. Returns "" when no deadline was given.
func (a *App) dueDateFrom(p parsedTask) string {
	if !p.dueSet {
		return ""
	}
	return a.now().AddDate(0, 0, p.dueInDays).Format(dateFormat)
}

func (a *App) addTask(raw string) {
	p, ok := parseTaskInput(raw)
	if !ok {
		return
	}

	t := model.Task{
		ID:        strconv.FormatInt(a.now().UnixNano(), 36),
		Title:     p.title,
		Done:      false,
		Kind:      p.kind,
		Date:      a.now().Format(dateFormat),
		Time:      p.time,
		EndTime:   p.endTime,
		Tags:      p.tags,
		DueDate:   a.dueDateFrom(p),
		CreatedAt: a.now(),
	}

	a.tasks = append(a.tasks, t)
	a.persist()
	a.selectTaskByID(t.ID)
}

// selectTaskByID moves the selection onto the task with the given ID and
// scrolls it into view, so a freshly added task is the one under the cursor.
// Today's list is sorted (appointments by time) and simple mode's is merged by
// creation time, so the new row is found by ID rather than assumed to be last.
// An active filter can hide the task entirely, in which case the selection is
// left where clamping puts it.
func (a *App) selectTaskByID(id string) {
	if a.simple {
		entries := a.simpleEntries()
		for i, e := range entries {
			if !e.isNote && e.task.ID == id {
				a.simpleSelected = i
				a.syncSimpleScroll(entries)
				return
			}
		}
		return
	}
	for i, t := range a.todayTasks() {
		if t.ID == id {
			a.todaySelected = i
			// Selection only follows the cursor in the pane that owns it;
			// adding is Today-only, so focus Today to make the move visible.
			a.focus = focusToday
			a.syncScroll()
			return
		}
	}
	a.clampSelections()
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
	n := len(a.currentList())
	if a.focus == focusNotes {
		n = len(a.notes)
	}
	if n == 0 {
		return
	}
	sel := a.currentSelected()
	sel += delta
	if sel < 0 {
		sel = 0
	}
	if sel >= n {
		sel = n - 1
	}
	a.setCurrentSelected(sel)
	a.syncScroll()
}

// syncScroll keeps the current pane's scroll offset such that the selected
// row stays within the visible viewport, scrolling the minimal amount needed.
func (a *App) syncScroll() {
	visible := a.visibleRowsFor(a.focus)
	scroll := a.currentScroll()

	// On the notes board the selection is a NOTE index while the scroll
	// offset is a DISPLAY-LINE offset (a note wraps to several lines), so the
	// selected note's line span has to be resolved before they can be
	// compared — treating the two as the same unit scrolls to the wrong row
	// as soon as any note wraps.
	if a.focus == focusNotes {
		first, last := a.noteLineSpan(a.notesSelected)
		if first < 0 {
			return
		}
		if first < scroll {
			scroll = first
		} else if last >= scroll+visible {
			// Prefer showing the note's start when it's taller than the pane,
			// rather than its end.
			scroll = last - visible + 1
			if scroll > first {
				scroll = first
			}
		}
		if scroll < 0 {
			scroll = 0
		}
		a.notesScroll = scroll
		return
	}

	sel := a.currentSelected()
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

// noteLineSpan returns the first and last display-line indexes occupied by
// the given note at the current pane width, or (-1, -1) if out of range.
func (a App) noteLineSpan(index int) (int, int) {
	if index < 0 || index >= len(a.notes) {
		return -1, -1
	}
	lines := noteLines(a.notes, a.notesContentWidth())
	first, last := -1, -1
	for i, l := range lines {
		if l.noteIndex == index {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	return first, last
}

// notesContentWidth is the interior text width of the Notes pane.
func (a App) notesContentWidth() int {
	w := a.geometry().notesWidth - 4
	if w < 1 {
		w = 1
	}
	return w
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
	if a.notesSelected >= len(a.notes) {
		a.notesSelected = len(a.notes) - 1
	}
	if a.notesSelected < 0 {
		a.notesSelected = 0
	}
	a.syncScroll()
}

// migrateSelected carries the focused Overdue task onto today's list,
// stamping OriginalDate the first time so the task's age keeps counting from
// when it was FIRST scheduled rather than restarting at each migration.
func (a *App) migrateSelected() {
	list := a.currentList()
	sel := a.currentSelected()
	if sel < 0 || sel >= len(list) {
		return
	}
	id := list[sel].ID
	today := a.now().Format(dateFormat)
	for i := range a.tasks {
		if a.tasks[i].ID != id {
			continue
		}
		// Only stamp on the FIRST migration. Overwriting on every carry
		// forward would rebase the age counter and make deferring free,
		// which is exactly what the age badge exists to discourage.
		if a.tasks[i].OriginalDate == "" {
			a.tasks[i].OriginalDate = a.tasks[i].Date
		}
		a.tasks[i].Date = today
		a.persist()
		a.status = "Moved to today: " + a.tasks[i].Title
		break
	}
	// The task has left the Overdue list, so the cursor would otherwise sit
	// past its new end.
	a.clampSelections()
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

// saveSettings persists every user preference at once. Settings are written
// as a whole struct, so saving one field from a freshly-built Settings{} would
// blank the others — always send the full current state.
func (a App) saveSettings() {
	if a.noPersist {
		return
	}
	_ = model.SaveSettings(model.Settings{
		Theme:                     currentTheme().Name,
		Layout:                    a.layout.String(),
		PomodoroFocusMinutes:      a.pomo.workMinutes,
		PomodoroShortBreakMinutes: a.pomo.shortBreakMinutes,
		PomodoroLongBreakMinutes:  a.pomo.longBreakMinutes,
		PomodoroLongBreakEvery:    a.pomo.longBreakEvery,
		PomodoroAutoStartNext:     a.pomo.autoStartNext,
	})
}

// selectedTask returns the task under the cursor in the focused pane, or nil
// when the pane is empty (or the selection is somehow out of range).
func (a *App) selectedTask() *model.Task {
	list := a.currentList()
	sel := a.currentSelected()
	if sel < 0 || sel >= len(list) {
		return nil
	}
	return &list[sel]
}

// updateConfirmDelete handles the y/n prompt shown before a delete. Anything
// other than an explicit confirmation cancels, so a stray keypress can't
// destroy a task.
func (a App) updateConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		switch {
		case a.simple:
			a.deleteSimpleSelected()
		case a.focus == focusNotes:
			a.deleteSelectedNote()
		default:
			a.deleteSelected()
		}
		a.mode = modeNormal
		return a, nil
	default:
		a.mode = modeNormal
		a.status = "Delete cancelled"
		return a, nil
	}
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

// currentList is the task list of the focused pane. With Notes focused there
// is no task list, so it returns nil rather than falling through to Overdue —
// paired with currentSelected() returning notesSelected, an `else` here made
// every task operation silently act on an arbitrary overdue row.
func (a App) currentList() []model.Task {
	switch a.focus {
	case focusToday:
		return a.todayTasks()
	case focusOverdue:
		return a.overdueTasks()
	default:
		return nil
	}
}

func (a App) currentSelected() int {
	switch a.focus {
	case focusToday:
		return a.todaySelected
	case focusNotes:
		return a.notesSelected
	case focusOverdue:
		return a.overdueSelected
	default:
		// focusReports has no list; nothing meaningful to select.
		return 0
	}
}

func (a *App) setCurrentSelected(v int) {
	switch a.focus {
	case focusToday:
		a.todaySelected = v
	case focusNotes:
		a.notesSelected = v
	case focusOverdue:
		a.overdueSelected = v
	}
}

func (a App) currentScroll() int {
	switch a.focus {
	case focusToday:
		return a.todayScroll
	case focusNotes:
		return a.notesScroll
	case focusOverdue:
		return a.overdueScroll
	default:
		return 0
	}
}

func (a *App) setCurrentScroll(v int) {
	switch a.focus {
	case focusToday:
		a.todayScroll = v
	case focusNotes:
		a.notesScroll = v
	case focusOverdue:
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
	now := a.now()
	today := now.Format(dateFormat)
	var out []model.Task
	for _, t := range a.tasks {
		// A deadline keeps a task in Today's list regardless of the day it
		// was scheduled on — right up to the deadline and past it, since a
		// missed one is what most needs looking at. Completing it is what
		// takes it off the list, not the calendar.
		if t.Date == today || (dueDatesEnabled && t.HasDueDate() && !t.Done) {
			out = append(out, t)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		di, iHas := out[i].DaysUntilDue(now)
		dj, jHas := out[j].DaysUntilDue(now)
		if !dueDatesEnabled {
			iHas, jHas = false, false
		}

		// Deadlines pin to the top, ordered by urgency: the most overdue
		// first, counting up through today and out to the furthest deadline.
		// One continuous gradient rather than an "overdue" block above a
		// "upcoming" one — they're the same axis, and splitting them would
		// put a task due today below one that was due yesterday for no
		// reason the user can see.
		if iHas != jHas {
			return iHas
		}
		if iHas && di != dj {
			return di < dj
		}

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
		// A task with a deadline lives in Today's list until it's done, so
		// excluding it here keeps it from appearing in both panes at once.
		if t.Date < today && !t.Done && !(dueDatesEnabled && t.HasDueDate()) {
			out = append(out, t)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Date < out[j].Date
	})
	return a.applyFilters(out)
}

func (a App) helpGroups() []helpGroup {
	if a.simple && a.mode == modeNormal {
		what := "task"
		if a.simpleNoteMode {
			what = "note"
		}
		return []helpGroup{
			{"", []helpKey{
				{"a", "add " + what}, {"tab", "switch to " + map[bool]string{true: "task", false: "note"}[a.simpleNoteMode]},
				{"space", "toggle"}, {"enter", "edit"}, {"d", "delete"}, {"i", "important"},
				{"↑/↓ j/k", "navigate"}, {"S", "settings"}, {"q", "quit"},
			}},
		}
	}

	// While a confirmation is up the help bar is emptied rather than
	// duplicated: the prompt on the status line already carries its own
	// [y]/[any other key] hint, and every other binding is inert until the
	// prompt is answered, so listing them would offer keys that do nothing.
	if a.mode == modeConfirmDelete || a.mode == modeConfirmClearNotes {
		return nil
	}
	// The Settings modal draws its own key hints inside its box, same reason
	// as the confirmation prompts above.
	if a.mode == modeSettings {
		return nil
	}
	if a.mode == modeNoteEditing {
		return []helpGroup{
			{"", []helpKey{
				{"enter", "save"}, {"ctrl+j / opt+enter", "new line"},
				// Esc is how you LEAVE the add loop, not how you undo the
				// notes already written, so it isn't a "cancel" there.
				{"esc", map[bool]string{true: "done", false: "cancel"}[a.noteEditIndex < 0]},
			}},
		}
	}
	if a.mode == modeAdding {
		// One hint for both annotations. The time is position-sensitive
		// (last field only) and the tag isn't, which is worth saying since
		// it's the one rule that isn't guessable.
		hint := "#tag anywhere  ·  HH:MM or HH:MM-HH:MM at the end"
		return []helpGroup{
			{"", []helpKey{
				{"enter", "save"}, {"esc", "cancel"},
				{"", hint},
			}},
		}
	}
	if a.focus == focusNotes {
		notesKeys := []helpKey{
			{"a", "add"}, {"enter", "edit"}, {"d", "delete"}, {"C", "clear board"},
		}
		viewKeys := []helpKey{{"tab", "switch pane"}, {"↑/↓ j/k", "navigate"}}
		if a.notesExpanded {
			notesKeys = append(notesKeys, helpKey{"e", "shrink"})
			// No pane to switch to while expanded, and tab is inert there.
			viewKeys = []helpKey{{"↑/↓ j/k", "navigate"}}
		} else {
			notesKeys = append(notesKeys, helpKey{"e", "expand"})
		}
		return []helpGroup{
			{"Notes", notesKeys},
			{"View", viewKeys},
			{"App", []helpKey{
				{"S", "settings"}, {"q", "quit"},
			}},
		}
	}
	if a.focus == focusReports {
		// The Reports pane has no list and no items, so none of the task or
		// note bindings apply — only chart navigation and the app-wide keys.
		return []helpGroup{
			{"Chart", []helpKey{
				{"←/→ h/l", "switch chart"}, {"", a.reportChart.String()},
			}},
			{"View", []helpKey{{"tab", "switch pane"}}},
			{"App", []helpKey{
				{"S", "settings"}, {"q", "quit"},
			}},
		}
	}

	// Adding is Today-only (there's no such thing as adding a task that's
	// already overdue), and so is editing — the inline input lives in the
	// Today pane. Both hints are omitted when Overdue has focus rather than
	// advertising keys that do nothing there.
	//
	// Space means different things in the two panes — toggle done in Today,
	// carry the task forward in Overdue — so the hint has to say which.
	var taskKeys []helpKey
	if a.focus == focusOverdue {
		taskKeys = []helpKey{{"space", "move to today"}}
	} else {
		taskKeys = []helpKey{{"a", "add"}, {"space", "done"}, {"enter", "edit"}}
	}
	taskKeys = append(taskKeys,
		helpKey{"d", "delete"}, helpKey{"i", "important"})

	return []helpGroup{
		{"Task", taskKeys},
		{"View", []helpKey{
			{"tab", "switch pane"}, {"↑/↓ j/k", "navigate"}, {"I/U", "filters"},
		}},
		// Pomodoro's keys aren't listed here — they're rendered inside the
		// Pomodoro pane itself, next to the thing they control.
		{"App", []helpKey{
			{"S", "settings"}, {"q", "quit"},
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
	// Measured against the NORMAL bindings, not whatever the current mode
	// shows. A confirmation empties the help bar, and a mode-sensitive
	// measurement would then hand those rows to the panes — so opening a
	// prompt made every pane grow, and answering it made them shrink back,
	// jumping the layout underneath the question being asked.
	normal := a
	normal.mode = modeNormal
	help := lipgloss.Height(renderHelpBar(normal.helpGroups(), a.width))
	return errLine + help
}

// visibleRowsFor computes how many task rows fit in the given pane's content
// height. Mirrors the height math used when actually laying out the panes in
// View(), so scroll math and rendering never disagree about viewport size.
func (a App) visibleRowsFor(focus focusedPane) int {
	g := a.geometry()
	paneHeight := g.todayHeight
	switch focus {
	case focusOverdue:
		paneHeight = g.overdueHeight
	case focusNotes:
		paneHeight = g.notesHeight
	}
	contentHeight := paneHeight - 2 // border top+bottom
	// No title row is reserved: the pane title is drawn ON the top border by
	// renderPane, so it costs no body line.
	//
	// renderTaskList always emits a scroll-indicator line (blank when there's
	// nothing to scroll), so its line is reserved unconditionally here —
	// otherwise the pane grows by a line whenever "N more" starts appearing.
	indicatorLines := 1
	rows := contentHeight - indicatorLines
	if focus == focusToday && a.mode == modeAdding {
		rows-- // reserve a line for the inline add-task input
	}
	if focus == focusNotes && a.mode == modeNoteEditing {
		// The note editor is multi-line, so reserve its full height.
		rows -= a.noteEditorHeight()
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

	page := a.renderPage()
	if a.mode == modeSettings {
		return overlayModal(page, a.renderSettingsModal(), a.width, a.height)
	}
	return page
}

// renderPage builds the full frame for every mode except modeSettings, which
// composes its modal on top of this in View() instead of threading modal
// state through the per-pane rendering below.
func (a App) renderPage() string {
	helpLine := renderHelpBar(a.helpGroups(), a.width)

	// chromeLines reserves the help bar's NORMAL height so the panes don't
	// resize when a mode blanks it (see there). Pad back up to that height
	// with empty rows, or the page would render short of the terminal and
	// leave unpainted lines at the bottom.
	if want := a.chromeLines() - 1; lipgloss.Height(helpLine) < want {
		helpLine += strings.Repeat("\n", want-lipgloss.Height(helpLine))
	}

	var page string
	if a.simple {
		page = a.assemblePage(a.renderSimple(), helpLine)
	} else {
		g := a.geometry()
		if a.notesExpanded && a.focus == focusNotes {
			page = a.assemblePage(a.renderNotesPane(g), helpLine)
		} else {
			leftWidth := g.taskWidth
			rightWidth := g.infoWidth
			todayHeight := g.todayHeight
			overdueHeight := g.overdueHeight

			filters := filterLabel(a.filterImportant, a.filterUndone)

			today := a.todayTasks()
			todayVisible := a.visibleRowsFor(focusToday)
			todayBody := renderTaskList(today, a.todaySelected, a.todayScroll, todayVisible, a.focus == focusToday, false, leftWidth-4, a.now())
			if a.mode == modeAdding {
				a.input.TextStyle = lipgloss.NewStyle().Foreground(colorText).Background(colorPaneBg)
				a.input.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorMuted).Background(colorPaneBg)
				a.input.PromptStyle = lipgloss.NewStyle().Foreground(colorAccent).Background(colorPaneBg)
				a.input.Cursor.Style = lipgloss.NewStyle().Foreground(colorText).Background(colorPaneBg)

				if field := a.inputFieldWidth(); lipgloss.Width(a.input.Placeholder) > field {
					a.input.Placeholder = fitToWidth(a.input.Placeholder, field)
				}

				a.input.Width = 0
				glyph := "+ "
				if a.taskEditID != "" {
					glyph = "✎ "
				}
				inputLine := inputPromptStyle.Render(glyph) + a.input.View()
				if pad := (leftWidth - 4) - lipgloss.Width(inputLine); pad > 0 {
					inputLine += lipgloss.NewStyle().Background(colorPaneBg).Render(strings.Repeat(" ", pad))
				}
				todayBody += "\n" + inputLine
			}
			todayPane := renderPane(fmt.Sprintf("Today (%d)%s", len(today), filters), todayBody, a.focus == focusToday, leftWidth, todayHeight)

			overdue := a.overdueTasks()
			overdueWidth := leftWidth
			overdueVisible := a.visibleRowsFor(focusOverdue)
			overdueBody := renderTaskList(overdue, a.overdueSelected, a.overdueScroll, overdueVisible, a.focus == focusOverdue, true, overdueWidth-4, a.now())
			overduePane := renderPane(fmt.Sprintf("Overdue (%d)%s", len(overdue), filters), overdueBody, a.focus == focusOverdue, overdueWidth, overdueHeight)

			tasks := lipgloss.JoinVertical(lipgloss.Left, todayPane, overduePane)
			if a.layout == layoutStacked {
				tasks = lipgloss.JoinHorizontal(lipgloss.Top, todayPane, overduePane)
			}

			greetWidth, reportsWidth, pomoWidth := rightWidth, rightWidth, rightWidth
			if a.layout == layoutStacked {
				pomoWidth = a.width - greetWidth - reportsWidth
			}

			greetBody := renderGreeting(a.now(), a.username, greetWidth-4, g.greetHeight-2)
			greetPane := renderPane("", greetBody, false, greetWidth, g.greetHeight)

			report := stats.Compute(a.tasks, a.now())
			reportsBody := renderReports(report, reportsWidth-4, g.reportsHeight-2, a.reportChart, a.focus == focusReports)
			reportsPane := renderPane("Reports", reportsBody, a.focus == focusReports, reportsWidth, g.reportsHeight)

			pomoBody := renderPomodoro(a.pomo, pomoWidth-4, g.pomoHeight-2)
			pomoPane := renderPane("Pomodoro", pomoBody, false, pomoWidth, g.pomoHeight)

			notesPane := a.renderNotesPane(g)

			if a.layout == layoutStacked && notesPane != "" {
				tasks = lipgloss.JoinHorizontal(lipgloss.Top, tasks, notesPane)
			}

			infoPanes := []string{greetPane, reportsPane, pomoPane}
			if a.layout != layoutStacked && a.layout != layoutThreeColumn && notesPane != "" {
				infoPanes = append(infoPanes, notesPane)
			}

			gutter := gutterColumn(lipgloss.Height(tasks))

			var body string
			switch a.layout {
			case layoutTasksRight:
				info := lipgloss.JoinVertical(lipgloss.Left, infoPanes...)
				body = lipgloss.JoinHorizontal(lipgloss.Top, info, gutter, tasks)
			case layoutStacked:
				info := lipgloss.JoinHorizontal(lipgloss.Top, greetPane, reportsPane, pomoPane)
				body = lipgloss.JoinVertical(lipgloss.Left, info, tasks)
			case layoutThreeColumn:
				info := lipgloss.JoinVertical(lipgloss.Left, infoPanes...)
				body = lipgloss.JoinHorizontal(lipgloss.Top, info, gutter, tasks, gutter, notesPane)
			default:
				info := lipgloss.JoinVertical(lipgloss.Left, infoPanes...)
				body = lipgloss.JoinHorizontal(lipgloss.Top, tasks, gutter, info)
			}

			page = a.assemblePage(body, helpLine)
		}
	}

	return page
}

// assemblePage stacks the body, the status/prompt line and the help bar into
// the final frame, padding every line to the terminal width. Shared by the
// normal layouts and the expanded Notes view so the status line, prompts and
// background padding behave identically in both.
func (a App) assemblePage(body, helpLine string) string {
	pageBg := lipgloss.NewStyle().Background(colorBg)

	errLine := ""
	switch {
	case a.mode == modeConfirmClearNotes:
		errLine = lipgloss.NewStyle().Bold(true).Foreground(colorDanger).Background(colorBg).
			Render(fmt.Sprintf("Clear all %d notes?", len(a.notes))) +
			confirmHint()

	case a.mode == modeConfirmDelete:
		// The confirmation takes over the status line so it's impossible to
		// miss, and names the item so there's no doubt about what's going.
		prompt := "Delete this task?"
		if a.simple {
			entries := a.simpleEntries()
			if a.simpleSelected >= 0 && a.simpleSelected < len(entries) {
				e := entries[a.simpleSelected]
				if e.isNote {
					prompt = fmt.Sprintf("Delete %q?", strings.SplitN(e.note.Body, "\n", 2)[0])
				} else {
					prompt = fmt.Sprintf("Delete %q?", e.task.Title)
				}
			}
		} else if a.focus == focusNotes {
			prompt = "Delete this note?"
			if a.notesSelected >= 0 && a.notesSelected < len(a.notes) {
				// First line only: a note can be many lines long.
				first := strings.SplitN(a.notes[a.notesSelected].Body, "\n", 2)[0]
				prompt = fmt.Sprintf("Delete %q?", first)
			}
		} else if t := a.selectedTask(); t != nil {
			prompt = fmt.Sprintf("Delete %q?", t.Title)
		}
		errLine = lipgloss.NewStyle().Bold(true).Foreground(colorDanger).Background(colorBg).Render(prompt) +
			confirmHint()
	case a.err != "":
		errLine = lipgloss.NewStyle().Foreground(colorDanger).Background(colorBg).Render(a.err)
	case a.status != "":
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
			switch pad := a.width - lipgloss.Width(l); {
			case pad > 0:
				lines[i] = l + pageBg.Render(strings.Repeat(" ", pad))
			case pad < 0:
				// Clamp too: an overlong line (e.g. the delete prompt naming
				// a very long task title) would otherwise push the page past
				// the terminal edge, since padding alone only ever grows.
				lines[i] = truncateANSI(l, a.width)
			}
		}
		return strings.Join(lines, "\n")
	}

	// In simple mode the body carries its own side margins, so only the
	// status and help lines are indented here — otherwise they'd sit flush
	// against the terminal edge while everything above is held off it (and
	// indenting the body too would apply its margin twice). `own` is the
	// margin the line already has (renderHelpBar prefixes one space), which
	// is subtracted so everything lands on the same column.
	indentLines := func(s string, own int) string {
		if !a.simple || s == "" {
			return s
		}
		n := simplePadding - own
		if n <= 0 {
			return s
		}
		pre := pageBg.Render(strings.Repeat(" ", n))
		lines := strings.Split(s, "\n")
		for i, l := range lines {
			if strings.TrimSpace(ansiRe.ReplaceAllString(l, "")) != "" {
				lines[i] = pre + l
			}
		}
		return strings.Join(lines, "\n")
	}

	full := lipgloss.JoinVertical(lipgloss.Left,
		padLines(body),
		padLines(indentLines(errLine, 0)),
		padLines(indentLines(helpLine, 1)),
	)
	return full
}

func padPanelLine(s string, w int, bg lipgloss.Color) string {
	if pad := w - lipgloss.Width(s); pad > 0 {
		return s + lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", pad))
	} else if pad < 0 {
		return truncateANSI(s, w)
	}
	return s
}
