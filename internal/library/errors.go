package library

import (
	"errors"
	"fmt"
)

// Sentinel errors for library operations.
var (
	// ErrTemplateExists indicates a template with the same name already exists.
	ErrTemplateExists = errors.New("template already exists")

	// ErrTemplateNotFound indicates the requested template is not in the library.
	ErrTemplateNotFound = errors.New("template not found in library")

	// ErrEmptyLibrary indicates the library has no installed templates.
	ErrEmptyLibrary = errors.New("library is empty")

	// ErrInvalidName indicates the template name is invalid.
	ErrInvalidName = errors.New("invalid template name")
)

// LibraryError represents a structured error from a library operation.
type LibraryError struct {
	Name      string // Template name involved
	Operation string // "add", "remove", "update", etc.
	Err       error
}

func (e *LibraryError) Error() string {
	if e.Name != "" {
		return fmt.Sprintf("library %s %q: %v", e.Operation, e.Name, e.Err)
	}
	return fmt.Sprintf("library %s: %v", e.Operation, e.Err)
}

func (e *LibraryError) Unwrap() error {
	return e.Err
}
