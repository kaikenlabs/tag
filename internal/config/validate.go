package config

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ValidationError represents a configuration validation failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Validate checks the configuration for errors before execution.
// Returns the first validation error encountered (fail-fast).
func (c *Config) Validate() error {
	if c == nil {
		return &ValidationError{Field: "config", Message: "configuration is nil"}
	}

	// Validate template path exists (if specified)
	if c.Env.Path != "" {
		if err := validateDirectory("env.TAG_PATH", c.Env.Path); err != nil {
			return err
		}
	}

	// Validate pre-hooks
	if err := validateHooks("hooks.pre", c.Hooks.Pre); err != nil {
		return err
	}

	// Validate post-hooks
	if err := validateHooks("hooks.post", c.Hooks.Post); err != nil {
		return err
	}

	return nil
}

func validateDirectory(field, path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return &ValidationError{
			Field:   field,
			Message: fmt.Sprintf("path does not exist: %s", path),
		}
	}
	if err != nil {
		return &ValidationError{
			Field:   field,
			Message: fmt.Sprintf("cannot access path: %s", err),
		}
	}
	if !info.IsDir() {
		return &ValidationError{
			Field:   field,
			Message: fmt.Sprintf("path is not a directory: %s", path),
		}
	}
	return nil
}

func validateHooks(hookType string, hooks [][]string) error {
	for i, hook := range hooks {
		field := fmt.Sprintf("%s[%d]", hookType, i)

		if len(hook) == 0 {
			return &ValidationError{
				Field:   field,
				Message: "hook command is empty",
			}
		}

		cmd := hook[0]
		if cmd == "" {
			return &ValidationError{
				Field:   field,
				Message: "hook command is empty",
			}
		}

		// Reject commands with leading/trailing whitespace
		if strings.TrimSpace(cmd) != cmd {
			return &ValidationError{
				Field:   field,
				Message: fmt.Sprintf("hook command has leading/trailing whitespace: %q", cmd),
			}
		}

		// If path contains separator, check file exists and is executable; otherwise look in PATH
		if strings.ContainsAny(cmd, "/\\") {
			if err := validateHookExecutable(field, cmd); err != nil {
				return err
			}
		} else {
			if _, err := exec.LookPath(cmd); err != nil {
				return &ValidationError{
					Field:   field,
					Message: fmt.Sprintf("hook command not found in PATH: %s", cmd),
				}
			}
		}
	}
	return nil
}

func validateHookExecutable(field, path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return &ValidationError{
			Field:   field,
			Message: fmt.Sprintf("hook executable not found: %s", path),
		}
	}
	if err != nil {
		return &ValidationError{
			Field:   field,
			Message: fmt.Sprintf("cannot access hook path: %s", err),
		}
	}
	if info.IsDir() {
		return &ValidationError{
			Field:   field,
			Message: fmt.Sprintf("hook path is a directory, not an executable: %s", path),
		}
	}
	// Check for execute permission (on POSIX systems)
	// On Windows, this check is less meaningful but won't cause false negatives
	if info.Mode()&0o111 == 0 {
		return &ValidationError{
			Field:   field,
			Message: fmt.Sprintf("hook file is not executable: %s", path),
		}
	}
	return nil
}
