package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func writeJSON(t *testing.T, path string, value any) []byte {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestMigrateLegacyDataIndependentlyAndPreservesSources(t *testing.T) {
	dataHome, configHome := isolateXDG(t)
	legacy := t.TempDir()
	changeDir(t, t.TempDir())
	if err := os.Mkdir(filepath.Join(mustGetwd(t), "data"), 0o755); err != nil {
		t.Fatal(err)
	}

	tasks := []Task{{ID: "t1", Title: "legacy"}}
	notes := []Note{{ID: "n1", Body: "legacy note"}}
	taskBytes := writeJSON(t, filepath.Join(mustGetwd(t), "data", "tasks.json"), tasks)
	writeJSON(t, filepath.Join(mustGetwd(t), "data", "notes.json"), notes)
	writeJSON(t, filepath.Join(mustGetwd(t), "data", "settings.json"), Settings{Theme: "Nord"})
	// Keep a second directory around to make sure migration is tied to cwd.
	writeJSON(t, filepath.Join(legacy, "tasks.json"), []Task{{ID: "wrong"}})

	report := MigrateLegacyData()
	if len(report.Errors) != 0 {
		t.Fatalf("unexpected migration errors: %v", report.Errors)
	}
	if got, err := Load(); err != nil || !reflect.DeepEqual(got, tasks) {
		t.Fatalf("migrated tasks = %#v, %v", got, err)
	}
	if got, err := LoadNotes(); err != nil || !reflect.DeepEqual(got, notes) {
		t.Fatalf("migrated notes = %#v, %v", got, err)
	}
	if got, err := LoadSettings(); err != nil || got.Theme != "Nord" {
		t.Fatalf("migrated settings = %#v, %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(mustGetwd(t), "data", "tasks.json")); err != nil || !reflect.DeepEqual(got, taskBytes) {
		t.Fatalf("legacy source changed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataHome, appDataDir, "tasks.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(configHome, appDataDir, "settings.json")); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateLegacyDestinationWinsAndMalformedFileDoesNotBlockOthers(t *testing.T) {
	isolateXDG(t)
	legacy := t.TempDir()
	changeDir(t, t.TempDir())
	dataDir := filepath.Join(mustGetwd(t), "data")
	if err := os.Mkdir(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(dataDir, "tasks.json"), []Task{{ID: "legacy"}})
	writeJSON(t, filepath.Join(dataDir, "settings.json"), Settings{Theme: "Ember"})
	if err := os.WriteFile(filepath.Join(dataDir, "notes.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(legacy, "unused"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Save([]Task{{ID: "existing"}}); err != nil {
		t.Fatal(err)
	}

	report := MigrateLegacyData()
	if len(report.Errors) != 1 || !strings.Contains(report.Errors[0].Error(), "notes.json") {
		t.Fatalf("errors = %v, want only malformed notes", report.Errors)
	}
	got, err := Load()
	if err != nil || len(got) != 1 || got[0].ID != "existing" {
		t.Fatalf("destination precedence failed: %#v, %v", got, err)
	}
	if got, err := LoadSettings(); err != nil || got.Theme != "Ember" {
		t.Fatalf("independent settings migration failed: %#v, %v", got, err)
	}
}

func TestMigrateLegacyRejectsWrongSchemaWithoutBlockingOthers(t *testing.T) {
	isolateXDG(t)
	changeDir(t, t.TempDir())
	dataDir := filepath.Join(mustGetwd(t), "data")
	if err := os.Mkdir(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(dataDir, "tasks.json"), []string{"not a task"})
	writeJSON(t, filepath.Join(dataDir, "notes.json"), []Note{{ID: "valid-note"}})

	report := MigrateLegacyData()
	if len(report.Errors) != 1 || !strings.Contains(report.Errors[0].Error(), "tasks.json") {
		t.Fatalf("errors = %v, want only invalid task schema", report.Errors)
	}
	if got, err := Load(); err != nil || len(got) != 0 {
		t.Fatalf("invalid tasks must not migrate: %#v, %v", got, err)
	}
	if got, err := LoadNotes(); err != nil || len(got) != 1 || got[0].ID != "valid-note" {
		t.Fatalf("valid notes should migrate independently: %#v, %v", got, err)
	}
}

func TestImportDataMergesStableIDsAndIsIdempotent(t *testing.T) {
	isolateXDG(t)
	tasks := []Task{{ID: "same", Title: "existing"}, {ID: "new", Title: "new"}}
	if err := Save(tasks[:1]); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	notes := []Note{{ID: "same-note", Body: "old", CreatedAt: now}}
	if err := SaveNotes(notes); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	writeJSON(t, filepath.Join(source, "tasks.json"), []Task{
		{ID: "same", Title: "existing"}, {ID: "same", Title: "conflict"}, {ID: "new", Title: "new"}, {ID: "added", Title: "added"},
	})
	writeJSON(t, filepath.Join(source, "notes.json"), []Note{
		{ID: "same-note", Body: "old", CreatedAt: now}, {ID: "added-note", Body: "added", CreatedAt: now},
	})
	writeJSON(t, filepath.Join(source, "settings.json"), Settings{Theme: "Nord"})

	report, err := ImportData(source)
	if err != nil || len(report.Errors) != 0 {
		t.Fatalf("import failed: %v, %v", err, report.Errors)
	}
	if report.TasksAdded != 2 || report.TasksDuplicate != 1 || report.TasksConflict != 1 {
		t.Fatalf("task counts = %+v", report)
	}
	if report.NotesAdded != 1 || report.NotesDuplicate != 1 || report.NotesConflict != 0 || report.SettingsResult != "imported" {
		t.Fatalf("note/settings counts = %+v", report)
	}
	if got, _ := Load(); len(got) != 3 || got[0].Title != "existing" {
		t.Fatalf("merged tasks = %#v", got)
	}

	report, err = ImportData(source)
	if err != nil || len(report.Errors) != 0 || report.TasksAdded != 0 || report.NotesAdded != 0 {
		t.Fatalf("repeat import not idempotent: %+v, %v", report, err)
	}
	if report.TasksDuplicate != 3 || report.TasksConflict != 1 || report.NotesDuplicate != 2 || report.SettingsResult != "skipped (destination exists)" {
		t.Fatalf("repeat counts = %+v", report)
	}
}

func TestImportDataReportsMalformedAndNoRecognizedFiles(t *testing.T) {
	isolateXDG(t)
	bad := t.TempDir()
	if err := os.WriteFile(filepath.Join(bad, "tasks.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(bad, "notes.json"), []Note{{ID: "n"}})
	report, err := ImportData(bad)
	if err != nil || len(report.Errors) != 1 || report.NotesAdded != 1 {
		t.Fatalf("independent malformed import = %+v, %v", report, err)
	}
	if _, err := ImportData(t.TempDir()); err == nil {
		t.Fatal("empty source should fail")
	}
	if _, err := ImportData(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing source should fail")
	}
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return dir
}
