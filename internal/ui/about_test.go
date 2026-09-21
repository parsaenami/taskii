package ui

import (
	"strings"
	"testing"
)

func TestSettingsAboutIncludesProjectDetailsWithoutPrivacyLine(t *testing.T) {
	a := NewApp(Options{})
	a.noPersist = true

	got := strings.Join(a.renderSettingsAbout(45), "\n")
	for _, want := range []string{
		"Created by Parsa Enami",
		"https://github.com/parsaenami/taskii",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("About content is missing %q", want)
		}
	}
	if strings.Contains(got, "Local data") || strings.Contains(got, "release checks") {
		t.Error("About content still contains the removed privacy line")
	}
}
