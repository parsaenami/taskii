package model

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Settings struct {
	Theme  string `json:"theme,omitempty"`
	Layout string `json:"layout,omitempty"`
	// Nil preserves the default for settings written before update checks were
	// introduced. A pointer distinguishes an explicit false from omission.
	CheckForUpdates *bool `json:"checkForUpdates,omitempty"`
	// Nil means the legacy default. A pointer distinguishes Sunday (0) from
	// an omitted week start; an empty non-nil workday set is invalid.
	Workdays  []time.Weekday `json:"workdays,omitempty"`
	WeekStart *time.Weekday  `json:"week_start,omitempty"`

	// Pomodoro durations are stored in minutes. Zero means "use the built-in
	// default" — LoadSettings never fabricates them, so an old settings file
	// without these fields keeps working.
	PomodoroFocusMinutes      int  `json:"pomodoroFocusMinutes,omitempty"`
	PomodoroShortBreakMinutes int  `json:"pomodoroShortBreakMinutes,omitempty"`
	PomodoroLongBreakMinutes  int  `json:"pomodoroLongBreakMinutes,omitempty"`
	PomodoroLongBreakEvery    int  `json:"pomodoroLongBreakEvery,omitempty"`
	PomodoroAutoStartNext     bool `json:"pomodoroAutoStartNext,omitempty"`
}

var defaultWorkdays = []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}

func (s Settings) EffectiveWorkdays() []time.Weekday {
	if s.Workdays == nil {
		return append([]time.Weekday(nil), defaultWorkdays...)
	}
	return append([]time.Weekday(nil), s.Workdays...)
}

func (s Settings) EffectiveWeekStart() time.Weekday {
	if s.WeekStart == nil {
		return time.Monday
	}
	return *s.WeekStart
}

// EffectiveCheckForUpdates keeps automatic checks enabled by default while
// still allowing users to explicitly opt out.
func (s Settings) EffectiveCheckForUpdates() bool {
	return s.CheckForUpdates == nil || *s.CheckForUpdates
}

func (s Settings) ValidateCalendar() error {
	if s.Workdays != nil {
		if err := ValidateWeekdays(s.Workdays); err != nil {
			return err
		}
	}
	if s.WeekStart != nil && (*s.WeekStart < time.Sunday || *s.WeekStart > time.Saturday) {
		return fmt.Errorf("invalid week start %d", *s.WeekStart)
	}
	return nil
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
	if err := s.ValidateCalendar(); err != nil {
		return Settings{}, err
	}
	return s, nil
}

func SaveSettings(s Settings) error {
	if err := s.ValidateCalendar(); err != nil {
		return err
	}
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
