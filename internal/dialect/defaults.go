package dialect

import (
	"fmt"
	"path/filepath"

	"github.com/kaikenlabs/tag/internal/xdg"
)

const builtinDir = "dialects"

// LoadDefaults creates a registry populated with only the built-in dialects.
func LoadDefaults() (*Registry, error) {
	reg := NewRegistry()

	if err := reg.LoadFS(builtinFS, builtinDir); err != nil {
		return nil, fmt.Errorf("failed to load built-in dialects: %w", err)
	}

	return reg, nil
}

// LoadWithOverrides creates a registry using three-tier loading:
//  1. Built-in dialects (embedded YAML files)
//  2. User-global dialects from userDir (if non-empty and directory exists)
//  3. Template-local dialects from templateDir (if non-empty and directory exists)
//
// Later tiers override individual type mappings via deep merge.
// Empty directory paths are silently skipped.
func LoadWithOverrides(userDir, templateDir string) (*Registry, error) {
	reg, err := LoadDefaults()
	if err != nil {
		return nil, err
	}

	if userDir != "" {
		if err := reg.LoadDir(userDir); err != nil {
			return nil, fmt.Errorf("failed to load user-global dialects from %s: %w", userDir, err)
		}
	}

	if templateDir != "" {
		if err := reg.LoadDir(templateDir); err != nil {
			return nil, fmt.Errorf("failed to load template-local dialects from %s: %w", templateDir, err)
		}
	}

	return reg, nil
}

// UserDialectsDir returns the user-global dialect directory path.
// This is <XDG_DATA_HOME>/tag/dialects/ (typically ~/.local/share/tag/dialects/).
func UserDialectsDir() (string, error) {
	dataHome, err := xdg.DataHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataHome, "dialects"), nil
}

// LoadForTemplate loads dialects using three-tier resolution for a given
// template directory: built-in → user-global → template-local (_dialects/).
// The dialectsDir constant must be passed in to avoid a dependency on
// internal/types from this leaf package.
func LoadForTemplate(templateRoot, dialectsDirName string) (*Registry, error) {
	userDir, err := UserDialectsDir()
	if err != nil {
		return nil, err
	}
	localDir := filepath.Join(templateRoot, dialectsDirName)
	return LoadWithOverrides(userDir, localDir)
}
