package convert

import (
	"errors"
	"fmt"
)

// Conversion errors.
var (
	ErrNoCookiecutterConfig = errors.New("cookiecutter.json not found")
	ErrInvalidConfig        = errors.New("invalid cookiecutter.json")
	ErrOutputExists         = errors.New("output directory already exists")
	ErrSourceNotFound       = errors.New("source template not found")
)

// ConversionError wraps errors with conversion context.
type ConversionError struct {
	Op      string // Operation that failed (e.g., "parse config", "convert path")
	Path    string // File path involved, if any
	Message string // Human-readable message
	Err     error  // Underlying error
}

func (e *ConversionError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s: %s: %s", e.Op, e.Path, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Message)
}

func (e *ConversionError) Unwrap() error {
	return e.Err
}

// NewConversionError creates a new ConversionError.
func NewConversionError(op, path, message string, err error) *ConversionError {
	return &ConversionError{
		Op:      op,
		Path:    path,
		Message: message,
		Err:     err,
	}
}

// Errorf creates a ConversionError with formatted message.
func Errorf(op, path string, format string, args ...any) *ConversionError {
	return &ConversionError{
		Op:      op,
		Path:    path,
		Message: fmt.Sprintf(format, args...),
	}
}
