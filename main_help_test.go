package main

import (
	"bytes"
	"flag"
	"strings"
	"testing"
)

func TestHelpDocumentsDataImportAndExport(t *testing.T) {
	var help bytes.Buffer
	fs := flag.NewFlagSet("taskii", flag.ContinueOnError)
	fs.SetOutput(&help)
	setUsage(fs)
	registerFlags(fs)

	if err := fs.Parse([]string{"--help"}); err != flag.ErrHelp {
		t.Fatalf("Parse(--help) error = %v, want flag.ErrHelp", err)
	}

	got := help.String()
	for _, want := range []string{
		"Usage: taskii [options]",
		"-import-data string",
		"-export string",
		"merge tasks.json, notes.json, and settings.json from DIRECTORY",
		"write tasks.json, notes.json, and settings.json to DIRECTORY/taski_data",
		"taskii --import-data /path/to/data",
		"taskii --export /path/to/backup",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("help output is missing %q\noutput:\n%s", want, got)
		}
	}
}
