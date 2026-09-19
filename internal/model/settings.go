package model

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Settings struct {
	Theme  string `json:"theme,omitempty"`
	Layout string `json:"layout,omitempty"`

	// Pomodoro durations are stored in minutes. Zero means "use the built-in
	// default" — LoadSettings never fabricates them, so an old settings file
	// without these fields keeps working.
	PomodoroFocusMinutes      int  `json:"pomodoroFocusMinutes,omitempty"`
	PomodoroShortBreakMinutes int  `json:"pomodoroShortBreakMinutes,omitempty"`
	PomodoroLongBreakMinutes  int  `json:"pomodoroLongBreakMinutes,omitempty"`
	PomodoroLongBreakEvery    int  `json:"pomodoroLongBreakEvery,omitempty"`
	PomodoroAutoStartNext     bool `json:"pomodoroAutoStartNext,omitempty"`
}

func LoadSettings() (Settings, error) {
	path, err := configPath("settings.json")
	if err != nil {
		return Settings{}, err
	}
	b, err := os.ReadFile(path)
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
	path, err := configPath("settings.json")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(path, b)
}
