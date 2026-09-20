package ui

import (
	"strings"
	"testing"
)

func TestSettingsAboutIncludesProjectAndPrivacyDetails(t *testing.T) {
	a := NewApp(Options{})
	a.noPersist = true

	got := strings.Join(a.renderSettingsAbout(45), "\n")
	for _, want := range []string{
		"Created by Parsa Enami",
		"https://github.com/parsaenami/taskii",
		"Local JSON storage; no cloud or account",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("About content is missing %q", want)
		}
	}
}
