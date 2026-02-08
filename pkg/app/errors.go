package app

import (
	"errors"
	"fmt"
)

// CommandError represents an error that occurred during command execution.
// It wraps an underlying cause error and provides a user-friendly message.
type CommandError struct {
	Message string
	Cause   error
}

// Error returns the error message.
func (e *CommandError) Error() string {
	return e.Message
}

// Unwrap returns the underlying cause error, enabling errors.Is and errors.As.
func (e *CommandError) Unwrap() error {
	return e.Cause
}

// Errorf creates a new CommandError with a formatted message.
// It supports the %w verb for wrapping errors, similar to fmt.Errorf.
func Errorf(format string, args ...any) error {
	wrapped := fmt.Errorf(format, args...)
	return &CommandError{
		Message: wrapped.Error(),
		Cause:   errors.Unwrap(wrapped),
	}
}
