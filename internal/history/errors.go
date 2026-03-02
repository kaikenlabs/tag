package history

import (
	"errors"
	"fmt"
)

// ErrNotFound is returned when a generation ID does not exist in the manifest.
var ErrNotFound = errors.New("generation not found")

// ErrConflict is returned when one or more files have been modified after the
// generation was recorded, and --force has not been passed.
var ErrConflict = errors.New("files modified after generation")

// ConflictError carries the list of paths that have conflicting modifications.
type ConflictError struct {
	Paths []string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("%s: %v", ErrConflict.Error(), e.Paths)
}

func (e *ConflictError) Unwrap() error { return ErrConflict }
