package tui

import (
	"os"
	"path/filepath"
)

// configDir returns the teetui config root (~/.config/teetui per §I), honoring
// XDG_CONFIG_HOME, and ensures it exists.
func configDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "teetui")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// historyPath returns the on-disk history file for an input mode slug.
func historyPath(modeSlug string) (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "history", modeSlug+".txt"), nil
}
