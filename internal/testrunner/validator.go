package testrunner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ValidationResult holds the outcome of running a single validation command.
type ValidationResult struct {
	Command  string
	Output   string
	ExitCode int
	Duration time.Duration
	Err      error
}

// RunValidationCommands executes each command sequentially in the given directory.
// Returns on the first failure if any command fails.
func RunValidationCommands(ctx context.Context, dir string, commands []string, env map[string]string, timeout time.Duration) *ValidationResult {
	for _, cmdStr := range commands {
		result := runSingleCommand(ctx, dir, cmdStr, env, timeout)
		if result.Err != nil || result.ExitCode != 0 {
			return result
		}
	}
	return nil
}

func runSingleCommand(ctx context.Context, dir, cmdStr string, env map[string]string, timeout time.Duration) *ValidationResult {
	start := time.Now()

	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Split command for shell execution.
	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr) // #nosec G204 -- commands from template config
	cmd.Dir = dir

	// Build environment.
	if len(env) > 0 {
		cmd.Env = cmd.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()
	duration := time.Since(start)

	result := &ValidationResult{
		Command:  cmdStr,
		Output:   buf.String(),
		Duration: duration,
	}

	if err != nil {
		result.Err = err
		if exitErr, ok := err.(*exec.ExitError); ok { //nolint:errorlint // need concrete type for ExitCode
			result.ExitCode = exitErr.ExitCode()
		} else if ctx.Err() != nil {
			result.Err = fmt.Errorf("command timed out after %s", timeout)
			result.ExitCode = -1
		} else {
			result.ExitCode = -1
		}
	}

	return result
}

// TruncateOutput limits output to roughly maxLen characters, keeping the last portion
// (which usually contains the error).
func TruncateOutput(output string, maxLen int) string {
	if len(output) <= maxLen {
		return output
	}
	lines := strings.Split(output, "\n")
	var kept []string
	total := 0
	for i := len(lines) - 1; i >= 0; i-- {
		if total+len(lines[i]) > maxLen {
			break
		}
		kept = append([]string{lines[i]}, kept...)
		total += len(lines[i]) + 1
	}
	return "…(truncated)\n" + strings.Join(kept, "\n")
}
