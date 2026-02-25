package scaffold

import "github.com/kaikenlabs/tag/internal/tmplconfig"

// IsCookiecutterTemplate checks if a directory contains a Cookiecutter template.
// Returns the path to cookiecutter.json if found, and a boolean indicating detection.
var IsCookiecutterTemplate = tmplconfig.IsCookiecutterTemplate
