package model

import "testing"

func TestEffectiveCheckForUpdatesDefaultsOnAndPreservesOptOut(t *testing.T) {
	if !(Settings{}).EffectiveCheckForUpdates() {
		t.Fatal("omitted checkForUpdates should default to enabled")
	}
	disabled := false
	if (Settings{CheckForUpdates: &disabled}).EffectiveCheckForUpdates() {
		t.Fatal("explicit update-check opt-out was ignored")
	}
	enabled := true
	if !(Settings{CheckForUpdates: &enabled}).EffectiveCheckForUpdates() {
		t.Fatal("explicit update-check opt-in was ignored")
	}
}
