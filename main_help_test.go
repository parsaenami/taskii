package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/parsaenami/taskii/internal/model"
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
		"merge tasks.json, notes.json, routines.json, and settings.json from DIRECTORY",
		"write tasks.json, notes.json, routines.json, and settings.json to DIRECTORY/taski_data",
		"taskii --import-data /path/to/data",
		"taskii --export /path/to/backup",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("help output is missing %q\noutput:\n%s", want, got)
		}
	}
}

func TestPrintImportReportIncludesRoutines(t *testing.T) {
	// printImportReport writes directly to stdout; pin its formatting through a
	// small pipe so routine counts cannot disappear while model imports remain.
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })

	printImportReport(model.ImportReport{RoutinesAdded: 3, RoutinesDuplicate: 2, RoutinesConflict: 1})
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out); !strings.Contains(got, "routines: added=3 duplicate=2 conflict=1") {
		t.Fatalf("import report missing routine counts:\n%s", got)
	}
}
