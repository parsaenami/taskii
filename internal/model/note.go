package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const notesPath = "data/notes.json"

// Note is one bullet on the Notes board. Body may contain newlines (entered
// with Shift+Enter / Alt+Enter) and has no length limit, so renderers must
// wrap rather than assume one note is one line.
type Note struct {
	ID        string    `json:"id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

func LoadNotes() ([]Note, error) {
	b, err := os.ReadFile(notesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Note{}, nil
		}
		return nil, err
	}
	if len(b) == 0 {
		return []Note{}, nil
	}
	var n []Note
	if err := json.Unmarshal(b, &n); err != nil {
		return nil, err
	}
	return n, nil
}

func SaveNotes(notes []Note) error {
	if err := os.MkdirAll(filepath.Dir(notesPath), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(notes, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(notesPath, b, 0o644)
}
