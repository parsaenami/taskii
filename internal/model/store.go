package model

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const dataPath = "data/tasks.json"

func Load() ([]Task, error) {
	b, err := os.ReadFile(dataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Task{}, nil
		}
		return nil, err
	}
	if len(b) == 0 {
		return []Task{}, nil
	}
	var tasks []Task
	if err := json.Unmarshal(b, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

func Save(tasks []Task) error {
	if err := os.MkdirAll(filepath.Dir(dataPath), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dataPath, b, 0o644)
}
