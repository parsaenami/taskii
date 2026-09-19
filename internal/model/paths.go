package model

import (
	"path/filepath"

	"github.com/adrg/xdg"
)

const appDataDir = "taskii"

func dataPath(name string) (string, error) {
	return xdg.DataFile(appDataDir + "/" + name)
}

func configPath(name string) (string, error) {
	return xdg.ConfigFile(appDataDir + "/" + name)
}

func legacyPath(dir, name string) string { return filepath.Join(dir, name) }
