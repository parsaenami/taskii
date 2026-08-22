package model

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const settingsPath = "data/settings.json"

type Settings struct {
	Theme  string `json:"theme,omitempty"`
	Layout string `json:"layout,omitempty"`
}

func LoadSettings() (Settings, error) {
	b, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Settings{}, nil
		}
		return Settings{}, err
	}
	if len(b) == 0 {
		return Settings{}, nil
	}
	var s Settings
	if err := json.Unmarshal(b, &s); err != nil {
		return Settings{}, err
	}
	return s, nil
}

func SaveSettings(s Settings) error {
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath, b, 0o644)
}
