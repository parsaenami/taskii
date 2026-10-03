package ui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/parsaenami/taskii/internal/model"
)

var sketchLayouts = []layout{layoutDashboardGrid, layoutHeaderColumns}

func assertSketchFrame(t *testing.T, a App) {
	t.Helper()
	frame := a.View()
	if got := lipgloss.Height(frame); got != a.height {
		t.Fatalf("%s frame height = %d, want %d", a.layout, got, a.height)
	}
	for i, line := range strings.Split(frame, "\n") {
		if got := lipgloss.Width(line); got != a.width {
			t.Fatalf("%s row %d width = %d, want %d", a.layout, i, got, a.width)
		}
	}
	if err := requireBackgroundEveryCell(frame); err != nil {
		t.Fatalf("%s: %v", a.layout, err)
	}
}

func TestSketchLayoutPanePositions(t *testing.T) {
	for _, lay := range sketchLayouts {
		t.Run(lay.String(), func(t *testing.T) {
			a := timelineTestApp()
			a.layout, a.username = lay, "Ada"
			g := a.geometry()
			lines := strings.Split(ansiRe.ReplaceAllString(a.View(), ""), "\n")
			assertPane := func(title string, row, col int) {
				t.Helper()
				needle := "╭─ " + title
				if !strings.HasPrefix(string([]rune(lines[row])[col:]), needle) {
					t.Fatalf("%s missing at row %d col %d: %q", title, row, col, lines[row])
				}
			}
			assertBlank := func(row int) {
				t.Helper()
				if strings.TrimSpace(lines[row]) != "" {
					t.Fatalf("row %d should be a page-background spacer: %q", row, lines[row])
				}
			}
			first := g.headerHeight + 1
			assertBlank(g.headerHeight)
			if lay == layoutDashboardGrid {
				assertPane("Pomodoro", 0, g.infoWidth+1)
				assertPane("Reports", first, 0)
				assertPane("Today", first, g.infoWidth+1)
				second := first + g.todayHeight + 1
				assertBlank(second - 1)
				assertPane("Notes", second, 0)
				assertPane("Overdue", second, g.infoWidth+1)
				for _, row := range bigTimerRows("25:00") {
					if !strings.Contains(strings.Join(lines[:g.headerHeight], "\n"), row) {
						t.Fatal("Layout 5 should retain the block countdown")
					}
				}
			} else {
				assertPane("Reports", first, 0)
				assertPane("Today", first, g.infoWidth+1)
				assertPane("Notes", first, g.infoWidth+g.taskWidth+2)
				second := first + g.todayHeight + 1
				assertPane("Pomodoro", second, 0)
				assertPane("Overdue", second, g.infoWidth+1)
				if strings.TrimSpace(string([]rune(lines[second-1])[:g.infoWidth+g.taskWidth+1])) != "" {
					t.Fatal("left and middle columns should have a blank row between their panes")
				}
				if !strings.HasSuffix(lines[first+g.notesHeight-1], "╯") {
					t.Fatal("Notes should span both rows including their spacer")
				}
				for _, row := range bigTimerRows("25:00") {
					if !strings.Contains(strings.Join(lines[second:], "\n"), row) {
						t.Fatal("Header Columns should retain the full block countdown")
					}
				}
			}
			// Both sketches keep the approved full wordmark at typical size.
			for _, row := range taskiiBanner {
				if !strings.Contains(strings.Join(lines[:g.headerHeight], "\n"), row) {
					t.Fatalf("header lost approved banner row %q", row)
				}
			}
		})
	}
}

func TestSketchHeaderFittingAndBackground(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(old)
	applyTheme(currentTheme())
	now := timelineTestApp().now()
	for _, width := range []int{10, 34, 79, 160} {
		for _, right := range []bool{false, true} {
			header := renderHeader(now, "Ada 界", width, 5, right)
			if err := requireBackgroundEveryCell(header); err != nil {
				t.Fatal(err)
			}
			if lipgloss.Height(header) != 5 {
				t.Fatal("header must honor its height budget")
			}
			for _, line := range strings.Split(header, "\n") {
				if lipgloss.Width(line) != width {
					t.Fatalf("header line width = %d, want %d", lipgloss.Width(line), width)
				}
				plainLine := ansiRe.ReplaceAllString(line, "")
				if !strings.HasPrefix(plainLine, " ") || !strings.HasSuffix(plainLine, " ") {
					t.Fatalf("header lost a one-cell side margin: %q", plainLine)
				}
			}
			plain := ansiRe.ReplaceAllString(header, "")
			if width == 34 && (!strings.Contains(plain, "TASKII") || !strings.Contains(plain, "v"+CurrentVersion())) {
				t.Fatal("narrow header should preserve plain TASKII and version")
			}
			if width >= 79 {
				for _, text := range []string{taskiiBanner[0], "Good afternoon, Ada 界", now.Format("Monday, January 2, 2006"), "v" + CurrentVersion()} {
					if !strings.Contains(plain, text) {
						t.Fatalf("wide header missing %q", text)
					}
				}
				if right {
					for _, line := range strings.Split(plain, "\n")[1:4] {
						if !strings.HasSuffix(line, " ") || strings.HasSuffix(line, "  ") {
							t.Fatal("full-width header metadata should stop exactly one cell before the far right")
						}
					}
				}
			}
		}
	}
}

func TestSketchLayoutsSettingsRoundTrip(t *testing.T) {
	for _, lay := range sketchLayouts {
		t.Run(lay.String(), func(t *testing.T) {
			isolateUICalendarData(t)
			a := NewApp(Options{})
			a.width, a.height, a.layout = 160, 45, layoutTasksLeft
			a.pomo.workMinutes, a.pomo.autoStartNext = 42, true
			a.workdays, a.weekStart = []time.Weekday{time.Sunday, time.Tuesday}, time.Saturday
			a.checkForUpdates = false
			setThemeByName("Nord")
			if err := a.saveSettings(); err != nil {
				t.Fatal(err)
			}
			preview := func(a App) App {
				a.openSettingsAt(sectionLayout)
				for a.layout != lay {
					updated, _ := a.updateSettings(tea.KeyMsg{Type: tea.KeyDown})
					a = updated.(App)
				}
				return a
			}
			cancelled := preview(a)
			if cancelled.settings.layoutChosen != layoutTasksLeft {
				t.Fatal("preview should not commit the new layout")
			}
			updated, _ := cancelled.updateSettings(tea.KeyMsg{Type: tea.KeyEsc})
			if updated.(App).layout != layoutTasksLeft {
				t.Fatal("Esc should restore the old layout")
			}
			saved, err := model.LoadSettings()
			if err != nil || saved.Layout != "Layout 1" {
				t.Fatalf("preview changed persisted layout: %+v, %v", saved, err)
			}

			a = preview(a)
			assertSettingsModalGeometry(t, a)
			updated, _ = a.updateSettings(tea.KeyMsg{Type: tea.KeyEnter})
			a = updated.(App)
			if a.status != "Layout: "+lay.String() {
				t.Fatalf("selection status = %q", a.status)
			}
			updated, _ = a.updateSettings(tea.KeyMsg{Type: tea.KeyDown})
			a = updated.(App)
			updated, _ = a.updateSettings(tea.KeyMsg{Type: tea.KeyEsc})
			a = updated.(App)
			if a.layout != lay {
				t.Fatal("Esc after another preview should retain the explicit choice")
			}
			saved, err = model.LoadSettings()
			if err != nil || saved.Layout != lay.String() || saved.Theme != "Nord" || saved.PomodoroFocusMinutes != 42 ||
				!saved.PomodoroAutoStartNext || saved.EffectiveWeekStart() != time.Saturday ||
				!reflect.DeepEqual(saved.Workdays, a.workdays) || saved.EffectiveCheckForUpdates() {
				t.Fatalf("layout choice lost complete settings: %+v, %v", saved, err)
			}
			if restored := NewApp(Options{}); restored.layout != lay {
				t.Fatalf("saved name %q did not restore layout", saved.Layout)
			}
		})
	}
	for i, lay := range allLayouts {
		if lay.next() != allLayouts[(i+1)%len(allLayouts)] || layoutByName(lay.String()) != lay {
			t.Fatalf("layout %s does not cycle or resolve by its stable name", lay)
		}
	}
}

func TestNumberedLayoutsAndLegacyPreferences(t *testing.T) {
	legacy := []string{"Tasks Left", "Tasks Right", "Stacked", "Three Column", "Dashboard Grid", "Header Columns"}
	a := timelineTestApp()
	a.openSettingsAt(sectionLayout)
	rows := a.renderSettingsLayout(40)
	if len(rows) != 6 {
		t.Fatalf("Settings lists %d layouts, want 6", len(rows))
	}
	for i, oldName := range legacy {
		t.Run(oldName, func(t *testing.T) {
			isolateUICalendarData(t)
			name := fmt.Sprintf("Layout %d", i+1)
			lay := allLayouts[i]
			if lay.String() != name || layoutByName(name) != lay || layoutByName(oldName) != lay {
				t.Fatalf("numbered/legacy name did not resolve to %s", name)
			}
			if !strings.Contains(ansiRe.ReplaceAllString(rows[i], ""), name) {
				t.Fatalf("Settings row missing %s: %q", name, rows[i])
			}
			if err := model.SaveSettings(model.Settings{Layout: oldName}); err != nil {
				t.Fatal(err)
			}
			restored := NewApp(Options{})
			if restored.layout != lay {
				t.Fatalf("legacy saved name %q restored %s", oldName, restored.layout)
			}
			if err := restored.saveSettings(); err != nil {
				t.Fatal(err)
			}
			saved, err := model.LoadSettings()
			if err != nil || saved.Layout != name {
				t.Fatalf("saving legacy preference did not use %s: %+v, %v", name, saved, err)
			}
		})
	}
	if layoutByName("unknown") != layoutTasksLeft || layout(-1).String() != "Layout 1" {
		t.Fatal("unknown layouts should default to Layout 1")
	}
}

func TestLayout5ColumnProportions(t *testing.T) {
	for _, width := range []int{70, 71, 99, 100, 101, 159, 160, 161, 220} {
		a := timelineTestApp()
		a.layout, a.width = layoutDashboardGrid, width
		g := a.geometry()
		left := (width - 1) * 2 / 5
		if g.infoWidth != left || g.headerWidth != left || g.notesWidth != left || g.taskWidth != width-left-1 {
			t.Fatalf("%d-column Layout 5 has incorrect left/right split: %+v", width, g)
		}
		// Check rendered boxes too: a stale input/render width must not displace
		// the right column from its geometry-defined start.
		lines := strings.Split(ansiRe.ReplaceAllString(a.View(), ""), "\n")
		for _, row := range []int{0, g.headerHeight + g.headerGap, g.headerHeight + g.headerGap + g.todayHeight + g.rowGap} {
			if !strings.HasPrefix(string([]rune(lines[row])[left+1:]), "╭─ ") {
				t.Fatalf("right pane displaced at width %d row %d: %q", width, row, lines[row])
			}
		}
	}
}

func TestLayout5HorizontalPomodoro(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(old)
	applyTheme(currentTheme())
	for _, size := range [][2]int{{70, 24}, {100, 24}, {160, 24}, {160, 45}, {220, 45}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			a := timelineTestApp()
			a.layout, a.width, a.height = layoutDashboardGrid, size[0], size[1]
			a.pomo.remaining, a.pomo.completed = 13*time.Minute+42*time.Second, 2
			g := a.geometry()
			width, height := g.taskWidth-4, g.pomoHeight-2
			if height < 3 {
				t.Fatalf("short-screen timer lost its three digit rows: %+v", g)
			}
			body := renderHorizontalPomodoro(a.pomo, width, height)
			if lipgloss.Height(body) != height {
				t.Fatalf("timer height = %d, want %d", lipgloss.Height(body), height)
			}
			if err := requireBackgroundEveryCell(body); err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(ansiRe.ReplaceAllString(body, ""), "\n")
			contextRows := min(4, height)
			top := (height - max(3, contextRows) + 1) / 2
			phaseRow := top
			if top > 0 {
				phaseRow--
			}
			for i, row := range bigTimerRows("13:42") {
				if !strings.HasPrefix(lines[top+i], row) || lipgloss.Width(lines[top+i]) != width {
					t.Fatalf("left block-countdown row lost or mis-sized: %q", lines[top+i])
				}
			}
			phaseCol := strings.Index(lines[phaseRow], "Focus")
			if phaseCol < 0 {
				t.Fatalf("phase missing from context: %q", lines[phaseRow])
			}
			contextCol := lipgloss.Width(lines[phaseRow][:phaseCol]) - 2 // run pip precedes phase
			if top > 0 {
				if strings.TrimSpace(string([]rune(lines[phaseRow])[:contextCol])) != "" {
					t.Fatal("spare timer row should retain background filler above the clock and bar")
				}
				if strings.TrimSpace(string([]rune(lines[top])[contextCol:])) != "" {
					t.Fatal("old phase position should be background filler")
				}
			}
			sessionRow := top + 1
			if height == 3 {
				sessionRow = top
			}
			if contextCol <= 19 || !strings.Contains(lines[sessionRow], "Long next") &&
				!strings.Contains(lines[sessionRow], "Long break next") && !strings.Contains(lines[sessionRow], "L→") {
				t.Fatalf("phase/session context missing beside clock: %q", lines[top:top+contextRows])
			}
			if !strings.Contains(lines[sessionRow], "2") {
				t.Fatal("session count missing")
			}
			keysRow := top + contextRows - 1
			if strings.TrimSpace(string([]rune(lines[keysRow-1])[contextCol:])) != "" {
				t.Fatal("session context should have a blank row before controls")
			}
			for _, key := range []string{"[p]", "[r]", "[n]"} {
				if !strings.Contains(lines[keysRow], key) {
					t.Fatalf("context controls lost %s: %q", key, lines[keysRow])
				}
			}
			if contextRows == 4 && strings.TrimSpace(string([]rune(lines[keysRow])[:contextCol])) != "" {
				t.Fatal("fourth row's clock and bar columns should be background filler")
			}
			if size[0] >= 100 {
				middle := string([]rune(lines[top+1])[21 : contextCol-2])
				if !strings.Contains(middle, "█") || !strings.Contains(middle, "░") {
					t.Fatalf("horizontal progress missing between clock and context: %q", middle)
				}
				for _, row := range []int{top, top + 2} {
					if strings.TrimSpace(string([]rune(lines[row])[21:contextCol-2])) != "" {
						t.Fatal("bar's upper and lower rows should be background filler")
					}
				}
			}
			assertSketchFrame(t, a)
		})
	}
}

func TestLayout5PomodoroTickAndControls(t *testing.T) {
	a := timelineTestApp()
	a.layout = layoutDashboardGrid
	a = timelineKey(a, "p")
	updated, _ := a.Update(pomodoroTickMsg(a.now()))
	a = updated.(App)
	if !a.pomo.running || a.pomo.remaining != 25*time.Minute-time.Second {
		t.Fatal("running horizontal timer did not tick")
	}
	for _, row := range bigTimerRows("24:59") {
		if !strings.Contains(ansiRe.ReplaceAllString(a.View(), ""), row) {
			t.Fatal("running tick did not refresh the block countdown")
		}
	}
	a = timelineKey(a, "r")
	if a.pomo.remaining != 25*time.Minute || a.pomo.running {
		t.Fatal("reset should pause the timer and restore phase duration")
	}
	a = timelineKey(a, "n")
	plain := ansiRe.ReplaceAllString(a.View(), "")
	if a.pomo.phase != phaseShortBreak || a.pomo.completed != 1 || !strings.Contains(plain, "Short Break") ||
		!strings.Contains(plain, "Sessions 1") || !strings.Contains(plain, "Short break") || strings.Contains(plain, "break next") {
		t.Fatal("skip should advance shared phase/session state and show the current break")
	}
	// Configured three-digit minute counts retain their approved glyphs even
	// in the short, narrow header, without losing the controls.
	a.width, a.height, a.pomo.phase = 70, 24, phaseWork
	a.pomo.workMinutes = 180
	a.pomo.reset()
	plain = ansiRe.ReplaceAllString(a.View(), "")
	for _, row := range bigTimerRows("180:00") {
		if !strings.Contains(plain, row) {
			t.Fatal("configured duration lost its wide countdown")
		}
	}
	for _, key := range []string{"[p]", "[r]", "[n]"} {
		if !strings.Contains(plain, key) {
			t.Fatalf("configured wide countdown lost %s", key)
		}
	}
}

func TestSketchLayoutsScrollingResizeAndEditors(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(old)
	applyTheme(currentTheme())
	for _, lay := range sketchLayouts {
		t.Run(lay.String(), func(t *testing.T) {
			a := timelineTestApp()
			a.layout = lay
			for i := 0; i < 30; i++ {
				a.tasks = append(a.tasks, model.Task{ID: fmt.Sprint(i), Title: fmt.Sprintf("T%02d", i), Date: "2026-09-21"})
			}
			for i := 0; i < 12; i++ {
				a.notes = append(a.notes, model.Note{ID: fmt.Sprint(i), Body: fmt.Sprintf("N%02d a wrapped note with extra words and a wide character 界", i)})
			}
			a.todaySelected, a.notesSelected = 29, 11
			for _, size := range [][2]int{{70, 24}, {160, 24}, {100, 30}, {160, 45}} {
				updated, _ := a.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
				a = updated.(App)
				a.focus = focusToday
				a.syncScroll()
				if !strings.Contains(ansiRe.ReplaceAllString(a.View(), ""), "T29") {
					t.Fatalf("selected task lost on resize to %v", size)
				}
				a.focus = focusNotes
				a.syncScroll()
				if !strings.Contains(ansiRe.ReplaceAllString(a.View(), ""), "N11") {
					t.Fatalf("selected wrapped note lost on resize to %v", size)
				}
				assertSketchFrame(t, a)
				beforeEdit := a.geometry()
				updated, _ = a.startNoteEdit(11)
				a = updated.(App)
				if a.geometry() != beforeEdit {
					t.Fatal("opening Notes editor should not change layout geometry")
				}
				a.noteInput.SetValue("edited 界\nsecond\nthird")
				a.noteInput.CursorStart()
				a.syncScroll()
				assertSketchFrame(t, a)
				if !strings.Contains(ansiRe.ReplaceAllString(a.View(), ""), "edited") {
					t.Fatalf("note editor is hidden at %v", size)
				}
				for _, text := range []string{"second", "third"} {
					if !strings.Contains(ansiRe.ReplaceAllString(a.View(), ""), text) {
						t.Fatalf("note editor line %q is hidden at %v", text, size)
					}
				}
				a = timelineKey(a, "enter")
				if a.notes[11].Body != "edited 界\nsecond\nthird" {
					t.Fatal("note editor failed to save")
				}
				// Restore a wrapped note for the next resize.
				a.notes[11].Body = "N11 a wrapped note with extra words and a wide character 界"
			}

			a.focus = focusToday
			a.tasks = []model.Task{{ID: "appointment", Title: "Standup", Kind: model.KindAppointment, Date: "2026-09-21", Time: "12:30"}}
			a.setTimeline(true)
			before := ansiRe.ReplaceAllString(a.View(), "")
			rows := a.visibleRowsFor(focusToday)
			a = timelineKey(a, "a")
			if rows != a.visibleRowsFor(focusToday) {
				t.Fatal("Timeline input changed its reserved viewport")
			}
			after := ansiRe.ReplaceAllString(a.View(), "")
			for _, needle := range []string{"NOW", "Standup"} {
				if strings.Index(before, needle) != strings.Index(after, needle) || !strings.Contains(after, needle) {
					t.Fatalf("Timeline %s moved when input opened", needle)
				}
			}
			assertSketchFrame(t, a)
			a = timelineKey(a, "esc")
			a.focus = focusNotes
			a = timelineKey(a, "e")
			assertSketchFrame(t, a)
			if !a.notesExpanded || strings.Contains(ansiRe.ReplaceAllString(a.View(), ""), "Pomodoro") {
				t.Fatal("expanded Notes should replace the header and panes")
			}
		})
	}
}

func TestSketchLayoutFramesAcrossThemesAndModes(t *testing.T) {
	oldProfile, oldTheme := lipgloss.ColorProfile(), currentTheme()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer func() {
		lipgloss.SetColorProfile(oldProfile)
		applyTheme(oldTheme)
	}()
	for _, theme := range curatedThemes {
		applyTheme(theme)
		for _, lay := range sketchLayouts {
			for _, size := range [][2]int{{70, 24}, {100, 24}, {101, 30}, {160, 45}, {220, 45}} {
				for _, mode := range []string{"today", "timeline", "task input", "notes edit", "confirm", "settings", "shortcuts"} {
					t.Run(fmt.Sprintf("%s/%s/%v/%s", theme.Name, lay, size, mode), func(t *testing.T) {
						a := timelineTestApp()
						a.layout, a.width, a.height = lay, size[0], size[1]
						a.tasks = []model.Task{{ID: "task", Title: "Appointment 界", Kind: model.KindAppointment, Date: "2026-09-21", Time: "12:30"}}
						a.notes = []model.Note{{ID: "note", Body: "Wrapped note with several words and wide text 界"}}
						switch mode {
						case "timeline":
							a.setTimeline(true)
						case "task input":
							a = timelineKey(a, "a")
							a.input.SetValue("A task 界")
						case "notes edit":
							a.focus = focusNotes
							updated, _ := a.startNoteEdit(0)
							a = updated.(App)
						case "confirm":
							a = timelineKey(a, "d")
						case "settings":
							a.openSettingsAt(sectionLayout)
						case "shortcuts":
							a.shortcutsOpen = true
						}
						assertSketchFrame(t, a)
					})
				}
			}
		}
	}
}

func TestModalSlicesWideGlyphAtEitherEdge(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(old)
	line := lipgloss.NewStyle().Background(colorBg).Render("a界b")
	for _, span := range [][2]int{{0, 2}, {2, 4}, {1, 3}} {
		slice := ansiSlice(line, span[0], span[1])
		if got := lipgloss.Width(slice); got != span[1]-span[0] {
			t.Fatalf("slice %v width = %d, want %d", span, got, span[1]-span[0])
		}
		if err := requireBackgroundEveryCell(slice); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSketchLayoutsKeepRoutineTaskInputAndTimerControlsVisible(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(old)
	applyTheme(currentTheme())
	for _, lay := range sketchLayouts {
		for _, width := range []int{70, 160} {
			t.Run(fmt.Sprintf("%s/%dx24", lay, width), func(t *testing.T) {
				a := routineFixture(false)
				a.layout, a.width, a.height = lay, width, 24
				g := a.geometry()
				a = routineKey(a, "a")
				a = routineKey(a, "visible input")
				if a.geometry() != g {
					t.Fatal("opening input should not change layout geometry")
				}
				assertSketchFrame(t, a)
				plain := ansiRe.ReplaceAllString(a.View(), "")
				lines := strings.Split(plain, "\n")
				var todayRows []string
				for _, line := range lines[g.headerHeight+g.headerGap+1 : g.headerHeight+g.headerGap+g.todayHeight-1] {
					col := g.infoWidth + 1
					todayRows = append(todayRows, string([]rune(line)[col:col+g.taskWidth]))
				}
				if !strings.Contains(strings.Join(todayRows, "\n"), "visible input") {
					t.Fatalf("Today input with due routines was truncated:\n%s", plain)
				}
				for _, key := range []string{"[p]", "[r]", "[n]"} {
					if !strings.Contains(plain, key) {
						t.Fatalf("short Pomodoro lost %s", key)
					}
				}
				if lay == layoutHeaderColumns && a.geometry().pomoHeight < pomoMinContentLines+2 {
					if !strings.Contains(plain, "25:00") {
						t.Fatal("constrained Pomodoro should retain a plain countdown")
					}
				} else {
					for _, row := range bigTimerRows("25:00") {
						if !strings.Contains(plain, row) {
							t.Fatal("full Pomodoro should retain the block countdown")
						}
					}
				}
			})
		}
	}
}
