package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// MigrationReport describes an attempted legacy-data migration. Errors are
// warnings: callers may display them while still starting the application.
type MigrationReport struct {
	Results []string
	Errors  []error
}

func (r MigrationReport) WarningText() string {
	if len(r.Errors) == 0 {
		return ""
	}
	parts := make([]string, 0, len(r.Errors))
	for _, err := range r.Errors {
		parts = append(parts, err.Error())
	}
	return "legacy data: " + strings.Join(parts, "; ")
}

// MigrateLegacyData copies valid files from ./data into their XDG locations.
// It deliberately handles each file independently and never removes the
// legacy source.
func MigrateLegacyData() MigrationReport {
	wd, err := os.Getwd()
	if err != nil {
		return MigrationReport{Errors: []error{err}}
	}
	return migrateLegacyDir(filepath.Join(wd, "data"))
}

func migrateLegacyDir(dir string) MigrationReport {
	var report MigrationReport
	for _, name := range []string{"tasks.json", "notes.json", "routines.json", "settings.json"} {
		src := legacyPath(dir, name)
		if _, err := os.Stat(src); err != nil {
			if !os.IsNotExist(err) {
				report.Errors = append(report.Errors, fmt.Errorf("%s: %w", name, err))
			}
			continue
		}
		dst, err := destinationPath(name)
		if err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("%s: %w", name, err))
			continue
		}
		if _, err := os.Stat(dst); err == nil {
			report.Results = append(report.Results, name+": skipped (destination exists)")
			continue
		} else if !os.IsNotExist(err) {
			report.Errors = append(report.Errors, fmt.Errorf("%s: destination: %w", name, err))
			continue
		}
		b, err := os.ReadFile(src)
		if err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("%s: read: %w", name, err))
			continue
		}
		if err := validateJSON(name, b); err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("%s: %w", name, err))
			continue
		}
		if err := atomicWriteFile(dst, b); err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("%s: copy: %w", name, err))
			continue
		}
		report.Results = append(report.Results, name+": migrated")
	}
	return report
}

type ImportReport struct {
	TasksAdded, TasksDuplicate, TasksConflict          int
	NotesAdded, NotesDuplicate, NotesConflict          int
	RoutinesAdded, RoutinesDuplicate, RoutinesConflict int
	SettingsResult                                     string
	Results                                            []string
	Errors                                             []error
}

// ImportData merges recognized files from dir into XDG storage. Existing
// records win; source files are never changed.
func ImportData(dir string) (ImportReport, error) {
	var report ImportReport
	info, err := os.Stat(dir)
	if err != nil {
		return report, fmt.Errorf("import source: %w", err)
	}
	if !info.IsDir() {
		return report, errors.New("import source is not a directory")
	}
	recognized := 0
	for _, name := range []string{"tasks.json", "notes.json", "routines.json", "settings.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			recognized++
		} else if !os.IsNotExist(err) {
			report.Errors = append(report.Errors, fmt.Errorf("%s: %w", name, err))
		}
	}
	if recognized == 0 {
		return report, errors.New("import source contains no recognized data files")
	}
	for _, name := range []string{"tasks.json", "notes.json", "routines.json", "settings.json"} {
		src := filepath.Join(dir, name)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}
		b, err := os.ReadFile(src)
		if err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("%s: read: %w", name, err))
			continue
		}
		var validationErr error
		if name == "routines.json" {
			validationErr = validateRoutineJSON(b, false)
		} else {
			validationErr = validateJSON(name, b)
		}
		if err := validationErr; err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("%s: %w", name, err))
			continue
		}
		switch name {
		case "tasks.json":
			mergeTasks(b, &report)
		case "notes.json":
			mergeNotes(b, &report)
		case "routines.json":
			mergeRoutines(b, &report)
		case "settings.json":
			importSettings(b, &report)
		}
	}
	return report, nil
}

func destinationPath(name string) (string, error) {
	if name == "settings.json" {
		return configPath(name)
	}
	return dataPath(name)
}

func validateJSON(name string, b []byte) error {
	if len(bytes.TrimSpace(b)) == 0 {
		return errors.New("empty JSON")
	}
	switch name {
	case "tasks.json":
		var tasks []Task
		if err := json.Unmarshal(b, &tasks); err != nil {
			return fmt.Errorf("invalid task data: %w", err)
		}
	case "notes.json":
		var notes []Note
		if err := json.Unmarshal(b, &notes); err != nil {
			return fmt.Errorf("invalid note data: %w", err)
		}
	case "routines.json":
		return validateRoutineJSON(b, true)
	case "settings.json":
		var settings Settings
		if err := json.Unmarshal(b, &settings); err != nil {
			return fmt.Errorf("invalid settings data: %w", err)
		}
		if err := settings.ValidateCalendar(); err != nil {
			return fmt.Errorf("invalid settings data: %w", err)
		}
	default:
		return fmt.Errorf("unrecognized data file %q", name)
	}
	return nil
}

func validateRoutineJSON(b []byte, unique bool) error {
	var routines []Routine
	if err := json.Unmarshal(b, &routines); err != nil {
		return fmt.Errorf("invalid routine data: %w", err)
	}
	if routines == nil {
		return errors.New("invalid routine data: expected array")
	}
	if unique {
		if err := ValidateRoutines(routines); err != nil {
			return fmt.Errorf("invalid routine data: %w", err)
		}
		return nil
	}
	// Imports merge each entry by ID, just like tasks and notes. Duplicate IDs
	// in the source become duplicate/conflict counts rather than failing all.
	for i, routine := range routines {
		if err := routine.Validate(); err != nil {
			return fmt.Errorf("invalid routine %d: %w", i, err)
		}
	}
	return nil
}

func mergeRoutines(b []byte, r *ImportReport) {
	var source []Routine
	if err := json.Unmarshal(b, &source); err != nil {
		r.Errors = append(r.Errors, fmt.Errorf("routines.json: %w", err))
		return
	}
	existing, err := LoadRoutines()
	if err != nil {
		r.Errors = append(r.Errors, fmt.Errorf("routines.json destination: %w", err))
		return
	}
	byID := make(map[string]Routine, len(existing))
	for _, item := range existing {
		byID[item.ID] = item
	}
	added := 0
	for _, item := range source {
		if old, ok := byID[item.ID]; ok {
			if reflect.DeepEqual(old, item) {
				r.RoutinesDuplicate++
			} else {
				r.RoutinesConflict++
			}
			continue
		}
		existing = append(existing, item)
		byID[item.ID] = item
		added++
	}
	if added > 0 {
		if err := SaveRoutines(existing); err != nil {
			r.Errors = append(r.Errors, fmt.Errorf("routines.json write: %w", err))
			return
		}
		r.RoutinesAdded += added
	}
}

func mergeTasks(b []byte, r *ImportReport) {
	var source, existing []Task
	if err := json.Unmarshal(b, &source); err != nil {
		r.Errors = append(r.Errors, fmt.Errorf("tasks.json: %w", err))
		return
	}
	existing, err := Load()
	if err != nil {
		r.Errors = append(r.Errors, fmt.Errorf("tasks.json destination: %w", err))
		return
	}
	byID := map[string]Task{}
	for _, item := range existing {
		byID[item.ID] = item
	}
	added := 0
	for _, item := range source {
		if old, ok := byID[item.ID]; ok {
			if reflect.DeepEqual(old, item) {
				r.TasksDuplicate++
			} else {
				r.TasksConflict++
			}
			continue
		}
		existing = append(existing, item)
		byID[item.ID] = item
		added++
	}
	if added > 0 {
		if err := Save(existing); err != nil {
			r.Errors = append(r.Errors, fmt.Errorf("tasks.json write: %w", err))
			return
		}
		r.TasksAdded += added
	}
}

func mergeNotes(b []byte, r *ImportReport) {
	var source, existing []Note
	if err := json.Unmarshal(b, &source); err != nil {
		r.Errors = append(r.Errors, fmt.Errorf("notes.json: %w", err))
		return
	}
	existing, err := LoadNotes()
	if err != nil {
		r.Errors = append(r.Errors, fmt.Errorf("notes.json destination: %w", err))
		return
	}
	byID := map[string]Note{}
	for _, item := range existing {
		byID[item.ID] = item
	}
	added := 0
	for _, item := range source {
		if old, ok := byID[item.ID]; ok {
			if reflect.DeepEqual(old, item) {
				r.NotesDuplicate++
			} else {
				r.NotesConflict++
			}
			continue
		}
		existing = append(existing, item)
		byID[item.ID] = item
		added++
	}
	if added > 0 {
		if err := SaveNotes(existing); err != nil {
			r.Errors = append(r.Errors, fmt.Errorf("notes.json write: %w", err))
			return
		}
		r.NotesAdded += added
	}
}

func importSettings(b []byte, r *ImportReport) {
	dst, err := configPath("settings.json")
	if err != nil {
		r.Errors = append(r.Errors, err)
		return
	}
	if _, err := os.Stat(dst); err == nil {
		r.SettingsResult = "skipped (destination exists)"
		return
	} else if !os.IsNotExist(err) {
		r.Errors = append(r.Errors, err)
		return
	}
	var s Settings
	if err := json.Unmarshal(b, &s); err != nil {
		r.Errors = append(r.Errors, fmt.Errorf("settings.json: %w", err))
		return
	}
	if err := atomicWriteFile(dst, b); err != nil {
		r.Errors = append(r.Errors, fmt.Errorf("settings.json write: %w", err))
		return
	}
	r.SettingsResult = "imported"
}
