package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/adrg/xdg"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/parsaenami/taskii/internal/model"
)

func isolateUICalendarData(t *testing.T) {
	t.Helper()
	theme := currentTheme().Name
	t.Cleanup(func() {
		xdg.Reload()
		setThemeByName(theme)
	})
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	xdg.Reload()
}

func TestAppCalendarDefaultsAndLoadedSettings(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		isolateUICalendarData(t)
		a := NewApp(Options{})
		want := []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}
		if !reflect.DeepEqual(a.workdays, want) || a.weekStart != time.Monday {
			t.Fatalf("calendar defaults = %v/%v, want %v/Monday", a.workdays, a.weekStart, want)
		}
	})

	t.Run("loaded", func(t *testing.T) {
		isolateUICalendarData(t)
		start := time.Sunday
		settings := model.Settings{Theme: "Nord", Layout: layoutStacked.String(), Workdays: []time.Weekday{time.Sunday, time.Wednesday}, WeekStart: &start}
		if err := model.SaveSettings(settings); err != nil {
			t.Fatal(err)
		}
		a := NewApp(Options{})
		if !reflect.DeepEqual(a.workdays, settings.Workdays) || a.weekStart != start {
			t.Fatalf("loaded calendar = %v/%v, want %v/%v", a.workdays, a.weekStart, settings.Workdays, start)
		}
	})
}

func TestSaveSettingsPreservesCalendarAndExistingPreferences(t *testing.T) {
	isolateUICalendarData(t)
	a := NewApp(Options{})
	a.workdays = []time.Weekday{time.Sunday, time.Tuesday, time.Thursday}
	a.weekStart = time.Saturday
	a.layout = layoutThreeColumn
	a.pomo.workMinutes = 42
	a.pomo.autoStartNext = true
	setThemeByName("Nord")
	if err := a.saveSettings(); err != nil {
		t.Fatal(err)
	}

	got, err := model.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Workdays, a.workdays) || got.EffectiveWeekStart() != time.Saturday {
		t.Fatalf("saved calendar = %v/%v", got.Workdays, got.EffectiveWeekStart())
	}
	if got.Theme != "Nord" || got.Layout != layoutThreeColumn.String() || got.PomodoroFocusMinutes != 42 || !got.PomodoroAutoStartNext {
		t.Fatalf("saving calendar lost another setting: %+v", got)
	}
}

func TestCalendarSettingsScratchCancelAndValidation(t *testing.T) {
	a := NewApp(Options{Mock: true})
	a.width, a.height = 100, 30
	origDays := append([]time.Weekday(nil), a.workdays...)
	origStart := a.weekStart
	a.openSettings()
	a.settings.section = sectionCalendar
	a.settings.focus = focusSettingsContent

	// Week-start changes and workday toggles remain in the modal scratch copy.
	m, _ := a.updateSettings(tea.KeyMsg{Type: tea.KeySpace})
	a = m.(App)
	a.settings.calendarCursor = 1 // the newly selected week-start day
	m, _ = a.updateSettings(tea.KeyMsg{Type: tea.KeySpace})
	a = m.(App)
	if a.settings.weekStart == origStart || reflect.DeepEqual(a.settings.workdays, origDays) {
		t.Fatal("calendar scratch values did not change")
	}
	if a.weekStart != origStart || !reflect.DeepEqual(a.workdays, origDays) {
		t.Fatal("calendar edits leaked into live App before save")
	}

	m, _ = a.updateSettings(tea.KeyMsg{Type: tea.KeyEsc})
	a = m.(App)
	if a.weekStart != origStart || !reflect.DeepEqual(a.workdays, origDays) {
		t.Fatal("Esc failed to discard calendar changes")
	}

	// The final selected workday cannot be removed.
	a.openSettings()
	a.settings.section = sectionCalendar
	a.settings.focus = focusSettingsContent
	a.settings.workdays = []time.Weekday{time.Monday}
	a.settings.weekStart = time.Monday
	a.settings.calendarCursor = 1
	m, _ = a.updateSettings(tea.KeyMsg{Type: tea.KeySpace})
	a = m.(App)
	if !reflect.DeepEqual(a.settings.workdays, []time.Weekday{time.Monday}) || !strings.Contains(strings.ToLower(a.settings.calendarError), "at least one") {
		t.Fatalf("last workday toggle = %v, error %q", a.settings.workdays, a.settings.calendarError)
	}
}

func TestCalendarSettingsHorizontalArrowsDoNotEdit(t *testing.T) {
	a := NewApp(Options{Mock: true})
	a.openSettings()
	a.settings.section = sectionCalendar
	a.settings.focus = focusSettingsContent
	origStart := a.settings.weekStart
	origDays := append([]time.Weekday(nil), a.settings.workdays...)

	m, _ := a.updateSettings(tea.KeyMsg{Type: tea.KeyRight})
	a = m.(App)
	if a.settings.focus != focusSettingsContent {
		t.Fatal("right arrow unexpectedly left Calendar content")
	}
	if a.settings.weekStart != origStart || !reflect.DeepEqual(a.settings.workdays, origDays) {
		t.Fatalf("right arrow edited Calendar values: %v/%v", a.settings.weekStart, a.settings.workdays)
	}

	m, _ = a.updateSettings(tea.KeyMsg{Type: tea.KeyLeft})
	a = m.(App)
	if a.settings.focus != focusSettingsNav {
		t.Fatal("left arrow did not return to the Settings menu")
	}
	if a.settings.weekStart != origStart || !reflect.DeepEqual(a.settings.workdays, origDays) {
		t.Fatalf("left arrow edited Calendar values: %v/%v", a.settings.weekStart, a.settings.workdays)
	}
}

func TestCalendarCommitReconcilesWithOldWorkdays(t *testing.T) {
	isolateUICalendarData(t)
	now := time.Date(2026, time.September, 23, 10, 0, 0, 0, time.UTC) // Wednesday
	routine := model.Routine{
		ID: "r1", Title: "Work routine", CreatedAt: time.Date(2026, time.September, 21, 8, 0, 0, 0, time.UTC),
		Schedule: model.ScheduleWorkdays,
	}
	if err := model.SaveRoutines([]model.Routine{routine}); err != nil {
		t.Fatal(err)
	}

	a := NewApp(Options{})
	a.now = func() time.Time { return now }
	// Undo startup's real-clock reconciliation so this test controls the date.
	a.routines = []model.Routine{routine}
	a.workdays = []time.Weekday{time.Monday}
	a.weekStart = time.Monday
	a.openSettings()
	a.settings.section = sectionCalendar
	a.settings.focus = focusSettingsContent
	a.settings.workdays = []time.Weekday{time.Tuesday}
	a.settings.weekStart = time.Sunday

	m, _ := a.updateSettings(tea.KeyMsg{Type: tea.KeyCtrlS})
	a = m.(App)
	if a.mode != modeNormal || !reflect.DeepEqual(a.workdays, []time.Weekday{time.Tuesday}) || a.weekStart != time.Sunday {
		t.Fatalf("calendar commit failed: mode=%v days=%v start=%v err=%q", a.mode, a.workdays, a.weekStart, a.err)
	}
	if a.routines[0].History["2026-09-21"] != model.RoutineMissed {
		t.Fatalf("old Monday was not reconciled: %+v", a.routines[0])
	}
	if a.routines[0].History["2026-09-22"] != "" {
		t.Fatalf("new Tuesday workday rewrote old history: %+v", a.routines[0].History)
	}
	stored, err := model.LoadRoutines()
	if err != nil || stored[0].History["2026-09-21"] != model.RoutineMissed || stored[0].History["2026-09-22"] != "" {
		t.Fatalf("stored reconciliation = %+v, %v", stored, err)
	}
	settings, err := model.LoadSettings()
	if err != nil || !reflect.DeepEqual(settings.Workdays, []time.Weekday{time.Tuesday}) || settings.EffectiveWeekStart() != time.Sunday {
		t.Fatalf("stored calendar = %+v, %v", settings, err)
	}
}

func TestCalendarSettingsModalGeometryAndBackground(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(old)
	applyTheme(currentTheme())

	a := NewApp(Options{Mock: true})
	a.width, a.height = 100, 30
	a.openSettings()
	a.settings.section = sectionCalendar
	a.settings.focus = focusSettingsContent
	modal := a.renderSettingsModal()
	for i, line := range strings.Split(modal, "\n") {
		if got := lipgloss.Width(line); got != settingsModalWidth {
			t.Fatalf("modal line %d width = %d, want %d", i, got, settingsModalWidth)
		}
	}
	if !strings.Contains(modal, "\x1b[48;2;") {
		t.Fatal("Calendar modal contains no explicit background styling")
	}
	if !strings.Contains(ansiRe.ReplaceAllString(modal, ""), "Calendar") {
		t.Fatal("Calendar section missing from modal nav")
	}
}

func TestMockCalendarUsesDefaultsWithoutPersistence(t *testing.T) {
	isolateUICalendarData(t)
	a := NewApp(Options{Mock: true})
	if !a.noPersist || a.weekStart != time.Monday || len(a.workdays) != 5 || len(a.routines) != 3 {
		t.Fatalf("mock calendar state = noPersist:%v start:%v days:%v routines:%v", a.noPersist, a.weekStart, a.workdays, a.routines)
	}
	a.weekStart = time.Sunday
	if err := a.saveSettings(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "taskii", "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("mock wrote settings: %v", err)
	}
}
