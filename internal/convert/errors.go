package convert

import (
	"errors"
)

// Conversion errors.
var (
	ErrNoCookiecutterConfig = errors.New("cookiecutter.json not found")
	ErrInvalidConfig        = errors.New("invalid cookiecutter.json")
	ErrOutputExists         = errors.New("output directory already exists")
	ErrSourceNotFound       = errors.New("source template not found")
)
