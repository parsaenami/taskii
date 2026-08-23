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

func TestThemePickerFlow(t *testing.T) {
	app := NewApp(Options{})
	app.width = 100
	app.height = 30
	app.now = func() time.Time { return time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC) }

	// Open theme picker
	m, _ := app.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}, Alt: true})
	a := m.(App)
	if a.mode != modeThemePicker {
		t.Fatalf("expected modeThemePicker, got %v", a.mode)
	}

	// Filter themes by "gruv"
	a.themeFilter.SetValue("gruv")
	a.filterThemes("gruv")
	if len(a.themeMatches) == 0 {
		t.Fatalf("expected matches for 'gruv', got 0")
	}

	// Navigate down
	origCursor := a.themeCursor
	m, _ = a.updateThemePicker(tea.KeyMsg{Type: tea.KeyDown})
	a = m.(App)
	if len(a.themeMatches) > 1 && a.themeCursor == origCursor && a.themeCursor < len(a.themeMatches)-1 {
		t.Errorf("expected themeCursor to move down")
	}

	// Render modal and check dimensions
	modal := a.renderThemePickerModal()
	if modal == "" {
		t.Errorf("renderThemePickerModal returned empty string")
	}
	modalLines := strings.Split(modal, "\n")
	if len(modalLines) != 16 {
		t.Errorf("expected 16 modal lines, got %d", len(modalLines))
	}
	for i, l := range modalLines {
		w := lipgloss.Width(l)
		if w != 56 {
			t.Errorf("modal line %d width = %d, expected 56: %q", i, w, l)
		}
	}

	view := a.View()
	lines := strings.Split(view, "\n")
	for i, l := range lines {
		w := lipgloss.Width(l)
		if w != a.width {
			t.Errorf("line %d width = %d, expected %d", i, w, a.width)
		}
	}

	// Cancel with esc
	m, _ = a.updateThemePicker(tea.KeyMsg{Type: tea.KeyEsc})
	a = m.(App)
	if a.mode != modeNormal {
		t.Errorf("expected modeNormal after Esc, got %v", a.mode)
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
