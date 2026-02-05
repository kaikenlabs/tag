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
	// Use fmt.Errorf to handle formatting and error wrapping
	wrapped := fmt.Errorf(format, args...)

	// Extract the underlying error if %w was used
	var cause error
	for _, arg := range args {
		if err, ok := arg.(error); ok {
			// Check if this error was actually wrapped (appears in the chain)
			if errors.Is(wrapped, err) {
				cause = err
				break
			}
		}
	}

	return &CommandError{
		Message: wrapped.Error(),
		Cause:   cause,
	}
}
