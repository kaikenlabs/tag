package scaffold

import (
	"os"
	"path/filepath"

	"github.com/kaikenlabs/tag/internal/types"
)

// IsCookiecutterTemplate checks if a directory contains a Cookiecutter template.
// Returns the path to cookiecutter.json if found, and a boolean indicating detection.
func IsCookiecutterTemplate(templateDir string) (cookiecutterPath string, isCookiecutter bool) {
	ccPath := filepath.Join(templateDir, types.CookiecutterConfigFile)
	if _, err := os.Stat(ccPath); err == nil {
		return ccPath, true
	}
	return "", false
}
