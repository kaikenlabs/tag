package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/internal/xdg"
)

// GlobalConfig holds user-level TAG settings persisted across projects.
type GlobalConfig struct {
	Editor string `json:"editor,omitempty"`
}

// globalConfigFile is the filename within the XDG config directory.
const globalConfigFile = "config.json"

// GlobalConfigPath returns the full path to the global config file.
func GlobalConfigPath() (string, error) {
	dir, err := xdg.ConfigHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, globalConfigFile), nil
}

// LoadGlobalConfig loads the user-level config from $XDG_CONFIG_HOME/tag/config.json.
// Returns a zero-value GlobalConfig if the file does not exist.
func LoadGlobalConfig() (*GlobalConfig, error) {
	path, err := GlobalConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &GlobalConfig{}, nil
		}
		return nil, err
	}

	var cfg GlobalConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SaveGlobalConfig writes the user-level config to $XDG_CONFIG_HOME/tag/config.json.
// It creates parent directories as needed.
func SaveGlobalConfig(cfg *GlobalConfig) error {
	path, err := GlobalConfigPath()
	if err != nil {
		return err
	}

	if mkdirErr := os.MkdirAll(filepath.Dir(path), types.DirModePrivate); mkdirErr != nil {
		return mkdirErr
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, types.FileModePrivate)
}
