package app

import (
	"errors"
	"fmt"
)

// Exit codes following POSIX conventions.
const (
	ExitOK          = 0   // Success
	ExitGeneral     = 1   // General application error
	ExitUsage       = 2   // Usage/argument error (missing args, invalid flags)
	ExitNotFound    = 3   // Resource not found (template, library)
	ExitInterrupted = 130 // Interrupted by SIGINT (Ctrl+C)
)

// CommandError represents an error that occurred during command execution.
// It wraps an underlying cause error and provides a user-friendly message.
// Implements cli.ExitCoder so urfave/cli can extract the exit code.
type CommandError struct {
	Message string
	Cause   error
	Code    int
}

// Error returns the error message.
func (e *CommandError) Error() string {
	return e.Message
}

// Unwrap returns the underlying cause error, enabling errors.Is and errors.As.
func (e *CommandError) Unwrap() error {
	return e.Cause
}

// ExitCode returns the exit code for this error. If Code is explicitly set,
// it is returned. Otherwise defaults to ExitGeneral (1).
// Implements the cli.ExitCoder interface.
func (e *CommandError) ExitCode() int {
	if e.Code != 0 {
		return e.Code
	}
	return ExitGeneral
}

// Errorf creates a new CommandError with a formatted message and exit code 1.
// It supports the %w verb for wrapping errors, similar to fmt.Errorf.
func Errorf(format string, args ...any) error {
	wrapped := fmt.Errorf(format, args...)
	return &CommandError{
		Message: wrapped.Error(),
		Cause:   errors.Unwrap(wrapped),
	}
}

// UsageErrorf creates a CommandError with exit code 2 (usage/argument error).
func UsageErrorf(format string, args ...any) error {
	wrapped := fmt.Errorf(format, args...)
	return &CommandError{
		Message: wrapped.Error(),
		Cause:   errors.Unwrap(wrapped),
		Code:    ExitUsage,
	}
}

// NotFoundErrorf creates a CommandError with exit code 3 (resource not found).
func NotFoundErrorf(format string, args ...any) error {
	wrapped := fmt.Errorf(format, args...)
	return &CommandError{
		Message: wrapped.Error(),
		Cause:   errors.Unwrap(wrapped),
		Code:    ExitNotFound,
	}
}
