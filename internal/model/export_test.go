package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestExportDataCreatesPortableSnapshot(t *testing.T) {
	isolateXDG(t)
	now := time.Date(2026, time.September, 20, 10, 30, 0, 0, time.UTC)
	tasks := []Task{{ID: "task-1", Title: "Export me", CreatedAt: now}}
	notes := []Note{{ID: "note-1", Body: "Keep me", CreatedAt: now}}
	settings := Settings{Theme: "Nord", Layout: "stacked"}
	if err := Save(tasks); err != nil {
		t.Fatal(err)
	}
	if err := SaveNotes(notes); err != nil {
		t.Fatal(err)
	}
	if err := SaveSettings(settings); err != nil {
		t.Fatal(err)
	}

	parent := filepath.Join(t.TempDir(), "nested", "exports")
	path, err := ExportData(parent)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(parent, exportDirName); path != want {
		t.Fatalf("ExportData() path = %q, want %q", path, want)
	}

	var gotTasks []Task
	readExportJSON(t, filepath.Join(path, "tasks.json"), &gotTasks)
	if !reflect.DeepEqual(gotTasks, tasks) {
		t.Fatalf("exported tasks = %#v, want %#v", gotTasks, tasks)
	}
	var gotNotes []Note
	readExportJSON(t, filepath.Join(path, "notes.json"), &gotNotes)
	if !reflect.DeepEqual(gotNotes, notes) {
		t.Fatalf("exported notes = %#v, want %#v", gotNotes, notes)
	}
	var gotSettings Settings
	readExportJSON(t, filepath.Join(path, "settings.json"), &gotSettings)
	if !reflect.DeepEqual(gotSettings, settings) {
		t.Fatalf("exported settings = %#v, want %#v", gotSettings, settings)
	}
}

func TestExportDataIncludesEmptyFilesAndDoesNotOverwrite(t *testing.T) {
	isolateXDG(t)
	parent := t.TempDir()
	path, err := ExportData(parent)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"tasks.json", "notes.json", "settings.json"} {
		if _, err := os.Stat(filepath.Join(path, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	marker := filepath.Join(path, "marker")
	if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ExportData(parent); err == nil {
		t.Fatal("second export should refuse to overwrite taski_data")
	}
	if got, err := os.ReadFile(marker); err != nil || string(got) != "keep" {
		t.Fatalf("existing export was modified: %q, %v", got, err)
	}
}

func TestExportDataRejectsFileDestination(t *testing.T) {
	isolateXDG(t)
	path := filepath.Join(t.TempDir(), "export-file")
	if err := os.WriteFile(path, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ExportData(path); err == nil {
		t.Fatal("file destination should fail")
	}
}

func readExportJSON(t *testing.T, path string, dst any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}
