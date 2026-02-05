package scaffold

import (
	"errors"
	"fmt"
)

// Common errors for the scaffold package.
var (
	// ErrPromptCancelled is returned when the user cancels an interactive prompt.
	ErrPromptCancelled = errors.New("prompt cancelled by user")

	// ErrRequiredVariableMissing is returned when a required variable has no value.
	ErrRequiredVariableMissing = errors.New("required variable missing")

	// ErrInvalidVariableType is returned when a variable value doesn't match its type.
	ErrInvalidVariableType = errors.New("invalid variable type")

	// ErrOutputExists is returned when the output directory already exists.
	ErrOutputExists = errors.New("output directory already exists")

	// ErrTemplateNotFound is returned when the template directory doesn't exist.
	ErrTemplateNotFound = errors.New("template directory not found")

	// ErrConfigNotFound is returned when tag.template.json is missing.
	ErrConfigNotFound = errors.New("tag.template.json not found")
)

// VariableError represents an error related to a specific variable.
type VariableError struct {
	Name    string
	Message string
	Err     error
}

func (e *VariableError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("variable %q: %s: %v", e.Name, e.Message, e.Err)
	}
	return fmt.Sprintf("variable %q: %s", e.Name, e.Message)
}

func (e *VariableError) Unwrap() error {
	return e.Err
}

// NewVariableError creates a new variable error.
func NewVariableError(name, message string, err error) *VariableError {
	return &VariableError{Name: name, Message: message, Err: err}
}

// PathError represents an error related to path processing.
type PathError struct {
	Path    string
	Message string
	Err     error
}

func (e *PathError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("path %q: %s: %v", e.Path, e.Message, e.Err)
	}
	return fmt.Sprintf("path %q: %s", e.Path, e.Message)
}

func (e *PathError) Unwrap() error {
	return e.Err
}

// NewPathError creates a new path error.
func NewPathError(path, message string, err error) *PathError {
	return &PathError{Path: path, Message: message, Err: err}
}

// TemplateError represents an error during template processing.
type TemplateError struct {
	File    string
	Message string
	Err     error
}

func (e *TemplateError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("template %q: %s: %v", e.File, e.Message, e.Err)
	}
	return fmt.Sprintf("template %q: %s", e.File, e.Message)
}

func (e *TemplateError) Unwrap() error {
	return e.Err
}

// NewTemplateError creates a new template error.
func NewTemplateError(file, message string, err error) *TemplateError {
	return &TemplateError{File: file, Message: message, Err: err}
}

// ErrHookFailed is returned when a hook command fails.
var ErrHookFailed = errors.New("hook command failed")

// HookError represents an error during hook execution.
type HookError struct {
	Phase    HookPhase // pre_scaffold or post_scaffold
	Command  string    // The command that failed
	Output   string    // Combined stdout/stderr output
	ExitCode int       // Exit code of the command
	Err      error     // Underlying error
}

func (e *HookError) Error() string {
	if e.ExitCode != 0 {
		return fmt.Sprintf("%s hook failed: command %q exited with code %d", e.Phase, e.Command, e.ExitCode)
	}
	if e.Err != nil {
		return fmt.Sprintf("%s hook failed: command %q: %v", e.Phase, e.Command, e.Err)
	}
	return fmt.Sprintf("%s hook failed: command %q", e.Phase, e.Command)
}

func (e *HookError) Unwrap() error {
	return e.Err
}

// NewHookError creates a new hook error.
func NewHookError(phase HookPhase, command, output string, exitCode int, err error) *HookError {
	return &HookError{
		Phase:    phase,
		Command:  command,
		Output:   output,
		ExitCode: exitCode,
		Err:      err,
	}
}
