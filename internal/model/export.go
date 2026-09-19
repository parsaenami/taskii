package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const exportDirName = "taski_data"

// ExportData writes a portable snapshot of all persisted data below dir.
// Existing exports are never overwritten.
func ExportData(dir string) (string, error) {
	if dir == "" {
		return "", errors.New("export destination is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create export destination: %w", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		return "", fmt.Errorf("export destination: %w", err)
	}
	if !info.IsDir() {
		return "", errors.New("export destination is not a directory")
	}

	tasks, err := Load()
	if err != nil {
		return "", fmt.Errorf("load tasks: %w", err)
	}
	notes, err := LoadNotes()
	if err != nil {
		return "", fmt.Errorf("load notes: %w", err)
	}
	settings, err := LoadSettings()
	if err != nil {
		return "", fmt.Errorf("load settings: %w", err)
	}

	target := filepath.Join(dir, exportDirName)
	if _, err := os.Stat(target); err == nil {
		return "", fmt.Errorf("export directory already exists: %s", target)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("check export directory: %w", err)
	}

	staging, err := os.MkdirTemp(dir, ".taski_data-*")
	if err != nil {
		return "", fmt.Errorf("create export staging directory: %w", err)
	}
	defer os.RemoveAll(staging)

	for _, file := range []struct {
		name  string
		value any
	}{
		{name: "tasks.json", value: tasks},
		{name: "notes.json", value: notes},
		{name: "settings.json", value: settings},
	} {
		data, err := json.MarshalIndent(file.value, "", "  ")
		if err != nil {
			return "", fmt.Errorf("encode %s: %w", file.name, err)
		}
		if err := atomicWriteFile(filepath.Join(staging, file.name), data); err != nil {
			return "", fmt.Errorf("write %s: %w", file.name, err)
		}
	}
	if err := os.Rename(staging, target); err != nil {
		return "", fmt.Errorf("finalize export: %w", err)
	}
	return target, nil
}
