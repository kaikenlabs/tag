// Package xdg provides XDG Base Directory resolution for TAG.
package xdg

import (
	"fmt"
	"os"
	"path/filepath"
)

const appName = "tag"

// DataHome returns the XDG data directory for TAG.
// It respects $XDG_DATA_HOME, defaulting to ~/.local/share/tag.
func DataHome() (string, error) {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		if !filepath.IsAbs(dir) {
			return "", fmt.Errorf("$XDG_DATA_HOME must be an absolute path, got: %s", dir)
		}
		return filepath.Join(dir, appName), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}

	return filepath.Join(home, ".local", "share", appName), nil
}

// ConfigHome returns the XDG config directory for TAG.
// It respects $XDG_CONFIG_HOME, defaulting to ~/.config/tag.
func ConfigHome() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		if !filepath.IsAbs(dir) {
			return "", fmt.Errorf("$XDG_CONFIG_HOME must be an absolute path, got: %s", dir)
		}
		return filepath.Join(dir, appName), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}

	return filepath.Join(home, ".config", appName), nil
}
