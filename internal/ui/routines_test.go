package ui

import (
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/parsaenami/taskii/internal/model"
)

func routineKey(a App, key string) App {
	var msg tea.KeyMsg
	switch key {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case " ":
		msg = tea.KeyMsg{Type: tea.KeySpace}
	case "tab":
		msg = tea.KeyMsg{Type: tea.KeyTab}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		msg = tea.KeyMsg{Type: tea.KeyUp}
	case "right":
		msg = tea.KeyMsg{Type: tea.KeyRight}
	case "left":
		msg = tea.KeyMsg{Type: tea.KeyLeft}
	case "home":
		msg = tea.KeyMsg{Type: tea.KeyHome}
	case "end":
		msg = tea.KeyMsg{Type: tea.KeyEnd}
	case "shift+tab":
		msg = tea.KeyMsg{Type: tea.KeyShiftTab}
	case "ctrl+s":
		msg = tea.KeyMsg{Type: tea.KeyCtrlS}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	m, _ := a.Update(msg)
	return m.(App)
}

func routineFixture(simple bool) App {
	now := time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC)
	a := NewApp(Options{Mock: true, Simple: simple})
	a.now = func() time.Time { return now }
	a.routines = []model.Routine{
		{ID: "r1", Title: "Coffee", CreatedAt: now.AddDate(0, 0, -4), Schedule: model.ScheduleEveryDay},
		{ID: "r2", Title: "Email", CreatedAt: now.AddDate(0, 0, -3), Schedule: model.ScheduleWorkdays},
	}
	a.tasks = []model.Task{{ID: "t1", Title: "Task", Date: now.Format(dateFormat), CreatedAt: now}}
	a.notes = []model.Note{{ID: "n1", Body: "Note", CreatedAt: now}}
	a.width, a.height = 110, 36
	return a
}

func TestRoutineTodayNavigationAndIsolation(t *testing.T) {
	a := routineFixture(false)
	if len(a.todayEntries()) != 3 || !strings.Contains(ansiRe.ReplaceAllString(a.View(), ""), "ROUTINES") {
		t.Fatal("missing Today routine section")
	}
	a = routineKey(a, "i")
	a = routineKey(a, "d")
	if a.mode != modeNormal || a.tasks[0].Important {
		t.Fatal("routine triggered task mutation")
	}
	a = routineKey(a, " ")
	if a.routines[0].History[a.now().Format(dateFormat)] != model.RoutineCompleted {
		t.Fatal("toggle did not complete")
	}
	a = routineKey(a, "s")
	if a.routines[0].History[a.now().Format(dateFormat)] != model.RoutineCompleted {
		t.Fatal("skip overwrote a completed routine")
	}
	if !strings.Contains(a.err, "cannot be skipped") || !strings.Contains(ansiRe.ReplaceAllString(a.View(), ""), a.err) {
		t.Fatalf("rejected skip did not show the bottom error: %q", a.err)
	}
	a = routineKey(a, "down")
	a = routineKey(a, "down")
	if got := a.selectedTask(); got == nil || got.ID != "t1" {
		t.Fatalf("boundary selection = %+v", got)
	}
	a = routineKey(a, "i")
	if !a.tasks[0].Important {
		t.Fatal("task action lost after routine prefix")
	}
	a.filterImportant = true
	if len(a.todayEntries()) != 3 {
		t.Fatal("filter hid routines")
	}
	a.filterUndone = true
	a.tasks[0].Done = true
	if len(a.todayEntries()) != 2 {
		t.Fatal("undone filter failed to filter tasks only")
	}
	a.setUpcoming(true)
	if len(a.todayEntries()) != 0 || strings.Contains(ansiRe.ReplaceAllString(a.View(), ""), "ROUTINES") {
		t.Fatal("routines leaked into Upcoming")
	}
}

func TestRoutineSimpleSelectionSurfaceAndSkip(t *testing.T) {
	a := routineFixture(true)
	if got := a.simpleEntries(); len(got) != 4 || !got[0].isRoutine || !got[1].isRoutine || got[2].isRoutine {
		t.Fatalf("entries: %+v", got)
	}
	view := a.View()
	if strings.Contains(ansiRe.ReplaceAllString(view, ""), "\nROUTINES") {
		t.Fatal("unexpected unindented section")
	}
	if !strings.Contains(ansiRe.ReplaceAllString(view, ""), "ROUTINES") {
		t.Fatal("missing section")
	}
	a = routineKey(a, "s")
	if len(a.dueRoutines()) != 1 || a.routines[0].History[a.now().Format(dateFormat)] != model.RoutineSkipped {
		t.Fatal("skip did not hide routine")
	}
	a = routineKey(a, "R")
	if a.mode != modeRoutineManager {
		t.Fatal("manager not open")
	}
	a = routineKey(a, "s")
	if a.routines[0].History[a.now().Format(dateFormat)] != "" {
		t.Fatal("skip not restored")
	}
	a = routineKey(a, "esc")
	a.setUpcoming(true)
	for _, e := range a.simpleEntries() {
		if e.isRoutine {
			t.Fatal("routine in Upcoming")
		}
	}
}

func TestRoutineManagerCRUDValidationAndHistory(t *testing.T) {
	a := routineFixture(false)
	a = routineKey(a, "R")
	a = routineKey(a, "a")
	if a.mode != modeRoutineEditor {
		t.Fatal("editor not opened")
	}
	a = routineKey(a, "ctrl+s")
	if a.routineUI.errorText == "" || len(a.routines) != 2 {
		t.Fatal("empty title accepted")
	}
	a.routineUI.title.SetValue("Sentry")
	a.routineUI.field = 1
	a = routineKey(a, "right")
	a = routineKey(a, "right")
	a = routineKey(a, "ctrl+s")
	if a.routineUI.errorText == "" {
		t.Fatal("empty custom weekdays accepted")
	}
	a.routineUI.field = 2
	a = routineKey(a, " ")
	a = routineKey(a, "ctrl+s")
	if len(a.routines) != 3 || a.routines[2].Schedule != model.ScheduleCustom || len(a.routines[2].CustomWeekdays) != 1 {
		t.Fatalf("add failed: %+v", a.routines)
	}
	a.routineUI.selected = 0
	a.routines[0].History = map[string]model.RoutineStatus{a.now().Format(dateFormat): model.RoutineCompleted}
	a = routineKey(a, "enter")
	a.routineUI.title.SetValue("Coffee revised")
	a.routineUI.field = 1
	a = routineKey(a, "right")
	a = routineKey(a, "ctrl+s")
	if a.routines[0].Title != "Coffee revised" || a.routines[0].History[a.now().Format(dateFormat)] != model.RoutineCompleted {
		t.Fatal("edit lost title or history")
	}
	a.routineUI.selected = 0
	a = routineKey(a, "d")
	if a.deleteItemID != "r1" || a.mode != modeConfirmDelete {
		t.Fatal("delete confirmation absent")
	}
	a.routineUI.selected = 1
	a = routineKey(a, "y")
	if len(a.routines) != 3 || a.status != "Routine list changed; delete cancelled" {
		t.Fatal("confirmation deleted wrong identity")
	}
	a.routineUI.selected = 0
	a = routineKey(a, "d")
	a = routineKey(a, "y")
	if len(a.routines) != 2 || a.routines[0].ID == "r1" {
		t.Fatal("delete failed")
	}
}

func TestRoutineScheduleEditSettlesOldDaysAndRollover(t *testing.T) {
	a := routineFixture(false)
	base := a.now()
	a.routines[0].CreatedAt = base.AddDate(0, 0, -2)
	a.routines[0].History = nil
	a = routineKey(a, "R")
	a = routineKey(a, "enter")
	a.routineUI.field = 1
	a = routineKey(a, "right")
	a = routineKey(a, "ctrl+s")
	for _, off := range []int{-2, -1} {
		if got := a.routines[0].History[base.AddDate(0, 0, off).Format(dateFormat)]; got != model.RoutineMissed {
			t.Fatalf("old recurrence on %d = %s", off, got)
		}
	}
	if _, ok := a.routines[0].History[base.Format(dateFormat)]; ok {
		t.Fatal("today recorded early")
	}
	writes := a.routines[0].LastEvaluatedDate
	a.now = func() time.Time { return base.AddDate(0, 0, 1) }
	a.advanceRoutineDate()
	if a.routines[0].History[base.Format(dateFormat)] != model.RoutineMissed {
		t.Fatal("midnight did not catch up")
	}
	if a.routines[0].LastEvaluatedDate == writes {
		t.Fatal("cursor not advanced")
	}
	state := a.routines[0].LastEvaluatedDate
	a.advanceRoutineDate()
	if a.routines[0].LastEvaluatedDate != state {
		t.Fatal("repeat tick advanced cursor")
	}
}

func TestRoutineDateRolloverResetsSelectionIdentity(t *testing.T) {
	for _, simple := range []bool{false, true} {
		a := routineFixture(simple)
		a.todaySelected, a.simpleSelected = 2, 2 // task, after due routines
		a.todayScroll, a.simpleScroll = 2, 2
		base := a.now()
		a.now = func() time.Time { return base.AddDate(0, 0, 1) }
		a.advanceRoutineDate()
		if a.todaySelected != 0 || a.simpleSelected != 0 || a.todayScroll != 0 || a.simpleScroll != 0 {
			t.Fatalf("simple=%t: rollover retained stale selection/scroll: %+v", simple, a)
		}
	}
}

func TestRoutineModalGeometryAndMock(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {110, 36}, {160, 45}} {
		a := routineFixture(false)
		a.width, a.height = size[0], size[1]
		a = routineKey(a, "R")
		for _, edit := range []bool{false, true} {
			if edit {
				a = routineKey(a, "enter")
			}
			view := a.View()
			if got := lipgloss.Height(view); got != a.height {
				t.Errorf("size %v edit=%v height=%d", size, edit, got)
			}
			for _, l := range strings.Split(view, "\n") {
				if lipgloss.Width(l) != a.width {
					t.Errorf("size %v edit=%v width=%d", size, edit, lipgloss.Width(l))
					break
				}
			}
		}
	}
	a := NewApp(Options{Mock: true})
	if len(a.routines) != 3 || a.routines[0].History[a.now().Format(dateFormat)] != model.RoutineCompleted {
		t.Fatalf("mock routines: %+v", a.routines)
	}
}

func TestRoutineModalKeyHintsUseAppStyleAndStayAtBottom(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(old)
	applyTheme(currentTheme())

	a := routineFixture(false)
	a.width, a.height = 100, 30
	a.openRoutineManager()

	assertFooter := func(modal string, wants []string) {
		t.Helper()
		plainLines := strings.Split(ansiRe.ReplaceAllString(modal, ""), "\n")
		footer := strings.Join(plainLines[len(plainLines)-3:len(plainLines)-1], "\n")
		for _, want := range wants {
			if !strings.Contains(footer, want) {
				t.Fatalf("routine footer missing %q at bottom:\n%s", want, footer)
			}
		}
		if !strings.Contains(modal, paneKeyStyle.Render("[esc]")) {
			t.Fatal("routine key hint does not use the pane key style")
		}
	}

	assertFooter(a.renderRoutineModal(), []string{"[a] add", "[esc] close"})
	a = routineKey(a, "enter")
	assertFooter(a.renderRoutineModal(), []string{"[space] day", "[ctrl+s] save", "[esc] cancel"})
}

func TestRoutineMutationsPersistAndMockDoesNot(t *testing.T) {
	isolateUICalendarData(t)
	a := NewApp(Options{})
	day := a.now().Format(dateFormat)
	r := model.Routine{ID: "persist-r", Title: "Persist", CreatedAt: a.now(), Schedule: model.ScheduleEveryDay}
	a.routines = []model.Routine{r}
	if !a.toggleRoutine(r.ID) {
		t.Fatal("toggle rejected")
	}
	loaded, err := model.LoadRoutines()
	if err != nil || len(loaded) != 1 || loaded[0].History[day] != model.RoutineCompleted {
		t.Fatalf("saved toggle = %+v, %v", loaded, err)
	}
	if a.skipRoutine(r.ID) {
		t.Fatal("completed routine was skipped")
	}
	if !strings.Contains(a.err, "cannot be skipped") {
		t.Fatalf("completed skip error = %q", a.err)
	}
	loaded, err = model.LoadRoutines()
	if err != nil || loaded[0].History[day] != model.RoutineCompleted {
		t.Fatalf("rejected skip changed persisted completion = %+v, %v", loaded, err)
	}
	if !a.toggleRoutine(r.ID) || !a.skipRoutine(r.ID) {
		t.Fatal("pending routine could not be skipped")
	}
	loaded, err = model.LoadRoutines()
	if err != nil || loaded[0].History[day] != model.RoutineSkipped {
		t.Fatalf("saved pending skip = %+v, %v", loaded, err)
	}
	a.deleteRoutine(r.ID)
	loaded, err = model.LoadRoutines()
	if err != nil || len(loaded) != 0 {
		t.Fatalf("saved deletion = %+v, %v", loaded, err)
	}
	m := NewApp(Options{Mock: true})
	m.toggleRoutine("mock-coffee")
	loaded, err = model.LoadRoutines()
	if err != nil || len(loaded) != 0 {
		t.Fatalf("mock touched real routines: %+v, %v", loaded, err)
	}
}

func TestRoutineFrameDimensionsAcrossLayoutsAndModes(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(old)
	for _, simple := range []bool{false, true} {
		for _, layout := range allLayouts {
			for _, size := range [][2]int{{80, 24}, {100, 30}, {160, 45}} {
				a := routineFixture(simple)
				a.layout = layout
				a.width, a.height = size[0], size[1]
				for _, modeName := range []string{"normal", "manager", "editor", "confirm"} {
					b := a
					switch modeName {
					case "manager":
						b.openRoutineManager()
					case "editor":
						b.openRoutineManager()
						b = routineKey(b, "enter")
					case "confirm":
						b.openRoutineManager()
						b = routineKey(b, "d")
					}
					frame := b.View()
					if h := lipgloss.Height(frame); h != b.height {
						t.Errorf("simple=%v layout=%v size=%v mode=%s height=%d", simple, layout, size, modeName, h)
					}
					for i, line := range strings.Split(frame, "\n") {
						if w := lipgloss.Width(line); w != b.width {
							t.Errorf("simple=%v layout=%v size=%v mode=%s row=%d width=%d", simple, layout, size, modeName, i, w)
							break
						}
					}
				}
			}
		}
	}
}

func TestSimpleRoutineSectionHasHeadingAndUpcomingDoesNot(t *testing.T) {
	a := routineFixture(true)
	lines := a.simpleLines(a.simpleEntries(), a.simpleListWidth())
	if len(lines) == 0 || lines[0].entryIndex != -1 || lines[0].text != "ROUTINES" {
		t.Fatalf("missing routine heading: %+v", lines)
	}
	if !strings.Contains(ansiRe.ReplaceAllString(a.View(), ""), "ROUTINES") {
		t.Fatal("routine heading not visible in simple mode")
	}
	a.upcoming = true
	for _, line := range a.simpleLines(a.simpleEntries(), a.simpleListWidth()) {
		if line.text == "ROUTINES" {
			t.Fatal("routine heading leaked into Upcoming")
		}
	}
}

func TestTodayRoutineSectionKeepsEmptyTasksHeading(t *testing.T) {
	a := routineFixture(false)
	a.tasks = nil
	plain := ansiRe.ReplaceAllString(a.renderTodayList(a.visibleRowsFor(focusToday), a.geometry().taskWidth-4), "")
	if !strings.Contains(plain, "ROUTINES") || !strings.Contains(plain, "TASKS") {
		t.Fatalf("missing section heading with no tasks: %s", plain)
	}
}

func TestRoutineManagerScrollKeepsSelectionVisible(t *testing.T) {
	a := routineFixture(false)
	for i := 0; i < 30; i++ {
		a.routines = append(a.routines, model.Routine{
			ID: "extra-" + string(rune('a'+i)), Title: "Routine extra " + string(rune('a'+i)),
			CreatedAt: a.now(), Schedule: model.ScheduleEveryDay,
		})
	}
	a.openRoutineManager()
	for i := 0; i < 25; i++ {
		a = routineKey(a, "down")
	}
	if a.routineUI.selected < a.routineUI.scroll || a.routineUI.selected >= a.routineUI.scroll+a.routineManagerVisibleRows() {
		t.Fatalf("selected %d outside scroll %d viewport %d", a.routineUI.selected, a.routineUI.scroll, a.routineManagerVisibleRows())
	}
	selected := a.routines[a.routineUI.selected].Title
	modal := ansiRe.ReplaceAllString(a.renderRoutineModal(), "")
	if !strings.Contains(modal, selected) {
		t.Fatalf("selected routine %q missing from manager viewport", selected)
	}
}

func TestRoutineEditorTitleCursorUsesArrowKeys(t *testing.T) {
	a := routineFixture(false)
	a.openRoutineManager()
	a = routineKey(a, "enter")
	a.routineUI.title.SetValue("Coffee")
	a.routineUI.title.CursorEnd()
	a = routineKey(a, "left")
	if got := a.routineUI.title.Position(); got != len("Coffee")-1 {
		t.Fatalf("left arrow ignored by title input: position %d", got)
	}
	a = routineKey(a, "X")
	if got := a.routineUI.title.Value(); got != "CoffeXe" {
		t.Fatalf("typing at moved cursor = %q", got)
	}
}

func routineCursorTrueColor(t *testing.T) {
	t.Helper()
	oldProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(oldProfile) })
}

// Check the styled character separately from the surrounding plain viewport:
// an inserted cursor glyph or highlighting the wrong repeated letter cannot
// satisfy both the span and text-position assertions.
func assertRoutineTitleCursor(t *testing.T, a App, inside int) string {
	t.Helper()
	input := a.routineUI.title
	value, pos := []rune(input.Value()), input.Position()
	cursorText, suffix := " ", ""
	if pos < len(value) {
		cursorText, suffix = string(value[pos]), string(value[pos+1:])
	}
	span := lipgloss.NewStyle().Foreground(currentTheme().AppTitleFg).Background(colorAccent).Render(cursorText)
	row := renderRoutineTitle(input, inside, true)
	if !strings.Contains(span, "\x1b[") || strings.Count(row, span) != 1 {
		t.Fatalf("position=%d: want exactly one filled cursor span %q in %q", pos, span, row)
	}
	parts := strings.Split(row, span)
	left := strings.TrimPrefix(ansiRe.ReplaceAllString(parts[0], ""), "▸ ")
	right := strings.TrimRight(ansiRe.ReplaceAllString(parts[1], ""), " ")
	if !strings.HasSuffix(string(value[:pos]), left) || !strings.HasPrefix(suffix, right) {
		t.Fatalf("cursor not at rune %d: left=%q cursor=%q right=%q value=%q", pos, left, cursorText, right, input.Value())
	}
	plain := ansiRe.ReplaceAllString(row, "")
	if lipgloss.Width(row) != inside || strings.Contains(plain, "▌") {
		t.Fatalf("position=%d: invalid title viewport %q (width %d, want %d)", pos, plain, lipgloss.Width(row), inside)
	}
	needed := lipgloss.Width(input.Value())
	if pos == len(value) {
		needed++ // the end cursor is a blank, not another title character
	}
	if needed <= inside-2 && plain != fitToWidth("▸ "+input.Value(), inside) {
		t.Fatalf("cursor altered plain title: %q, value=%q", plain, input.Value())
	}
	if err := requireBackgroundEveryCell(row); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(a.renderRoutineModal(), row) {
		t.Fatal("modal clipped or replaced the styled title viewport")
	}
	if input.Value() != a.routineUI.title.Value() || input.Position() != a.routineUI.title.Position() {
		t.Fatal("rendering changed input value or insertion position")
	}
	return row
}

func TestRoutineEditorFilledCursorTracksEditing(t *testing.T) {
	routineCursorTrueColor(t)
	a := routineFixture(false)
	a.openRoutineManager()
	a = routineKey(a, "enter")
	a.routineUI.title.SetValue("hello")
	assertCursor := func(wantValue string, wantPos int) {
		t.Helper()
		if a.routineUI.title.Value() != wantValue || a.routineUI.title.Position() != wantPos {
			t.Fatalf("input value=%q position=%d, want %q at %d", a.routineUI.title.Value(), a.routineUI.title.Position(), wantValue, wantPos)
		}
		assertRoutineTitleCursor(t, a, a.routineModalWidth()-4)
	}
	a = routineKey(a, "home")
	assertCursor("hello", 0)
	a = routineKey(a, "right")
	assertCursor("hello", 1)
	a = routineKey(a, "right")
	assertCursor("hello", 2) // highlight the first l; do not insert a cell before it
	a = routineKey(a, "X")
	assertCursor("heXllo", 3) // the same original l is still highlighted
	a = routineKey(a, "left")
	assertCursor("heXllo", 2)
	a = routineKey(a, "home")
	assertCursor("heXllo", 0)
	a = routineKey(a, "end")
	assertCursor("heXllo", 6)
	a = routineKey(a, "tab")
	row := renderRoutineTitle(a.routineUI.title, a.routineModalWidth()-4, false)
	cursorStyle := lipgloss.NewStyle().Foreground(currentTheme().AppTitleFg).Background(colorAccent)
	if a.routineUI.title.Focused() || strings.Contains(row, cursorStyle.Render(" ")) ||
		ansiRe.ReplaceAllString(row, "") != fitToWidth("  heXllo", a.routineModalWidth()-4) {
		t.Fatalf("blurred title shows a cursor or altered text: %q", row)
	}
	// Both the field focus and the widget focus must permit a cursor.
	if strings.Contains(renderRoutineTitle(a.routineUI.title, a.routineModalWidth()-4, true), cursorStyle.Render(" ")) {
		t.Fatal("blurred widget shows a cursor in a selected field")
	}
	a = routineKey(a, "shift+tab")
	assertCursor("heXllo", 6)
	if strings.Contains(renderRoutineTitle(a.routineUI.title, a.routineModalWidth()-4, false), cursorStyle.Render(" ")) {
		t.Fatal("unselected field shows a cursor in a focused widget")
	}
}

func TestRoutineEditorEmptyTitleHasFilledBlankCursor(t *testing.T) {
	routineCursorTrueColor(t)
	a := routineFixture(false)
	a.openRoutineManager()
	a = routineKey(a, "a")
	assertRoutineTitleCursor(t, a, a.routineModalWidth()-4)
	a = routineKey(a, "界")
	assertRoutineTitleCursor(t, a, a.routineModalWidth()-4)
	a = routineKey(a, "home")
	assertRoutineTitleCursor(t, a, a.routineModalWidth()-4)
}

func TestRoutineEditorUnicodeCursorAndLongTitleViewport(t *testing.T) {
	routineCursorTrueColor(t)
	for _, title := range []string{"茶é猫", strings.Repeat("日", 40), strings.Repeat("ab", 40)} {
		a := routineFixture(false)
		a.width, a.height = 32, 16
		a.openRoutineManager()
		a = routineKey(a, "enter")
		a.routineUI.title.SetValue(title)
		inside := a.routineModalWidth() - 4
		check := func() {
			t.Helper()
			assertRoutineTitleCursor(t, a, inside)
		}
		a = routineKey(a, "end")
		check()
		for range []rune(title) {
			a = routineKey(a, "left")
			check()
		}
		a = routineKey(a, "home")
		check()
		a = routineKey(a, "right")
		check()
		a = routineKey(a, "界")
		want := string([]rune(title)[:1]) + "界" + string([]rune(title)[1:])
		if a.routineUI.title.Value() != want {
			t.Fatalf("Unicode insertion = %q, want %q", a.routineUI.title.Value(), want)
		}
		check()
		for _, width := range []int{12, 50, 16, 32} {
			m, _ := a.Update(tea.WindowSizeMsg{Width: width, Height: 18})
			a = m.(App)
			inside = a.routineModalWidth() - 4
			check()
			a = routineKey(a, "end")
			check()
			for range []rune(a.routineUI.title.Value()) {
				a = routineKey(a, "left")
				check()
			}
		}
	}
}

func TestRoutineDeleteConfirmationLivesOnlyInModalFooter(t *testing.T) {
	for _, simple := range []bool{false, true} {
		a := routineFixture(simple)
		a.openRoutineManager()
		before := a.geometry()
		a = routineKey(a, "d")
		if a.geometry() != before {
			t.Fatal("opening routine confirmation changed page geometry")
		}
		modal := strings.Split(ansiRe.ReplaceAllString(a.renderRoutineModal(), ""), "\n")
		footer := strings.Join(modal[len(modal)-4:len(modal)-1], "\n")
		for _, want := range []string{`Delete "Coffee" and its history?`, "[y/enter] delete", "[any other key] cancel"} {
			if !strings.Contains(footer, want) {
				t.Fatalf("missing %q in modal footer:\n%s", want, footer)
			}
			if strings.Count(ansiRe.ReplaceAllString(a.View(), ""), want) != 1 {
				t.Fatalf("confirmation missing or duplicated in frame: %q", want)
			}
			if strings.Contains(ansiRe.ReplaceAllString(a.renderPage(), ""), want) {
				t.Fatalf("routine confirmation leaked to app bottom: %q", want)
			}
		}
		if strings.Contains(footer, "[a]") || strings.Contains(footer, "[esc] close") {
			t.Fatal("confirmation advertises inactive manager bindings")
		}
		cancelled := routineKey(a, "x")
		if cancelled.mode != modeRoutineManager || len(cancelled.routines) != 2 {
			t.Fatal("any-other-key cancel hint disagrees with handler")
		}
		deleted := routineKey(a, "enter")
		if deleted.mode != modeRoutineManager || len(deleted.routines) != 1 {
			t.Fatal("Enter confirmation hint disagrees with handler")
		}
	}
	for _, focus := range []focusedPane{focusToday, focusNotes} {
		a := routineFixture(false)
		a.routines = nil
		a.focus = focus
		a = routineKey(a, "d")
		page := ansiRe.ReplaceAllString(a.renderPage(), "")
		if !strings.Contains(page, "Delete ") || !strings.Contains(page, "[y]") {
			t.Fatalf("normal task/note confirmation lost its page prompt: %s", page)
		}
	}
	a := routineFixture(false)
	a.routines[0].Title = strings.Repeat("Long routine title ", 10)
	a.openRoutineManager()
	a = routineKey(a, "d")
	if !strings.Contains(ansiRe.ReplaceAllString(a.renderRoutineModal(), ""), "and its history?") {
		t.Fatal("long routine title hid the history deletion warning")
	}
}

func TestRoutineModalHierarchyAndBackgroundAcrossThemes(t *testing.T) {
	oldProfile, oldTheme := lipgloss.ColorProfile(), currentTheme()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer func() { lipgloss.SetColorProfile(oldProfile); applyTheme(oldTheme) }()
	for _, theme := range curatedThemes {
		applyTheme(theme)
		for _, size := range [][2]int{{32, 12}, {50, 16}, {80, 24}, {110, 36}} {
			a := routineFixture(false)
			a.width, a.height = size[0], size[1]
			a.routines[0].History = map[string]model.RoutineStatus{a.now().Format(dateFormat): model.RoutineCompleted}
			a.openRoutineManager()
			for _, mode := range []mode{modeRoutineManager, modeRoutineEditor, modeConfirmDelete} {
				b := a
				if mode == modeRoutineEditor {
					b = routineKey(b, "enter")
					b.routineUI.title.SetCursor(2)
					b.routineUI.schedule = model.ScheduleCustom
					b.routineUI.days[time.Monday] = true
				} else if mode == modeConfirmDelete {
					b = routineKey(b, "d")
				}
				modal := b.renderRoutineModal()
				if lipgloss.Height(modal) != b.routineModalHeight() {
					t.Fatalf("%s %v mode=%v: modal height mismatch", theme.Name, size, mode)
				}
				for _, line := range strings.Split(modal, "\n") {
					if lipgloss.Width(line) != b.routineModalWidth() {
						t.Fatalf("%s %v mode=%v: modal width mismatch", theme.Name, size, mode)
					}
				}
				if err := requireBackgroundEveryCell(modal); err != nil {
					t.Fatalf("%s %v mode=%v: %v", theme.Name, size, mode, err)
				}
				if mode == modeRoutineEditor {
					assertRoutineTitleCursor(t, b, b.routineModalWidth()-4)
				}
				// The wider case fits complete values, allowing checks of distinct
				// foreground roles without relying on exact RGB escape encodings.
				if size[0] >= 80 {
					if mode == modeRoutineEditor {
						text := lipgloss.NewStyle().Foreground(colorText).Background(colorPanel)
						if !strings.Contains(modal, text.Render("Co")) || !strings.Contains(modal, text.Render("fee")) {
							t.Fatal("title input does not use the value foreground")
						}
						label := lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Background(colorPaneBg)
						if !strings.Contains(modal, label.Render("Title")) {
							t.Fatal("editor label does not use the label foreground")
						}
					} else if !strings.Contains(modal, lipgloss.NewStyle().Foreground(colorGreen).Background(colorPanel).Render("completed")) {
						t.Fatal("manager completion status lost its distinct foreground")
					}
				}
			}
		}
	}
}

func TestRoutineEditorAvoidsRepeatedIDCollisions(t *testing.T) {
	a := routineFixture(false)
	base := strconv.FormatInt(a.now().UnixNano(), 36)
	for _, id := range []string{base, base + "-1", base + "-2"} {
		a.routines = append(a.routines, model.Routine{ID: id, Title: "Other", CreatedAt: a.now(), Schedule: model.ScheduleEveryDay})
	}
	a.openRoutineManager()
	a = routineKey(a, "a")
	a.routineUI.title.SetValue("New")
	a = routineKey(a, "ctrl+s")
	if a.mode != modeRoutineManager || a.routines[len(a.routines)-1].ID != base+"-3" {
		t.Fatalf("new ID not unique after collisions: mode=%v routines=%+v", a.mode, a.routines)
	}
}

func TestTodayScrollKeepsSelectedRowVisibleWithoutRoutines(t *testing.T) {
	a := routineFixture(false)
	a.routines = nil
	a.width, a.height = 100, 28
	for i := 0; i < 35; i++ {
		a.tasks = append(a.tasks, model.Task{
			ID: "task-" + string(rune('a'+i)), Title: "Row " + string(rune('a'+i)),
			Date: a.now().Format(dateFormat), CreatedAt: a.now(),
		})
	}
	a.clampSelections()
	for i := 0; i < 30; i++ {
		a = routineKey(a, "down")
	}
	if a.todaySelected < a.todayScroll || a.todaySelected >= a.todayScroll+a.visibleRowsFor(focusToday) {
		t.Fatalf("selected row %d outside visible range %d..%d", a.todaySelected, a.todayScroll, a.todayScroll+a.visibleRowsFor(focusToday))
	}
	body := ansiRe.ReplaceAllString(a.renderTodayList(a.visibleRowsFor(focusToday), a.geometry().taskWidth-4), "")
	if !strings.Contains(body, a.todayEntries()[a.todaySelected].task.Title) {
		t.Fatal("selected task not rendered after scrolling")
	}
}
