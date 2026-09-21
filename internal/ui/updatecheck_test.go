package ui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/parsaenami/taskii/internal/model"
	"github.com/parsaenami/taskii/internal/updatecheck"
)

func TestUpdateChecksDefaultOnWithPersistentDataAndOffInMock(t *testing.T) {
	isolateUICalendarData(t)
	if a := NewApp(Options{}); !a.checkForUpdates {
		t.Fatal("normal app should default to automatic update checks")
	}
	if a := NewApp(Options{Mock: true}); a.checkForUpdates {
		t.Fatal("mock app should not check for updates")
	}

	disabled := false
	if err := model.SaveSettings(model.Settings{CheckForUpdates: &disabled}); err != nil {
		t.Fatal(err)
	}
	if a := NewApp(Options{}); a.checkForUpdates {
		t.Fatal("saved update-check opt-out was ignored")
	}
}

func TestCurrentVersionAndUpdateCheckEligibility(t *testing.T) {
	original := Version
	t.Cleanup(func() { Version = original })

	Version = "v1.2.3"
	if got := CurrentVersion(); got != "1.2.3" {
		t.Fatalf("CurrentVersion() = %q, want 1.2.3", got)
	}
	a := NewApp(Options{})
	if !a.shouldCheckForUpdates() {
		t.Fatal("release build with checks enabled should check")
	}
	a.noPersist = true
	if a.shouldCheckForUpdates() {
		t.Fatal("non-persistent/mock app should not check")
	}

	Version = "dev"
	a.noPersist = false
	if a.shouldCheckForUpdates() {
		t.Fatal("development build should not check")
	}
}

func TestUpdateResultSuggestsNewReleaseWithoutSurfacingFailures(t *testing.T) {
	a := NewApp(Options{Mock: true})
	a.err = ""
	a.status = "Settings saved"

	updated, _ := a.Update(updateCheckMsg{result: updatecheck.Result{
		Current: "0.3.0", Latest: "0.4.0", URL: "https://example.test/v0.4.0", Available: true,
	}})
	a = updated.(App)
	if !a.updateChecked || !strings.Contains(a.status, "v0.3.0 → v0.4.0") || !strings.Contains(a.status, "https://example.test/v0.4.0") {
		t.Fatalf("update suggestion = checked:%v status:%q", a.updateChecked, a.status)
	}

	a.err, a.status = "", "keep this"
	updated, _ = a.Update(updateCheckMsg{err: errors.New("offline")})
	a = updated.(App)
	if a.err != "" || a.status != "keep this" {
		t.Fatalf("release-check failure leaked into UI: err=%q status=%q", a.err, a.status)
	}
}

func TestUpdateSettingUsesScratchStateAndRendersGuidance(t *testing.T) {
	a := NewApp(Options{})
	a.noPersist = true
	a.width, a.height = 100, 30
	original := a.checkForUpdates

	a.openSettings()
	a.settings.section = sectionUpdates
	a.settings.focus = focusSettingsContent
	m, _ := a.updateSettings(tea.KeyMsg{Type: tea.KeySpace})
	a = m.(App)
	if a.settings.checkForUpdates == original || a.checkForUpdates != original {
		t.Fatal("Updates toggle did not remain in modal scratch state")
	}
	m, _ = a.updateSettings(tea.KeyMsg{Type: tea.KeyEsc})
	a = m.(App)
	if a.checkForUpdates != original {
		t.Fatal("Esc did not discard update preference")
	}

	a.openSettings()
	a.settings.section = sectionUpdates
	a.settings.focus = focusSettingsContent
	m, _ = a.updateSettings(tea.KeyMsg{Type: tea.KeySpace})
	a = m.(App)
	m, _ = a.updateSettings(tea.KeyMsg{Type: tea.KeyCtrlS})
	a = m.(App)
	if a.mode != modeNormal || a.checkForUpdates == original {
		t.Fatalf("saved Updates setting = mode:%v enabled:%v", a.mode, a.checkForUpdates)
	}

	a.openSettings()
	a.settings.section = sectionUpdates
	plain := ansiRe.ReplaceAllString(strings.Join(a.renderSettingsUpdates(48), "\n"), "")
	for _, want := range []string{"Automatic checks", "Current", "24 hours", "brew upgrade parsaenami/tap/taskii", "github.com/parsaenami/taskii/releases/latest"} {
		if !strings.Contains(plain, want) {
			t.Errorf("Updates section missing %q\n%s", want, plain)
		}
	}
	assertSettingsModalGeometry(t, a)
}

func TestEnablingUpdateChecksStartsImmediateCheck(t *testing.T) {
	isolateUICalendarData(t)
	originalVersion := Version
	Version = "0.3.0"
	t.Cleanup(func() { Version = originalVersion })

	a := NewApp(Options{})
	a.checkForUpdates = false
	a.openSettings()
	a.settings.section = sectionUpdates
	a.settings.focus = focusSettingsContent
	m, _ := a.updateSettings(tea.KeyMsg{Type: tea.KeySpace})
	a = m.(App)
	m, cmd := a.updateSettings(tea.KeyMsg{Type: tea.KeyCtrlS})
	a = m.(App)
	if !a.checkForUpdates || a.mode != modeNormal || cmd == nil {
		t.Fatalf("enabling checks = enabled:%v mode:%v cmd:%v", a.checkForUpdates, a.mode, cmd != nil)
	}
}

func TestSaveSettingsPreservesUpdatePreference(t *testing.T) {
	isolateUICalendarData(t)
	a := NewApp(Options{})
	a.checkForUpdates = false
	if err := a.saveSettings(); err != nil {
		t.Fatal(err)
	}
	settings, err := model.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.CheckForUpdates == nil || *settings.CheckForUpdates {
		t.Fatalf("saved CheckForUpdates = %#v, want explicit false", settings.CheckForUpdates)
	}
}
