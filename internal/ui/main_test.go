package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"
)

func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "taskii-ui-test-")
	if err != nil {
		panic(err)
	}

	if err := os.Setenv("XDG_DATA_HOME", filepath.Join(root, "data")); err != nil {
		panic(err)
	}
	if err := os.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config")); err != nil {
		panic(err)
	}
	if err := os.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache")); err != nil {
		panic(err)
	}
	xdg.Reload()

	code := m.Run()
	_ = os.RemoveAll(root)
	os.Exit(code)
}
