package commands

import (
	"github.com/kaikenlabs/tag/internal/dialect"
	"github.com/kaikenlabs/tag/internal/types"
)

// loadDialectRegistry loads dialects with three-tier resolution:
// built-in → user-global → template-local (_dialects/).
// A non-empty templateDir enables the template-local tier.
func loadDialectRegistry(templateDir string) (*dialect.Registry, error) {
	if templateDir != "" {
		return dialect.LoadForTemplate(templateDir, types.DialectsDir)
	}
	return loadDialectRegistryGlobal()
}

// loadDialectRegistryGlobal loads dialects with two-tier resolution only:
// built-in → user-global (no template-local).
// Used for contexts where a template directory is not available (e.g., generators).
func loadDialectRegistryGlobal() (*dialect.Registry, error) {
	userDir, err := dialect.UserDialectsDir()
	if err != nil {
		return nil, err
	}
	return dialect.LoadWithOverrides(userDir, "")
}
