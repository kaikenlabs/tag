package scaffold

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/shlex"
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

// NewHookRunner creates a new hook runner using direct argv execution (no shell interpretation).
func NewHookRunner() HookRunner {
	return &ArgvHookRunner{}
}

// shellMetachars contains characters that indicate shell features (pipes, redirects, etc.)
// which won't work with direct argv execution. Quotes are excluded since shlex handles them.
const shellMetachars = "|&;<>()$`!*?#~"

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

	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...) //nolint:gosec // G204: hook commands are user-approved via --accept-hooks or interactive prompt
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
		} else {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				result.ExitCode = exitErr.ExitCode()
			} else {
				result.ExitCode = 1 // Default to 1 for non-exit errors
			}
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
// It implements HookRunner and also provides RunArgv for pre-split command arrays.
// This is the unified hook runner used by both scaffold and generate commands.
type ArgvHookRunner struct{}

// NewArgvHookRunner creates a new argv-based hook runner.
func NewArgvHookRunner() *ArgvHookRunner {
	return &ArgvHookRunner{}
}

// Run implements HookRunner by splitting shell command strings into argv arrays
// using POSIX shell quoting rules, then executing them directly (no shell interpretation).
// Commands that use shell features (pipes, redirects, variable expansion) must explicitly
// invoke a shell, e.g. "sh -c 'echo hello | grep hello'".
func (r *ArgvHookRunner) Run(phase HookPhase, commands []string, workDir string, env []string) ([]HookResult, error) {
	if len(commands) == 0 {
		return nil, nil
	}

	// Convert string commands to argv arrays
	argvCommands := make([][]string, 0, len(commands))
	for _, cmdStr := range commands {
		// Warn if the command contains shell metacharacters and doesn't already use a shell
		if containsShellMetachars(cmdStr) && !isExplicitShellCommand(cmdStr) {
			fmt.Printf("Warning: hook command %q contains shell metacharacters that won't be interpreted.\n", cmdStr)
			fmt.Printf("  If you need shell features, use: sh -c '%s'\n", cmdStr)
		}

		argv, err := shlex.Split(cmdStr)
		if err != nil {
			return nil, NewHookError(phase, cmdStr, "", 1, fmt.Errorf("failed to parse hook command: %w", err))
		}
		if len(argv) == 0 {
			continue
		}
		argvCommands = append(argvCommands, argv)
	}

	return r.RunArgv(phase, argvCommands, workDir, env)
}

// containsShellMetachars checks if a command string contains shell metacharacters
// that won't work with direct argv execution.
func containsShellMetachars(cmd string) bool {
	return strings.ContainsAny(cmd, shellMetachars)
}

// isExplicitShellCommand checks if a command already invokes a shell explicitly.
func isExplicitShellCommand(cmd string) bool {
	trimmed := strings.TrimSpace(cmd)
	for _, prefix := range []string{"sh -c ", "bash -c ", "/bin/sh -c ", "/bin/bash -c ", "cmd /C ", "cmd.exe /C "} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
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

		execArgv = resolveInterpreter(execArgv, workDir)

		cmdDisplay := strings.Join(argv, " ")
		result := execWithTimeout(cmdDisplay, execArgv, workDir, env)
		results = append(results, result)

		if result.Err != nil {
			return results, NewHookError(phase, cmdDisplay, result.Output, result.ExitCode, result.Err)
		}
	}

	return results, nil
}

// ConfirmHooks checks whether hooks should be executed based on flags and user confirmation.
// Returns true if hooks should run, false if they should be skipped.
// Returns an error only if prompting fails.
func ConfirmHooks(hooks *HooksConfig, acceptHooks, noInput bool, prompter Prompter) (bool, error) {
	if len(collectAllHooks(hooks)) == 0 {
		return false, nil
	}

	// --accept-hooks: run without prompting
	if acceptHooks {
		return true, nil
	}

	// --no-input without --accept-hooks: skip hooks
	if noInput {
		fmt.Println("Skipping hooks (use --accept-hooks to run them in non-interactive mode).")
		return false, nil
	}

	// Interactive: display hooks and prompt for confirmation
	displayHookList(hooks)

	confirmed, err := prompter.Confirm("Do you want to execute these hooks?", false)
	if err != nil {
		return false, fmt.Errorf("failed to confirm hooks: %w", err)
	}

	if !confirmed {
		fmt.Println("Hooks skipped by user choice.")
	}

	return confirmed, nil
}

// collectAllHooks returns all hooks (pre + post) from the config.
func collectAllHooks(hooks *HooksConfig) []string {
	if hooks == nil {
		return nil
	}
	all := make([]string, 0, len(hooks.PreScaffold)+len(hooks.PostScaffold))
	all = append(all, hooks.PreScaffold...)
	all = append(all, hooks.PostScaffold...)
	return all
}

// displayHookList prints the list of configured hooks to stdout.
func displayHookList(hooks *HooksConfig) {
	fmt.Println("This template defines the following hooks:")
	if len(hooks.PreScaffold) > 0 {
		fmt.Println("  Pre-scaffold:")
		for _, cmd := range hooks.PreScaffold {
			fmt.Printf("    - %s\n", cmd)
		}
	}
	if len(hooks.PostScaffold) > 0 {
		fmt.Println("  Post-scaffold:")
		for _, cmd := range hooks.PostScaffold {
			fmt.Printf("    - %s\n", cmd)
		}
	}
}

// PrintHookResults prints the output of hook execution results to stdout.
func PrintHookResults(results []HookResult) {
	for _, result := range results {
		if result.Output != "" {
			fmt.Print(result.Output)
			if !strings.HasSuffix(result.Output, "\n") {
				fmt.Println()
			}
		}
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

	PrintHookResults(results)

	return err
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

	PrintHookResults(results)

	if err != nil {
		// Post-scaffold failures are warnings, not errors
		fmt.Printf("Warning: post-scaffold hook failed: %v\n", err)
		fmt.Printf("Note: Scaffold completed successfully, but some post-scaffold tasks may not have run.\n")
	}
}

// RunArgvHooks executes pre-split argv hook commands and prints their output.
// This is used by the generate command which stores hooks as [][]string.
// Returns an error if any command fails.
func RunArgvHooks(phase HookPhase, hooks [][]string, workDir string, env []string) ([]HookResult, error) {
	if len(hooks) == 0 {
		return nil, nil
	}

	runner := NewArgvHookRunner()
	return runner.RunArgv(phase, hooks, workDir, env)
}
