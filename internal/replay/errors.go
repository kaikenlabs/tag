package replay

import (
	"errors"
	"fmt"
)

// Common errors for the replay package.
var (
	// ErrReplayNotFound is returned when a replay file doesn't exist.
	ErrReplayNotFound = errors.New("replay file not found")

	// ErrReplayCorrupt is returned when a replay file exists but cannot be parsed.
	ErrReplayCorrupt = errors.New("replay file is corrupt or invalid")

	// ErrEmptyTemplateSource is returned when an empty template source is provided.
	ErrEmptyTemplateSource = errors.New("empty template source")
)

// ReplayError represents an error related to replay operations.
type ReplayError struct {
	TemplateID string
	Operation  string
	Err        error
}

func (e *ReplayError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("replay %s for %q: %v", e.Operation, e.TemplateID, e.Err)
	}
	return fmt.Sprintf("replay %s for %q", e.Operation, e.TemplateID)
}

func (e *ReplayError) Unwrap() error {
	return e.Err
}

// NewReplayError creates a new replay error.
func NewReplayError(templateID, operation string, err error) *ReplayError {
	return &ReplayError{TemplateID: templateID, Operation: operation, Err: err}
}
