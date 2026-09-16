package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestThemeRegistryInitialization(t *testing.T) {
	all := allAvailableThemes()
	if len(all) < 300 {
		t.Fatalf("expected at least 300 themes, got %d", len(all))
	}

	curatedCount := 0
	bubbletintCount := 0
	for _, th := range all {
		if th.Source == SourceCurated {
			curatedCount++
		} else if th.Source == SourceBubbletint {
			bubbletintCount++
		}
		if th.Name == "" {
			t.Errorf("found theme with empty name: %+v", th)
		}
		if th.Bg == "" || th.Text == "" || th.PaneBg == "" || th.Panel == "" {
			t.Errorf("theme %q missing essential colors", th.Name)
		}
		if len(th.HeatmapRamp) != 5 {
			t.Errorf("theme %q HeatmapRamp length = %d, expected 5", th.Name, len(th.HeatmapRamp))
		}
	}

	if curatedCount != 7 {
		t.Errorf("expected 7 curated themes, got %d", curatedCount)
	}
	if bubbletintCount < 300 {
		t.Errorf("expected > 300 bubbletint themes, got %d", bubbletintCount)
	}
}

func TestThemeCyclingForwardAndBackward(t *testing.T) {
	setDefaultTheme()
	start := currentTheme().Name
	if start != "Ember" {
		t.Fatalf("expected default theme Ember, got %s", start)
	}

	next := cycleTheme()
	if next == start {
		t.Errorf("expected cycleTheme to advance, got same theme %s", next)
	}

	prev := cycleThemePrev()
	if prev != start {
		t.Errorf("expected cycleThemePrev to return to %s, got %s", start, prev)
	}
}

func TestSetThemeByName(t *testing.T) {
	setThemeByName("Nord")
	if currentTheme().Name != "Nord" {
		t.Errorf("expected Nord, got %s", currentTheme().Name)
	}

	// Case-insensitive test on curated theme
	setThemeByName("tokyo night")
	if currentTheme().Name != "Tokyo Night" {
		t.Errorf("expected Tokyo Night, got %s", currentTheme().Name)
	}

	// Test a bubbletint theme
	setThemeByName("Catppuccin Mocha")
	if !strings.EqualFold(currentTheme().Name, "Catppuccin Mocha") {
		t.Errorf("expected Catppuccin Mocha, got %s", currentTheme().Name)
	}

	// Fallback test
	setThemeByName("non_existent_theme_xyz")
	if currentTheme().Name != defaultThemeName {
		t.Errorf("expected fallback to %s, got %s", defaultThemeName, currentTheme().Name)
	}
}

func TestSettingsThemeSectionFlow(t *testing.T) {
	app := NewApp(Options{})
	app.noPersist = true
	app.width = 100
	app.height = 30
	app.now = func() time.Time { return time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC) }

	// 'S' opens the settings modal even when notes are expanded; the Theme
	// section is then reached through the nav column.
	app.notesExpanded = true
	m, _ := app.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	a := m.(App)
	if a.mode != modeSettings {
		t.Fatalf("expected modeSettings when notes are expanded, got %v", a.mode)
	}
	a.settings.section = sectionTheme
	a.settings.focus = focusSettingsContent

	// Filter themes by "gruv"
	a.settings.themeFilter.SetValue("gruv")
	a.settings.filterThemes("gruv")
	if len(a.settings.themeMatches) == 0 {
		t.Fatalf("expected matches for 'gruv', got 0")
	}

	// Navigate down
	origCursor := a.settings.themeCursor
	m, _ = a.updateSettings(tea.KeyMsg{Type: tea.KeyDown})
	a = m.(App)
	if len(a.settings.themeMatches) > 1 && a.settings.themeCursor == origCursor {
		t.Errorf("expected themeCursor to move down")
	}

	assertSettingsModalGeometry(t, a)

	// Cancel with esc restores the pre-modal theme.
	m, _ = a.updateSettings(tea.KeyMsg{Type: tea.KeyEsc})
	a = m.(App)
	if a.mode != modeNormal {
		t.Errorf("expected modeNormal after Esc, got %v", a.mode)
	}
	if got := currentTheme().Name; got != a.settings.origTheme {
		t.Errorf("Esc left theme %q, expected restore to %q", got, a.settings.origTheme)
	}
}

func TestSettingsChoiceSurvivesClose(t *testing.T) {
	// Regression: choosing a layout/theme then closing the modal used to
	// roll the choice back, because Esc restored the values captured when
	// the modal OPENED. The choice persisted to disk but not in memory, so
	// it only appeared after a restart.
	t.Run("layout", func(t *testing.T) {
		app := NewApp(Options{})
		app.noPersist = true
		app.width, app.height = 100, 30
		app.layout = layoutTasksLeft

		app.openSettings()
		app.settings.section = sectionLayout
		app.settings.focus = focusSettingsContent

		// Move to another layout and choose it.
		m, _ := app.updateSettings(tea.KeyMsg{Type: tea.KeyDown})
		a := m.(App)
		want := allLayouts[a.settings.layoutCursor]
		m, _ = a.updateSettings(tea.KeyMsg{Type: tea.KeyEnter})
		a = m.(App)

		// Closing must keep it.
		m, _ = a.updateSettings(tea.KeyMsg{Type: tea.KeyEsc})
		a = m.(App)
		if a.layout != want {
			t.Errorf("layout after choose+esc = %v, expected %v", a.layout, want)
		}
	})

	t.Run("theme", func(t *testing.T) {
		app := NewApp(Options{})
		app.noPersist = true
		app.width, app.height = 100, 30

		app.openSettings()
		app.settings.section = sectionTheme
		app.settings.focus = focusSettingsContent

		m, _ := app.updateSettings(tea.KeyMsg{Type: tea.KeyDown})
		a := m.(App)
		want := a.settings.themeMatches[a.settings.themeCursor].Name
		m, _ = a.updateSettings(tea.KeyMsg{Type: tea.KeyEnter})
		a = m.(App)

		m, _ = a.updateSettings(tea.KeyMsg{Type: tea.KeyEsc})
		a = m.(App)
		if got := currentTheme().Name; got != want {
			t.Errorf("theme after choose+esc = %q, expected %q", got, want)
		}
	})

	t.Run("esc still discards unchosen previews", func(t *testing.T) {
		// The re-baselining must not defeat cancel: previews made after the
		// last explicit choice are still discarded.
		app := NewApp(Options{})
		app.noPersist = true
		app.width, app.height = 100, 30
		app.layout = layoutTasksLeft

		app.openSettings()
		app.settings.section = sectionLayout
		app.settings.focus = focusSettingsContent

		// Choose one layout...
		m, _ := app.updateSettings(tea.KeyMsg{Type: tea.KeyDown})
		a := m.(App)
		chosen := allLayouts[a.settings.layoutCursor]
		m, _ = a.updateSettings(tea.KeyMsg{Type: tea.KeyEnter})
		a = m.(App)

		// ...then preview a different one WITHOUT choosing it.
		m, _ = a.updateSettings(tea.KeyMsg{Type: tea.KeyDown})
		a = m.(App)
		if a.layout == chosen {
			t.Fatalf("preview did not move off the chosen layout")
		}

		m, _ = a.updateSettings(tea.KeyMsg{Type: tea.KeyEsc})
		a = m.(App)
		if a.layout != chosen {
			t.Errorf("esc after an unchosen preview = %v, expected the last chosen %v",
				a.layout, chosen)
		}
	})
}

// TestThemeLayoutShortcutsRemoved pins the decision that theme and layout are
// reachable ONLY through the Settings modal: the old top-level `t` (cycle
// theme), `T` (browse themes) and `L` (cycle layout) bindings must be inert
// on the main page, in simple mode, and while the Notes board is expanded.
func TestThemeLayoutShortcutsRemoved(t *testing.T) {
	press := func(a App, r rune) App {
		m, _ := a.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		return m.(App)
	}

	for _, tc := range []struct {
		name     string
		expanded bool
		simple   bool
	}{
		{"normal", false, false},
		{"notes expanded", true, false},
		{"simple mode", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := NewApp(Options{})
			base.noPersist = true
			base.width, base.height = 100, 30
			base.now = func() time.Time { return time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC) }
			base.notesExpanded = tc.expanded
			base.simple = tc.simple
			base.layout = layoutTasksLeft
			startTheme := currentTheme().Name

			for _, r := range []rune{'t', 'T', 'L'} {
				a := press(base, r)
				if a.mode != modeNormal {
					t.Errorf("%q opened mode %v, expected it to be inert", r, a.mode)
				}
				if a.layout != layoutTasksLeft {
					t.Errorf("%q changed layout to %v", r, a.layout)
				}
				if got := currentTheme().Name; got != startTheme {
					t.Errorf("%q changed theme to %q, expected %q", r, got, startTheme)
					setThemeByName(startTheme)
				}
			}
		})
	}
}

func TestSettingsSectionNavigation(t *testing.T) {
	app := NewApp(Options{})
	app.noPersist = true
	app.width = 100
	app.height = 30
	app.now = func() time.Time { return time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC) }

	app.openSettings()
	if app.settings.section != sectionPomodoro || app.settings.focus != focusSettingsNav {
		t.Fatalf("expected to open on Pomodoro nav, got section=%v focus=%v",
			app.settings.section, app.settings.focus)
	}

	// Every section renders at the modal's fixed geometry.
	for _, sec := range []settingsSection{sectionPomodoro, sectionLayout, sectionTheme, sectionAbout} {
		app.settings.section = sec
		assertSettingsModalGeometry(t, app)
	}

	// Down through the nav column reaches About and wraps.
	app.settings.section = sectionPomodoro
	a := app
	for i := 0; i < sectionCount; i++ {
		m, _ := a.updateSettings(tea.KeyMsg{Type: tea.KeyDown})
		a = m.(App)
	}
	if a.settings.section != sectionPomodoro {
		t.Errorf("expected nav to wrap back to Pomodoro, got %v", a.settings.section)
	}

	// About has no content cursor, so right/enter must not trap focus there.
	a.settings.section = sectionAbout
	m, _ := a.updateSettings(tea.KeyMsg{Type: tea.KeyRight})
	a = m.(App)
	if a.settings.focus != focusSettingsNav {
		t.Errorf("expected focus to stay in nav for About, got %v", a.settings.focus)
	}
}

func TestSettingsLayoutPreviewAndChoose(t *testing.T) {
	app := NewApp(Options{})
	app.noPersist = true
	app.width = 100
	app.height = 30
	app.now = func() time.Time { return time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC) }
	app.layout = layoutTasksLeft

	app.openSettings()
	app.settings.section = sectionLayout
	app.settings.focus = focusSettingsContent

	// Moving the cursor previews the layout live, without committing it.
	m, _ := app.updateSettings(tea.KeyMsg{Type: tea.KeyDown})
	a := m.(App)
	want := allLayouts[a.settings.layoutCursor]
	if a.layout != want {
		t.Errorf("expected live preview of %v, got %v", want, a.layout)
	}
	if a.settings.layoutChosen != layoutTasksLeft {
		t.Errorf("preview must not commit: chosen = %v", a.settings.layoutChosen)
	}

	// Enter commits it.
	m, _ = a.updateSettings(tea.KeyMsg{Type: tea.KeyEnter})
	a = m.(App)
	if a.settings.layoutChosen != want {
		t.Errorf("expected chosen = %v, got %v", want, a.settings.layoutChosen)
	}

	// Esc after a preview-only move restores the layout that was in effect
	// when the modal opened.
	b := app
	b.settings.focus = focusSettingsContent
	m, _ = b.updateSettings(tea.KeyMsg{Type: tea.KeyDown})
	b = m.(App)
	m, _ = b.updateSettings(tea.KeyMsg{Type: tea.KeyEsc})
	b = m.(App)
	if b.layout != layoutTasksLeft {
		t.Errorf("expected Esc to restore layoutTasksLeft, got %v", b.layout)
	}
}

// assertSettingsModalGeometry pins the modal to its fixed box: every line
// exactly settingsModalWidth cells wide, and the composed page still exactly
// the terminal width. Both have regressed before from off-by-one padding in
// the hand-built border.
func assertSettingsModalGeometry(t *testing.T, a App) {
	t.Helper()
	modal := a.renderSettingsModal()
	if modal == "" {
		t.Fatalf("renderSettingsModal returned empty string")
	}
	for i, l := range strings.Split(modal, "\n") {
		if w := lipgloss.Width(l); w != settingsModalWidth {
			t.Errorf("section %v: modal line %d width = %d, expected %d: %q",
				a.settings.section, i, w, settingsModalWidth, l)
		}
	}
	for i, l := range strings.Split(a.View(), "\n") {
		if w := lipgloss.Width(l); w != a.width {
			t.Errorf("section %v: view line %d width = %d, expected %d",
				a.settings.section, i, w, a.width)
		}
	}
}

func TestCustomThemeParsing(t *testing.T) {
	// 1. JSON with full taskii Theme
	jsonTheme := []byte(`{
		"name": "My Custom Theme",
		"bg": "#101010",
		"panel": "#202020",
		"border": "#303030",
		"border_focus": "#00ff00",
		"text": "#ffffff",
		"muted": "#888888",
		"accent": "#00ff00",
		"green": "#00ff00",
		"warning": "#ffff00",
		"danger": "#ff0000",
		"purple": "#ff00ff",
		"app_title_fg": "#000000",
		"pane_bg": "#151515"
	}`)

	th, ok := parseSingleCustomTheme(jsonTheme)
	if !ok {
		t.Fatalf("failed to parse custom Theme JSON")
	}
	if th.Name != "My Custom Theme" || th.Source != SourceCustom || th.Bg != "#101010" {
		t.Errorf("unexpected parsed custom theme: %+v", th)
	}

	// 2. JSON with bubbletint Tint format
	jsonTint := []byte(`{
		"display_name": "My Tint Theme",
		"id": "my_tint",
		"dark": true,
		"bg": "#121212",
		"fg": "#eeeeee",
		"red": "#ff5555",
		"green": "#50fa7b",
		"yellow": "#f1fa8c",
		"blue": "#bd93f9"
	}`)

	th2, ok2 := parseSingleCustomTheme(jsonTint)
	if !ok2 {
		t.Fatalf("failed to parse custom Tint JSON")
	}
	if th2.Name != "My Tint Theme" || th2.Source != SourceCustom || th2.Bg != "#121212" {
		t.Errorf("unexpected parsed custom tint: %+v", th2)
	}
}
