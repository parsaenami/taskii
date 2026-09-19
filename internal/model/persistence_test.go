package model

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/adrg/xdg"
)

func isolateXDG(t *testing.T) (string, string) {
	t.Helper()
	dataHome := t.TempDir()
	configHome := t.TempDir()
	t.Cleanup(xdg.Reload)
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	xdg.Reload()
	return dataHome, configHome
}

func changeDir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}

func TestTaskPersistenceUsesXDGAfterChangingDirectory(t *testing.T) {
	dataHome, _ := isolateXDG(t)
	tasks := []Task{{ID: "task-1", Title: "Keep focus"}}

	changeDir(t, t.TempDir())
	if err := Save(tasks); err != nil {
		t.Fatal(err)
	}
	changeDir(t, t.TempDir())
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, tasks) {
		t.Fatalf("Load() = %#v, want %#v", loaded, tasks)
	}
	if _, err := os.Stat(filepath.Join(dataHome, appDataDir, "tasks.json")); err != nil {
		t.Fatalf("tasks were not saved under XDG data home: %v", err)
	}
}

func TestNotePersistenceUsesXDGAfterChangingDirectory(t *testing.T) {
	dataHome, _ := isolateXDG(t)
	notes := []Note{{ID: "note-1", Body: "Remember this"}}

	changeDir(t, t.TempDir())
	if err := SaveNotes(notes); err != nil {
		t.Fatal(err)
	}
	changeDir(t, t.TempDir())
	loaded, err := LoadNotes()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, notes) {
		t.Fatalf("LoadNotes() = %#v, want %#v", loaded, notes)
	}
	if _, err := os.Stat(filepath.Join(dataHome, appDataDir, "notes.json")); err != nil {
		t.Fatalf("notes were not saved under XDG data home: %v", err)
	}
}

func TestSettingsPersistenceUsesXDGConfigAfterChangingDirectory(t *testing.T) {
	_, configHome := isolateXDG(t)
	want := Settings{Theme: "Ember", Layout: "stacked"}

	changeDir(t, t.TempDir())
	if err := SaveSettings(want); err != nil {
		t.Fatal(err)
	}
	changeDir(t, t.TempDir())
	got, err := LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadSettings() = %#v, want %#v", got, want)
	}
	if _, err := os.Stat(filepath.Join(configHome, appDataDir, "settings.json")); err != nil {
		t.Fatalf("settings were not saved under XDG config home: %v", err)
	}
}
