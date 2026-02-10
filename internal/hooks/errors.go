package hooks

import (
	"errors"
	"fmt"
)

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
