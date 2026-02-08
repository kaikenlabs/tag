package scaffold

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"
)

const (
	// DefaultHookTimeout is the maximum time a single hook command can run.
	DefaultHookTimeout = 5 * time.Minute

	// MaxHookOutputSize is the maximum size of combined stdout/stderr output to capture.
	// Output beyond this limit is truncated.
	MaxHookOutputSize = 1024 * 1024 // 1MB
)

// HookPhase indicates when a hook runs in the scaffold lifecycle.
type HookPhase string

const (
	// HookPhasePre runs before file generation (working dir: template directory).
	HookPhasePre HookPhase = "pre_scaffold"
	// HookPhasePost runs after file generation (working dir: output directory).
	HookPhasePost HookPhase = "post_scaffold"
	// HookPhasePreGen runs before code generation (working dir: project directory).
	HookPhasePreGen HookPhase = "pre_generate"
	// HookPhasePostGen runs after code generation (working dir: project directory).
	HookPhasePostGen HookPhase = "post_generate"
)

// HookResult contains the result of executing a single hook command.
type HookResult struct {
	Command  string // The command that was executed
	Output   string // Combined stdout and stderr output
	ExitCode int    // Exit code (0 = success)
	Err      error  // Error if execution failed
}

// HookRunner executes hook commands.
type HookRunner interface {
	// Run executes a list of commands sequentially.
	// Returns results for each command executed (may be partial on failure).
	// Returns error if any command fails.
	Run(phase HookPhase, commands []string, workDir string, env []string) ([]HookResult, error)
}

// ShellHookRunner executes hooks via the system shell.
type ShellHookRunner struct{}

// NewHookRunner creates a new hook runner using the system shell.
func NewHookRunner() HookRunner {
	return &ShellHookRunner{}
}

// Run executes commands sequentially via the system shell.
// On Unix: /bin/sh -c "command"
// On Windows: cmd.exe /C "command"
func (r *ShellHookRunner) Run(phase HookPhase, commands []string, workDir string, env []string) ([]HookResult, error) {
	if len(commands) == 0 {
		return nil, nil
	}

	results := make([]HookResult, 0, len(commands))

	for _, cmdStr := range commands {
		result := r.executeCommand(cmdStr, workDir, env)
		results = append(results, result)

		if result.Err != nil {
			return results, NewHookError(phase, cmdStr, result.Output, result.ExitCode, result.Err)
		}
	}

	return results, nil
}

// executeCommand runs a single command via the system shell with timeout.
func (r *ShellHookRunner) executeCommand(cmdStr, workDir string, env []string) HookResult {
	var cmdArgs []string
	if runtime.GOOS == "windows" {
		cmdArgs = []string{"cmd.exe", "/C", cmdStr}
	} else {
		cmdArgs = []string{"/bin/sh", "-c", cmdStr}
	}
	return execWithTimeout(cmdStr, cmdArgs, workDir, env)
}

// execWithTimeout runs a command with timeout, output limiting, and structured result handling.
// cmdDisplay is the human-readable command string for results/errors.
// cmdArgs is the argv array passed to exec.Command.
func execWithTimeout(cmdDisplay string, cmdArgs []string, workDir string, env []string) HookResult {
	result := HookResult{
		Command: cmdDisplay,
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), DefaultHookTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	cmd.Dir = workDir

	// Set environment - ensure we always have a valid environment with PATH
	if len(env) == 0 {
		cmd.Env = os.Environ()
	} else {
		cmd.Env = env
	}

	// Capture stdout and stderr together with size limit
	output := &limitedBuffer{max: MaxHookOutputSize}
	cmd.Stdout = output
	cmd.Stderr = output

	// Execute the command
	err := cmd.Run()
	result.Output = output.String()

	if err != nil {
		result.Err = err
		// Check for context timeout/cancellation
		if ctx.Err() == context.DeadlineExceeded {
			result.Err = fmt.Errorf("hook timed out after %v: %w", DefaultHookTimeout, err)
			result.ExitCode = -1 // Special code for timeout
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			// Extract exit code from ExitError
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1 // Default to 1 for non-exit errors
		}
	}

	return result
}

// limitedBuffer is a bytes.Buffer that stops accepting writes after reaching max size.
type limitedBuffer struct {
	buf       bytes.Buffer
	max       int
	truncated bool
}

func (l *limitedBuffer) Write(p []byte) (n int, err error) {
	if l.truncated {
		return len(p), nil // Discard but report success
	}

	remaining := l.max - l.buf.Len()
	if remaining <= 0 {
		l.truncated = true
		l.buf.WriteString("\n... output truncated (exceeded 1MB limit) ...\n")
		return len(p), nil
	}

	if len(p) > remaining {
		l.buf.Write(p[:remaining])
		l.truncated = true
		l.buf.WriteString("\n... output truncated (exceeded 1MB limit) ...\n")
		return len(p), nil
	}

	return l.buf.Write(p)
}

func (l *limitedBuffer) String() string {
	return l.buf.String()
}

// Ensure limitedBuffer implements io.Writer
var _ io.Writer = (*limitedBuffer)(nil)

// ArgvHookRunner executes hooks using direct argv arrays (no shell interpretation).
// This is safer than ShellHookRunner as it prevents shell injection.
type ArgvHookRunner struct{}

// NewArgvHookRunner creates a new argv-based hook runner.
func NewArgvHookRunner() *ArgvHookRunner {
	return &ArgvHookRunner{}
}

// RunArgv executes commands sequentially using direct argv arrays.
// Each command is a []string where the first element is the program and the rest are arguments.
func (r *ArgvHookRunner) RunArgv(phase HookPhase, commands [][]string, workDir string, env []string) ([]HookResult, error) {
	if len(commands) == 0 {
		return nil, nil
	}

	results := make([]HookResult, 0, len(commands))

	for _, argv := range commands {
		if len(argv) == 0 {
			continue
		}

		// Resolve relative command paths
		cmdPath := argv[0]
		if strings.ContainsAny(cmdPath, "/\\") && !filepath.IsAbs(cmdPath) {
			cmdPath = filepath.Join(workDir, cmdPath)
		}

		execArgv := make([]string, len(argv))
		execArgv[0] = cmdPath
		copy(execArgv[1:], argv[1:])

		cmdDisplay := strings.Join(argv, " ")
		result := execWithTimeout(cmdDisplay, execArgv, workDir, env)
		results = append(results, result)

		if result.Err != nil {
			return results, NewHookError(phase, cmdDisplay, result.Output, result.ExitCode, result.Err)
		}
	}

	return results, nil
}

// BuildHookEnv creates environment variables for hook execution.
// It merges the current environment with TAG-specific variables.
func BuildHookEnv(vars map[string]any, templateDir, outputDir string) []string {
	// Start with current environment
	env := os.Environ()

	// Add TAG-specific variables
	env = append(env, fmt.Sprintf("TAG_TEMPLATE_DIR=%s", templateDir))
	env = append(env, fmt.Sprintf("TAG_OUTPUT_DIR=%s", outputDir))

	// Add project_name as a special variable
	if projectName, ok := vars["project_name"]; ok {
		env = append(env, fmt.Sprintf("TAG_PROJECT_NAME=%s", stringifyValue(projectName)))
	}

	// Add all user variables with TAG_VAR_ prefix
	for name, value := range vars {
		envKey := formatEnvKey(name)
		envValue := stringifyValue(value)
		env = append(env, fmt.Sprintf("%s=%s", envKey, envValue))
	}

	return env
}

// formatEnvKey converts a variable name to an environment variable key.
// Example: "project_name" -> "TAG_VAR_PROJECT_NAME"
// Example: "use-docker" -> "TAG_VAR_USE_DOCKER"
func formatEnvKey(name string) string {
	// Convert to uppercase and replace non-alphanumeric with underscores
	var result strings.Builder
	result.WriteString("TAG_VAR_")

	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			result.WriteRune(unicode.ToUpper(r))
		} else {
			result.WriteRune('_')
		}
	}

	return result.String()
}

// stringifyValue converts a variable value to a string for environment variables.
func stringifyValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case float64:
		// Check if it's an integer
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%g", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case []any, map[string]any:
		// JSON encode complex types
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(data)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// RunPreScaffoldHooks executes pre-scaffold hooks and returns an error if any fail.
// Pre-scaffold hooks run in the template directory before any files are created.
func RunPreScaffoldHooks(runner HookRunner, hooks *HooksConfig, templateDir string, env []string) error {
	if hooks == nil || len(hooks.PreScaffold) == 0 {
		return nil
	}

	fmt.Printf("Running pre-scaffold hooks...\n")

	results, err := runner.Run(HookPhasePre, hooks.PreScaffold, templateDir, env)
	if err != nil {
		// Print output from failed command
		if len(results) > 0 {
			lastResult := results[len(results)-1]
			if lastResult.Output != "" {
				fmt.Printf("Hook output:\n%s\n", lastResult.Output)
			}
		}
		return err
	}

	// Print success output if any
	for _, result := range results {
		if result.Output != "" {
			fmt.Print(result.Output)
			if !strings.HasSuffix(result.Output, "\n") {
				fmt.Println()
			}
		}
	}

	return nil
}

// RunPostScaffoldHooks executes post-scaffold hooks.
// Post-scaffold hooks run in the output directory after all files are created.
// Failures are logged as warnings but don't stop the scaffold process.
func RunPostScaffoldHooks(runner HookRunner, hooks *HooksConfig, outputDir string, env []string) {
	if hooks == nil || len(hooks.PostScaffold) == 0 {
		return
	}

	fmt.Printf("Running post-scaffold hooks...\n")

	results, err := runner.Run(HookPhasePost, hooks.PostScaffold, outputDir, env)

	// Print output from all executed commands
	for _, result := range results {
		if result.Output != "" {
			fmt.Print(result.Output)
			if !strings.HasSuffix(result.Output, "\n") {
				fmt.Println()
			}
		}
	}

	if err != nil {
		// Post-scaffold failures are warnings, not errors
		fmt.Printf("Warning: post-scaffold hook failed: %v\n", err)
		fmt.Printf("Note: Scaffold completed successfully, but some post-scaffold tasks may not have run.\n")
	}
}
