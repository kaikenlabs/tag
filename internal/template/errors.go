package template

import (
	"errors"
	"fmt"
)

// Common sentinel errors for the template package.
var (
	// ErrParse indicates a template parsing error.
	ErrParse = errors.New("template parse error")

	// ErrExecute indicates a template execution error.
	ErrExecute = errors.New("template execution error")
)

// TemplateError wraps template-related errors with additional context.
type TemplateError struct {
	// Op is the operation that failed (e.g., "parse", "execute").
	Op string

	// Template is the name or path of the template.
	Template string

	// Line is the line number where the error occurred (if available).
	Line int

	// Column is the column number where the error occurred (if available).
	Column int

	// Err is the underlying error.
	Err error
}

// Error returns the formatted error message.
func (e *TemplateError) Error() string {
	if e.Template == "" {
		return fmt.Sprintf("%s: %v", e.Op, e.Err)
	}
	if e.Line > 0 {
		if e.Column > 0 {
			return fmt.Sprintf("%s: %s:%d:%d: %v", e.Op, e.Template, e.Line, e.Column, e.Err)
		}
		return fmt.Sprintf("%s: %s:%d: %v", e.Op, e.Template, e.Line, e.Err)
	}
	return fmt.Sprintf("%s: %s: %v", e.Op, e.Template, e.Err)
}

// Unwrap returns the underlying error for errors.Is and errors.As.
func (e *TemplateError) Unwrap() error {
	return e.Err
}

// NewParseError creates a new parse error with context.
func NewParseError(template string, line, column int, err error) error {
	return &TemplateError{
		Op:       "parse",
		Template: template,
		Line:     line,
		Column:   column,
		Err:      fmt.Errorf("%w: %v", ErrParse, err),
	}
}

// NewExecuteError creates a new execution error with context.
func NewExecuteError(template string, err error) error {
	return &TemplateError{
		Op:       "execute",
		Template: template,
		Err:      fmt.Errorf("%w: %v", ErrExecute, err),
	}
}

