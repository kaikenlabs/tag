package scaffold

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockHookRunner is a mock implementation of HookRunner for testing.
type MockHookRunner struct {
	RunFunc func(phase HookPhase, commands []string, workDir string, env []string) ([]HookResult, error)
	Calls   []MockHookCall
}

type MockHookCall struct {
	Phase    HookPhase
	Commands []string
	WorkDir  string
	Env      []string
}

func (m *MockHookRunner) Run(phase HookPhase, commands []string, workDir string, env []string) ([]HookResult, error) {
	m.Calls = append(m.Calls, MockHookCall{
		Phase:    phase,
		Commands: commands,
		WorkDir:  workDir,
		Env:      env,
	})
	if m.RunFunc != nil {
		return m.RunFunc(phase, commands, workDir, env)
	}
	// Default: return success for all commands
	results := make([]HookResult, len(commands))
	for i, cmd := range commands {
		results[i] = HookResult{Command: cmd, ExitCode: 0}
	}
	return results, nil
}

// --- Unit Tests for BuildHookEnv ---

func TestUT_BuildHookEnv_BasicVariables(t *testing.T) {
	vars := map[string]any{
		"project_name": "my_project",
		"author":       "John Doe",
	}

	env := BuildHookEnv(vars, "/template", "/output")

	// Check TAG-specific variables
	assertEnvContains(t, env, "TAG_TEMPLATE_DIR=/template")
	assertEnvContains(t, env, "TAG_OUTPUT_DIR=/output")
	assertEnvContains(t, env, "TAG_PROJECT_NAME=my_project")
	assertEnvContains(t, env, "TAG_VAR_PROJECT_NAME=my_project")
	assertEnvContains(t, env, "TAG_VAR_AUTHOR=John Doe")
}

func TestUT_BuildHookEnv_BooleanVariables(t *testing.T) {
	vars := map[string]any{
		"use_docker": true,
		"use_ci":     false,
	}

	env := BuildHookEnv(vars, "/template", "/output")

	assertEnvContains(t, env, "TAG_VAR_USE_DOCKER=true")
	assertEnvContains(t, env, "TAG_VAR_USE_CI=false")
}

func TestUT_BuildHookEnv_NumberVariables(t *testing.T) {
	vars := map[string]any{
		"port":    float64(8080),
		"version": float64(1.5),
	}

	env := BuildHookEnv(vars, "/template", "/output")

	assertEnvContains(t, env, "TAG_VAR_PORT=8080")
	assertEnvContains(t, env, "TAG_VAR_VERSION=1.5")
}

func TestUT_BuildHookEnv_ComplexVariables(t *testing.T) {
	vars := map[string]any{
		"features": []any{"auth", "logging", "metrics"},
		"config": map[string]any{
			"key": "value",
		},
	}

	env := BuildHookEnv(vars, "/template", "/output")

	// Complex types should be JSON encoded
	assertEnvContainsPrefix(t, env, "TAG_VAR_FEATURES=")
	assertEnvContainsPrefix(t, env, "TAG_VAR_CONFIG=")

	// Verify JSON encoding
	for _, e := range env {
		value, found := strings.CutPrefix(e, "TAG_VAR_FEATURES=")
		if !found {
			continue
		}
		var arr []string
		err := json.Unmarshal([]byte(value), &arr)
		require.NoError(t, err)
		assert.Equal(t, []string{"auth", "logging", "metrics"}, arr)
	}
}

func TestUT_BuildHookEnv_SpecialCharactersInNames(t *testing.T) {
	vars := map[string]any{
		"project-name":  "test",
		"use_docker":    true,
		"CamelCaseVar":  "value",
		"with.dots":     "dotted",
		"with spaces":   "spaced",
		"with@special!": "special",
	}

	env := BuildHookEnv(vars, "/template", "/output")

	// All should be converted to uppercase with underscores
	assertEnvContains(t, env, "TAG_VAR_PROJECT_NAME=test")
	assertEnvContains(t, env, "TAG_VAR_USE_DOCKER=true")
	assertEnvContains(t, env, "TAG_VAR_CAMELCASEVAR=value")
	assertEnvContains(t, env, "TAG_VAR_WITH_DOTS=dotted")
	assertEnvContains(t, env, "TAG_VAR_WITH_SPACES=spaced")
	assertEnvContains(t, env, "TAG_VAR_WITH_SPECIAL_=special")
}

func TestUT_BuildHookEnv_EmptyVars(t *testing.T) {
	vars := map[string]any{}

	env := BuildHookEnv(vars, "/template", "/output")

	// Should still have TAG_TEMPLATE_DIR and TAG_OUTPUT_DIR
	assertEnvContains(t, env, "TAG_TEMPLATE_DIR=/template")
	assertEnvContains(t, env, "TAG_OUTPUT_DIR=/output")
	// Should not have TAG_PROJECT_NAME if not in vars
	assertEnvNotContainsPrefix(t, env, "TAG_PROJECT_NAME=")
}

func TestUT_BuildHookEnv_IncludesSystemEnv(t *testing.T) {
	vars := map[string]any{}

	env := BuildHookEnv(vars, "/template", "/output")

	// Should include PATH from system environment
	hasPath := false
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") || strings.HasPrefix(e, "Path=") {
			hasPath = true
			break
		}
	}
	assert.True(t, hasPath, "should include system PATH")
}

// --- Unit Tests for sanitizeEnvValue ---

func TestUT_SanitizeEnvValue_Normal(t *testing.T) {
	result := sanitizeEnvValue("test", "hello world")
	assert.Equal(t, "hello world", result)
}

func TestUT_SanitizeEnvValue_StripsNewlines(t *testing.T) {
	result := sanitizeEnvValue("test", "line1\nline2\rline3")
	assert.Equal(t, "line1 line2 line3", result)
}

func TestUT_SanitizeEnvValue_TruncatesLongValues(t *testing.T) {
	longValue := strings.Repeat("a", MaxEnvValueLen+100)
	result := sanitizeEnvValue("test", longValue)
	assert.Len(t, result, MaxEnvValueLen)
}

func TestUT_SanitizeEnvValue_EmptyString(t *testing.T) {
	result := sanitizeEnvValue("test", "")
	assert.Equal(t, "", result)
}

func TestUT_BuildHookEnv_SanitizesValues(t *testing.T) {
	vars := map[string]any{
		"project_name": "test\ninjection",
	}

	env := BuildHookEnv(vars, "/template", "/output")

	// Newlines should be replaced with spaces
	for _, e := range env {
		value, found := strings.CutPrefix(e, "TAG_VAR_PROJECT_NAME=")
		if !found {
			continue
		}
		assert.NotContains(t, value, "\n")
		assert.Equal(t, "test injection", value)
		return
	}
	t.Error("TAG_VAR_PROJECT_NAME not found in env")
}

// --- Unit Tests for formatEnvKey ---

func TestUT_FormatEnvKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"project_name", "TAG_VAR_PROJECT_NAME"},
		{"projectName", "TAG_VAR_PROJECTNAME"},
		{"project-name", "TAG_VAR_PROJECT_NAME"},
		{"use.docker", "TAG_VAR_USE_DOCKER"},
		{"CamelCase", "TAG_VAR_CAMELCASE"},
		{"ALREADY_UPPER", "TAG_VAR_ALREADY_UPPER"},
		{"123numeric", "TAG_VAR_123NUMERIC"},
		{"with spaces", "TAG_VAR_WITH_SPACES"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := formatEnvKey(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- Unit Tests for stringifyValue ---

func TestUT_StringifyValue(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"string", "hello", "hello"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"integer float", float64(42), "42"},
		{"decimal float", float64(3.14), "3.14"},
		{"int", int(100), "100"},
		{"int64", int64(999), "999"},
		{"slice", []any{"a", "b"}, `["a","b"]`},
		{"map", map[string]any{"k": "v"}, `{"k":"v"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringifyValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- Unit Tests for ArgvHookRunner.Run (string command parsing) ---

func TestUT_ArgvHookRunner_Run_SuccessfulCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	runner := &ArgvHookRunner{}
	dir := t.TempDir()

	results, err := runner.Run(HookPhasePre, []string{"echo hello"}, dir, os.Environ())

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "echo hello", results[0].Command)
	assert.Equal(t, 0, results[0].ExitCode)
	assert.Contains(t, results[0].Output, "hello")
}

func TestUT_ArgvHookRunner_Run_MultipleCommands(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	runner := &ArgvHookRunner{}
	dir := t.TempDir()

	results, err := runner.Run(HookPhasePre, []string{"echo first", "echo second"}, dir, os.Environ())

	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Contains(t, results[0].Output, "first")
	assert.Contains(t, results[1].Output, "second")
}

func TestUT_ArgvHookRunner_Run_CommandFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	runner := &ArgvHookRunner{}
	dir := t.TempDir()

	// Use false (always exits with 1) instead of shell builtin "exit 1"
	results, err := runner.Run(HookPhasePre, []string{"false", "echo should not run"}, dir, os.Environ())

	require.Error(t, err)
	require.Len(t, results, 1) // Only first command was executed
	assert.Equal(t, 1, results[0].ExitCode)

	// Verify it's a HookError
	hookErr, ok := err.(*HookError)
	require.True(t, ok)
	assert.Equal(t, HookPhasePre, hookErr.Phase)
}

func TestUT_ArgvHookRunner_Run_MiddleCommandFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	runner := &ArgvHookRunner{}
	dir := t.TempDir()

	// "false" exits with 1 without needing a shell
	results, err := runner.Run(HookPhasePre, []string{"echo first", "false", "echo third"}, dir, os.Environ())

	require.Error(t, err)
	require.Len(t, results, 2) // First two commands were executed
	assert.Equal(t, 0, results[0].ExitCode)
	assert.Equal(t, 1, results[1].ExitCode)
}

func TestUT_ArgvHookRunner_Run_EmptyCommands(t *testing.T) {
	runner := &ArgvHookRunner{}
	dir := t.TempDir()

	results, err := runner.Run(HookPhasePre, []string{}, dir, os.Environ())

	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestUT_ArgvHookRunner_Run_NilCommands(t *testing.T) {
	runner := &ArgvHookRunner{}
	dir := t.TempDir()

	results, err := runner.Run(HookPhasePre, nil, dir, os.Environ())

	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestUT_ArgvHookRunner_Run_WorkingDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	runner := &ArgvHookRunner{}
	dir := t.TempDir()

	results, err := runner.Run(HookPhasePre, []string{"pwd"}, dir, os.Environ())

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Output, dir)
}

func TestUT_ArgvHookRunner_Run_QuotedArgs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	runner := &ArgvHookRunner{}
	dir := t.TempDir()

	// shlex should properly split quoted arguments
	results, err := runner.Run(HookPhasePre, []string{`echo "hello world"`}, dir, os.Environ())

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Output, "hello world")
}

func TestUT_ArgvHookRunner_Run_ExplicitShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	runner := &ArgvHookRunner{}
	dir := t.TempDir()
	env := append(os.Environ(), "TAG_TEST_VAR=test_value")

	// Users can explicitly invoke shell for shell features
	results, err := runner.Run(HookPhasePre, []string{"sh -c 'echo $TAG_TEST_VAR'"}, dir, env)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Output, "test_value")
}

func TestUT_ArgvHookRunner_Run_EmptyEnvUsesSystemEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	runner := &ArgvHookRunner{}
	dir := t.TempDir()

	// With nil env, should still work (uses os.Environ internally)
	results, err := runner.Run(HookPhasePre, []string{"echo hello"}, dir, nil)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Output, "hello")
}

func TestUT_ContainsShellMetachars(t *testing.T) {
	tests := []struct {
		cmd      string
		expected bool
	}{
		{"echo hello", false},
		{"echo hello | grep hello", true},
		{"echo hello > file.txt", true},
		{"echo $VAR", true},
		{"echo hello && echo world", true},
		{"echo hello; echo world", true},
		{`echo "quoted string"`, false}, // quotes are handled by shlex
		{"sh -c 'echo hello'", false},   // quotes are handled by shlex
		{"pwd", false},
		{"ls -la", false},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			result := containsShellMetachars(tt.cmd)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_ArgvHookRunner_Run_MetacharsAreLiteral(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	runner := &ArgvHookRunner{}
	dir := t.TempDir()

	// Without shell, > is passed as a literal argument, not a redirect
	results, err := runner.Run(HookPhasePre, []string{"echo hello > output.txt"}, dir, os.Environ())

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, 0, results[0].ExitCode)
	// echo receives ">", "output.txt" as arguments, printing them literally
	assert.Contains(t, results[0].Output, ">")
	assert.Contains(t, results[0].Output, "output.txt")

	// Verify no file was created by redirection
	_, err = os.Stat(filepath.Join(dir, "output.txt"))
	assert.True(t, os.IsNotExist(err), "hook should not have created a file via redirection")
}

func TestUT_IsExplicitShellCommand(t *testing.T) {
	tests := []struct {
		cmd      string
		expected bool
	}{
		{"sh -c 'echo hello'", true},
		{"/bin/sh -c 'echo hello'", true},
		{"bash -c 'echo hello'", true},
		{"/bin/bash -c 'echo hello'", true},
		{"cmd /C echo hello", true},
		{"cmd.exe /C echo hello", true},
		{"  sh -c 'indented'", true},
		{"echo hello", false},
		{"sh", false},
		{"sh -v", false},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			result := isExplicitShellCommand(tt.cmd)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_RunArgvHooks_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	dir := t.TempDir()

	results, err := RunArgvHooks(HookPhasePreGen, [][]string{{"echo", "hello"}}, dir, os.Environ())

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Output, "hello")
}

func TestUT_RunArgvHooks_EmptyCommands(t *testing.T) {
	results, err := RunArgvHooks(HookPhasePreGen, nil, "", nil)

	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestUT_LimitedBuffer_TruncatesLargeOutput(t *testing.T) {
	buf := &limitedBuffer{max: 100}

	// Write 50 bytes - should succeed
	n, err := buf.Write([]byte(strings.Repeat("a", 50)))
	assert.NoError(t, err)
	assert.Equal(t, 50, n)
	assert.Len(t, buf.String(), 50)

	// Write 60 more bytes - should be truncated
	n, err = buf.Write([]byte(strings.Repeat("b", 60)))
	assert.NoError(t, err)
	assert.Equal(t, 60, n) // Reports full write but truncates internally

	output := buf.String()
	assert.Contains(t, output, "truncated")
	assert.True(t, len(output) < 200) // Much less than 110 + message
}

func TestUT_LimitedBuffer_HandlesExactLimit(t *testing.T) {
	buf := &limitedBuffer{max: 10}

	// Write exactly 10 bytes
	n, err := buf.Write([]byte("1234567890"))
	assert.NoError(t, err)
	assert.Equal(t, 10, n)
	assert.Equal(t, "1234567890", buf.String())

	// Next write should trigger truncation
	n, err = buf.Write([]byte("X"))
	assert.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Contains(t, buf.String(), "truncated")
}

func TestUT_LimitedBuffer_DiscardsAfterTruncation(t *testing.T) {
	buf := &limitedBuffer{max: 5}

	// Fill the buffer and trigger truncation
	_, _ = buf.Write([]byte("12345"))
	_, _ = buf.Write([]byte("overflow"))

	snapshot := buf.String()

	// Additional writes should be discarded
	_, _ = buf.Write([]byte("more data"))
	_, _ = buf.Write([]byte("even more"))

	// Output shouldn't grow beyond the truncation message
	assert.Equal(t, snapshot, buf.String())
}

// --- Unit Tests for ConfirmHooks ---

// MockPrompterForHooks records calls and returns preset values.
type MockPrompterForHooks struct {
	ConfirmResult bool
	ConfirmErr    error
	ConfirmCalls  int
}

func (m *MockPrompterForHooks) Input(label, defaultValue string, secret bool) (string, error) {
	return defaultValue, nil
}

func (m *MockPrompterForHooks) Select(label string, options []string, defaultIndex int) (string, error) {
	if defaultIndex >= 0 && defaultIndex < len(options) {
		return options[defaultIndex], nil
	}
	return options[0], nil
}

func (m *MockPrompterForHooks) Confirm(label string, defaultValue bool) (bool, error) {
	m.ConfirmCalls++
	return m.ConfirmResult, m.ConfirmErr
}

func (m *MockPrompterForHooks) Number(label string, defaultValue float64) (float64, error) {
	return defaultValue, nil
}

func TestUT_ConfirmHooks_AcceptHooksFlag_SkipsPrompt(t *testing.T) {
	hooks := &HooksConfig{
		PreScaffold: []string{"echo pre"},
	}
	prompter := &MockPrompterForHooks{}

	allowed, err := ConfirmHooks(hooks, true, false, prompter)

	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 0, prompter.ConfirmCalls, "should not prompt when AcceptHooks is true")
}

func TestUT_ConfirmHooks_NoInputFlag_SkipsHooks(t *testing.T) {
	hooks := &HooksConfig{
		PreScaffold: []string{"echo pre"},
	}
	prompter := &MockPrompterForHooks{}

	allowed, err := ConfirmHooks(hooks, false, true, prompter)

	require.NoError(t, err)
	assert.False(t, allowed)
	assert.Equal(t, 0, prompter.ConfirmCalls, "should not prompt when NoInput is true")
}

func TestUT_ConfirmHooks_InteractiveConfirmed(t *testing.T) {
	hooks := &HooksConfig{
		PreScaffold:  []string{"echo pre"},
		PostScaffold: []string{"echo post"},
	}
	prompter := &MockPrompterForHooks{ConfirmResult: true}

	allowed, err := ConfirmHooks(hooks, false, false, prompter)

	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 1, prompter.ConfirmCalls)
}

func TestUT_ConfirmHooks_InteractiveDenied(t *testing.T) {
	hooks := &HooksConfig{
		PreScaffold: []string{"echo pre"},
	}
	prompter := &MockPrompterForHooks{ConfirmResult: false}

	allowed, err := ConfirmHooks(hooks, false, false, prompter)

	require.NoError(t, err)
	assert.False(t, allowed)
	assert.Equal(t, 1, prompter.ConfirmCalls)
}

func TestUT_ConfirmHooks_NilHooks_ReturnsFalse(t *testing.T) {
	prompter := &MockPrompterForHooks{}

	allowed, err := ConfirmHooks(nil, false, false, prompter)

	require.NoError(t, err)
	assert.False(t, allowed)
	assert.Equal(t, 0, prompter.ConfirmCalls)
}

func TestUT_ConfirmHooks_EmptyHooks_ReturnsFalse(t *testing.T) {
	hooks := &HooksConfig{}
	prompter := &MockPrompterForHooks{}

	allowed, err := ConfirmHooks(hooks, false, false, prompter)

	require.NoError(t, err)
	assert.False(t, allowed)
	assert.Equal(t, 0, prompter.ConfirmCalls)
}

func TestUT_ConfirmHooks_AcceptHooksOverridesInteractive(t *testing.T) {
	hooks := &HooksConfig{
		PreScaffold: []string{"echo pre"},
	}
	prompter := &MockPrompterForHooks{}

	// AcceptHooks=true should run even in interactive mode (no prompt)
	allowed, err := ConfirmHooks(hooks, true, false, prompter)

	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 0, prompter.ConfirmCalls)
}

func TestUT_ConfirmHooks_PromptError(t *testing.T) {
	hooks := &HooksConfig{
		PreScaffold: []string{"echo pre"},
	}
	prompter := &MockPrompterForHooks{
		ConfirmErr: assert.AnError,
	}

	_, err := ConfirmHooks(hooks, false, false, prompter)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to confirm hooks")
}

func TestIT_Scaffold_HooksSkippedInNoInputMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	// Create template with hooks that create marker files
	templateDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "output")

	config := map[string]any{
		"vars": map[string]any{
			"project_name": "test_project",
		},
		"hooks": map[string]any{
			"post_scaffold": []string{"touch hook_ran.txt"},
		},
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "tag.template.json"), configData, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "test.txt"), []byte("content"), 0o644))

	opts := Options{
		TemplateDir: templateDir,
		OutputDir:   outputDir,
		NoInput:     true,
		// AcceptHooks is false - hooks should be skipped
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.NoError(t, err)

	// Output should be created but hook marker should NOT exist
	assert.DirExists(t, outputDir)
	assert.FileExists(t, filepath.Join(outputDir, "test.txt"))
	assert.NoFileExists(t, filepath.Join(outputDir, "hook_ran.txt"), "hooks should be skipped without --accept-hooks")
}

// --- Integration Tests for Scaffold with Hooks ---

func TestIT_Scaffold_PreHookSuccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	// Create template with pre-scaffold hook
	templateDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "output")

	config := map[string]any{
		"vars": map[string]any{
			"project_name": "test_project",
		},
		"hooks": map[string]any{
			"pre_scaffold": []string{"echo pre-hook executed"},
		},
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "tag.template.json"), configData, 0o644))

	// Create a simple template file
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "test.txt"), []byte("content"), 0o644))

	opts := Options{
		TemplateDir: templateDir,
		OutputDir:   outputDir,
		NoInput:     true,
		AcceptHooks: true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.NoError(t, err)

	// Verify output was created
	assert.DirExists(t, outputDir)
	assert.FileExists(t, filepath.Join(outputDir, "test.txt"))
}

func TestIT_Scaffold_PreHookFailure_NoOutputCreated(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	// Create template with failing pre-scaffold hook
	templateDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "output")

	config := map[string]any{
		"vars": map[string]any{
			"project_name": "test_project",
		},
		"hooks": map[string]any{
			"pre_scaffold": []string{"false"},
		},
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "tag.template.json"), configData, 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "test.txt"), []byte("content"), 0o644))

	opts := Options{
		TemplateDir: templateDir,
		OutputDir:   outputDir,
		NoInput:     true,
		AcceptHooks: true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pre-scaffold hook failed")

	// Verify output was NOT created (clean up symlink-resolved path too)
	assert.NoDirExists(t, outputDir)
}

func TestIT_Scaffold_PostHookSuccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	// Create template with post-scaffold hook that creates a marker file
	templateDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "output")

	config := map[string]any{
		"vars": map[string]any{
			"project_name": "test_project",
		},
		"hooks": map[string]any{
			"post_scaffold": []string{"touch post_hook_marker.txt"},
		},
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "tag.template.json"), configData, 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "test.txt"), []byte("content"), 0o644))

	opts := Options{
		TemplateDir: templateDir,
		OutputDir:   outputDir,
		NoInput:     true,
		AcceptHooks: true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.NoError(t, err)

	// Verify output and marker file were created
	assert.DirExists(t, outputDir)
	assert.FileExists(t, filepath.Join(outputDir, "test.txt"))
	assert.FileExists(t, filepath.Join(outputDir, "post_hook_marker.txt"))
}

func TestIT_Scaffold_PostHookFailure_OutputPreserved(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	// Create template with failing post-scaffold hook
	templateDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "output")

	config := map[string]any{
		"vars": map[string]any{
			"project_name": "test_project",
		},
		"hooks": map[string]any{
			"post_scaffold": []string{"false"},
		},
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "tag.template.json"), configData, 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "test.txt"), []byte("content"), 0o644))

	opts := Options{
		TemplateDir: templateDir,
		OutputDir:   outputDir,
		NoInput:     true,
		AcceptHooks: true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	// Post-hook failures should NOT cause scaffold to fail
	err = s.Run(opts)
	require.NoError(t, err)

	// Verify output was still created despite hook failure
	assert.DirExists(t, outputDir)
	assert.FileExists(t, filepath.Join(outputDir, "test.txt"))
}

func TestIT_Scaffold_HooksReceiveEnvironmentVariables(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	// Create template that writes env vars to a file
	// Uses explicit shell invocation since variable expansion requires a shell
	templateDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "output")

	config := map[string]any{
		"vars": map[string]any{
			"project_name": "env_test_project",
			"author":       "Test Author",
		},
		"hooks": map[string]any{
			"post_scaffold": []string{
				"sh -c 'echo TAG_PROJECT_NAME=$TAG_PROJECT_NAME > env_check.txt'",
				"sh -c 'echo TAG_VAR_PROJECT_NAME=$TAG_VAR_PROJECT_NAME >> env_check.txt'",
				"sh -c 'echo TAG_VAR_AUTHOR=$TAG_VAR_AUTHOR >> env_check.txt'",
				"sh -c 'echo TAG_OUTPUT_DIR=$TAG_OUTPUT_DIR >> env_check.txt'",
			},
		},
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "tag.template.json"), configData, 0o644))

	opts := Options{
		TemplateDir: templateDir,
		OutputDir:   outputDir,
		NoInput:     true,
		AcceptHooks: true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.NoError(t, err)

	// Read and verify env check file
	envContent, err := os.ReadFile(filepath.Join(outputDir, "env_check.txt"))
	require.NoError(t, err)

	content := string(envContent)
	assert.Contains(t, content, "TAG_PROJECT_NAME=env_test_project")
	assert.Contains(t, content, "TAG_VAR_PROJECT_NAME=env_test_project")
	assert.Contains(t, content, "TAG_VAR_AUTHOR=Test Author")
	assert.Contains(t, content, "TAG_OUTPUT_DIR="+outputDir)
}

func TestIT_Scaffold_PreHooksRunInTemplateDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	// Create template with pre-hook that writes pwd to a marker file
	// Uses explicit shell invocation since redirect requires a shell
	templateDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "output")

	config := map[string]any{
		"vars": map[string]any{
			"project_name": "test_project",
		},
		"hooks": map[string]any{
			"pre_scaffold": []string{"sh -c 'pwd > pre_hook_pwd.txt'"},
		},
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "tag.template.json"), configData, 0o644))

	opts := Options{
		TemplateDir: templateDir,
		OutputDir:   outputDir,
		NoInput:     true,
		AcceptHooks: true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.NoError(t, err)

	// Read the marker file created in template directory
	pwdContent, err := os.ReadFile(filepath.Join(templateDir, "pre_hook_pwd.txt"))
	require.NoError(t, err)
	assert.Contains(t, string(pwdContent), templateDir)
}

func TestIT_Scaffold_PostHooksRunInOutputDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	// Create template with post-hook that writes pwd to a marker file
	// Uses explicit shell invocation since redirect requires a shell
	templateDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "output")

	config := map[string]any{
		"vars": map[string]any{
			"project_name": "test_project",
		},
		"hooks": map[string]any{
			"post_scaffold": []string{"sh -c 'pwd > post_hook_pwd.txt'"},
		},
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "tag.template.json"), configData, 0o644))

	opts := Options{
		TemplateDir: templateDir,
		OutputDir:   outputDir,
		NoInput:     true,
		AcceptHooks: true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.NoError(t, err)

	// Read the marker file created in output directory
	pwdContent, err := os.ReadFile(filepath.Join(outputDir, "post_hook_pwd.txt"))
	require.NoError(t, err)
	assert.Contains(t, string(pwdContent), outputDir)
}

func TestIT_Scaffold_MultipleHooksExecuteInOrder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	// Create template with multiple hooks that append to a file
	// Uses explicit shell invocation since redirects require a shell
	templateDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "output")

	config := map[string]any{
		"vars": map[string]any{
			"project_name": "test_project",
		},
		"hooks": map[string]any{
			"post_scaffold": []string{
				"sh -c 'echo first > order.txt'",
				"sh -c 'echo second >> order.txt'",
				"sh -c 'echo third >> order.txt'",
			},
		},
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "tag.template.json"), configData, 0o644))

	opts := Options{
		TemplateDir: templateDir,
		OutputDir:   outputDir,
		NoInput:     true,
		AcceptHooks: true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.NoError(t, err)

	// Verify hooks ran in order
	orderContent, err := os.ReadFile(filepath.Join(outputDir, "order.txt"))
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(orderContent)), "\n")
	require.Len(t, lines, 3)
	assert.Equal(t, "first", lines[0])
	assert.Equal(t, "second", lines[1])
	assert.Equal(t, "third", lines[2])
}

func TestIT_Scaffold_NoHooksConfigured(t *testing.T) {
	// Create template without hooks
	templateDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "output")

	config := map[string]any{
		"vars": map[string]any{
			"project_name": "test_project",
		},
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "tag.template.json"), configData, 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "test.txt"), []byte("content"), 0o644))

	opts := Options{
		TemplateDir: templateDir,
		OutputDir:   outputDir,
		NoInput:     true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.NoError(t, err)

	// Verify output was created successfully
	assert.DirExists(t, outputDir)
	assert.FileExists(t, filepath.Join(outputDir, "test.txt"))
}

// --- Helper functions ---

func assertEnvContains(t *testing.T, env []string, expected string) {
	t.Helper()
	if !slices.Contains(env, expected) {
		t.Errorf("expected env to contain %q, but it didn't.\nEnv: %v", expected, filterTagEnv(env))
	}
}

func assertEnvContainsPrefix(t *testing.T, env []string, prefix string) {
	t.Helper()
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return
		}
	}
	t.Errorf("expected env to contain entry with prefix %q, but it didn't.\nEnv: %v", prefix, filterTagEnv(env))
}

func assertEnvNotContainsPrefix(t *testing.T, env []string, prefix string) {
	t.Helper()
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			t.Errorf("expected env NOT to contain entry with prefix %q, but found %q", prefix, e)
			return
		}
	}
}

// filterTagEnv returns only TAG_ prefixed env vars for easier debugging
func filterTagEnv(env []string) []string {
	var result []string
	for _, e := range env {
		if strings.HasPrefix(e, "TAG_") {
			result = append(result, e)
		}
	}
	return result
}
