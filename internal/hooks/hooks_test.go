package hooks

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/google/shlex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/types"
)

// --- Unit Tests for BuildHookEnv ---

func TestUT_BuildHookEnv_BasicVariables(t *testing.T) {
	vars := map[string]any{
		"project_name": "my_project",
		"author":       "John Doe",
	}

	env := BuildHookEnv(vars, "/template", "/output", io.Discard)

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

	env := BuildHookEnv(vars, "/template", "/output", io.Discard)

	assertEnvContains(t, env, "TAG_VAR_USE_DOCKER=true")
	assertEnvContains(t, env, "TAG_VAR_USE_CI=false")
}

func TestUT_BuildHookEnv_NumberVariables(t *testing.T) {
	vars := map[string]any{
		"port":    float64(8080),
		"version": float64(1.5),
	}

	env := BuildHookEnv(vars, "/template", "/output", io.Discard)

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

	env := BuildHookEnv(vars, "/template", "/output", io.Discard)

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

	env := BuildHookEnv(vars, "/template", "/output", io.Discard)

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

	env := BuildHookEnv(vars, "/template", "/output", io.Discard)

	// Should still have TAG_TEMPLATE_DIR and TAG_OUTPUT_DIR
	assertEnvContains(t, env, "TAG_TEMPLATE_DIR=/template")
	assertEnvContains(t, env, "TAG_OUTPUT_DIR=/output")
	// Should not have TAG_PROJECT_NAME if not in vars
	assertEnvNotContainsPrefix(t, env, "TAG_PROJECT_NAME=")
}

func TestUT_BuildHookEnv_IncludesSystemEnv(t *testing.T) {
	vars := map[string]any{}

	env := BuildHookEnv(vars, "/template", "/output", io.Discard)

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
	result := sanitizeEnvValue("test", "hello world", io.Discard)
	assert.Equal(t, "hello world", result)
}

func TestUT_SanitizeEnvValue_StripsNewlines(t *testing.T) {
	result := sanitizeEnvValue("test", "line1\nline2\rline3", io.Discard)
	assert.Equal(t, "line1 line2 line3", result)
}

func TestUT_SanitizeEnvValue_TruncatesLongValues(t *testing.T) {
	longValue := strings.Repeat("a", MaxEnvValueLen+100)
	result := sanitizeEnvValue("test", longValue, io.Discard)
	assert.Len(t, result, MaxEnvValueLen)
}

func TestUT_SanitizeEnvValue_EmptyString(t *testing.T) {
	result := sanitizeEnvValue("test", "", io.Discard)
	assert.Equal(t, "", result)
}

func TestUT_BuildHookEnv_SanitizesValues(t *testing.T) {
	vars := map[string]any{
		"project_name": "test\ninjection",
	}

	env := BuildHookEnv(vars, "/template", "/output", io.Discard)

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

// MockConfirmer implements the Confirmer interface for testing.
type MockConfirmer struct {
	ConfirmResult bool
	ConfirmErr    error
	ConfirmCalls  int
}

func (m *MockConfirmer) Confirm(label string, defaultValue bool) (bool, error) {
	m.ConfirmCalls++
	return m.ConfirmResult, m.ConfirmErr
}

func TestUT_ConfirmHooks_AcceptHooksFlag_SkipsPrompt(t *testing.T) {
	h := &types.HooksConfig{
		PreScaffold: []string{"echo pre"},
	}
	confirmer := &MockConfirmer{}

	allowed, err := ConfirmHooks(h, true, false, confirmer, "", io.Discard)

	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 0, confirmer.ConfirmCalls, "should not prompt when AcceptHooks is true")
}

func TestUT_ConfirmHooks_NoInputFlag_SkipsHooks(t *testing.T) {
	h := &types.HooksConfig{
		PreScaffold: []string{"echo pre"},
	}
	confirmer := &MockConfirmer{}

	allowed, err := ConfirmHooks(h, false, true, confirmer, "", io.Discard)

	require.NoError(t, err)
	assert.False(t, allowed)
	assert.Equal(t, 0, confirmer.ConfirmCalls, "should not prompt when NoInput is true")
}

func TestUT_ConfirmHooks_InteractiveConfirmed(t *testing.T) {
	h := &types.HooksConfig{
		PreScaffold:  []string{"echo pre"},
		PostScaffold: []string{"echo post"},
	}
	confirmer := &MockConfirmer{ConfirmResult: true}

	allowed, err := ConfirmHooks(h, false, false, confirmer, "", io.Discard)

	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 1, confirmer.ConfirmCalls)
}

func TestUT_ConfirmHooks_InteractiveDenied(t *testing.T) {
	h := &types.HooksConfig{
		PreScaffold: []string{"echo pre"},
	}
	confirmer := &MockConfirmer{ConfirmResult: false}

	allowed, err := ConfirmHooks(h, false, false, confirmer, "", io.Discard)

	require.NoError(t, err)
	assert.False(t, allowed)
	assert.Equal(t, 1, confirmer.ConfirmCalls)
}

func TestUT_ConfirmHooks_NilHooks_ReturnsFalse(t *testing.T) {
	confirmer := &MockConfirmer{}

	allowed, err := ConfirmHooks(nil, false, false, confirmer, "", io.Discard)

	require.NoError(t, err)
	assert.False(t, allowed)
	assert.Equal(t, 0, confirmer.ConfirmCalls)
}

func TestUT_ConfirmHooks_EmptyHooks_ReturnsFalse(t *testing.T) {
	h := &types.HooksConfig{}
	confirmer := &MockConfirmer{}

	allowed, err := ConfirmHooks(h, false, false, confirmer, "", io.Discard)

	require.NoError(t, err)
	assert.False(t, allowed)
	assert.Equal(t, 0, confirmer.ConfirmCalls)
}

func TestUT_ConfirmHooks_AcceptHooksOverridesInteractive(t *testing.T) {
	h := &types.HooksConfig{
		PreScaffold: []string{"echo pre"},
	}
	confirmer := &MockConfirmer{}

	// AcceptHooks=true should run even in interactive mode (no prompt)
	allowed, err := ConfirmHooks(h, true, false, confirmer, "", io.Discard)

	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 0, confirmer.ConfirmCalls)
}

func TestUT_ConfirmHooks_PromptError(t *testing.T) {
	h := &types.HooksConfig{
		PreScaffold: []string{"echo pre"},
	}
	confirmer := &MockConfirmer{
		ConfirmErr: assert.AnError,
	}

	_, err := ConfirmHooks(h, false, false, confirmer, "", io.Discard)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to confirm hooks")
}

// --- Unit Tests for describeHookCommand ---

func TestUT_DescribeHookCommand_BareCommand(t *testing.T) {
	// Bare commands like "go mod tidy" should return no annotation
	result := describeHookCommand("go mod tidy", t.TempDir())
	assert.Equal(t, "", result)
}

func TestUT_DescribeHookCommand_PythonScript(t *testing.T) {
	// .py file without shebang should show python interpreter
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "setup.py"), []byte("print('hello')"), 0o755))

	result := describeHookCommand("hooks/setup.py", tmpDir)
	// Should be either "(python3)" or "(python)" depending on system
	assert.Contains(t, result, "python")
	assert.NotContains(t, result, "NOT FOUND")
}

func TestUT_DescribeHookCommand_ShellScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	// .sh file without shebang should show "sh" interpreter
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "setup.sh"), []byte("echo hello"), 0o755))

	result := describeHookCommand("hooks/setup.sh", tmpDir)
	assert.Equal(t, "(sh)", result)
}

func TestUT_DescribeHookCommand_ScriptWithShebang(t *testing.T) {
	// File with shebang should show "(has shebang)"
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "run.sh"), []byte("#!/bin/bash\necho hello"), 0o755))

	result := describeHookCommand("hooks/run.sh", tmpDir)
	assert.Equal(t, "(has shebang)", result)
}

func TestUT_DescribeHookCommand_ShebangNotExecutable_FallsThrough(t *testing.T) {
	// Script has shebang but no exec bit — should fall through to extension-based annotation
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "setup.py"), []byte("#!/usr/bin/env python3\nprint('hi')"), 0o644))

	result := describeHookCommand("hooks/setup.py", tmpDir)
	// Should show python interpreter, NOT "(has shebang)", since file isn't executable
	assert.Contains(t, result, "python")
	assert.NotContains(t, result, "shebang")
	assert.NotContains(t, result, "NOT FOUND")
}

func TestUT_DescribeHookCommand_UnknownExtension(t *testing.T) {
	// .xyz file should return no annotation
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "script.xyz"), []byte("content"), 0o755))

	result := describeHookCommand("hooks/script.xyz", tmpDir)
	assert.Equal(t, "", result)
}

func TestUT_DescribeHookCommand_NonexistentFile(t *testing.T) {
	// Missing file should return no annotation (fail gracefully)
	result := describeHookCommand("hooks/missing.py", t.TempDir())
	assert.Equal(t, "", result)
}

func TestUT_ResolveHookCmd(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string
		baseDir  string
		expected string
	}{
		{"file reference made absolute", "hooks/post_gen.py", "/tmpl", "/tmpl/hooks/post_gen.py"},
		{"already absolute unchanged", "/usr/bin/python3 hooks/setup.py", "/tmpl", "/usr/bin/python3 hooks/setup.py"},
		{"bare command unchanged", "echo hello", "/tmpl", "echo hello"},
		{"command with args", "hooks/run.sh --verbose", "/tmpl", "/tmpl/hooks/run.sh --verbose"},
		{"quoted path with spaces", `"hooks/my script.py" --flag`, "/tmpl", "'/tmpl/hooks/my script.py' --flag"},
		{"base dir with spaces", "hooks/run.sh", "/my templates", "'/my templates/hooks/run.sh'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveHookCmd(tt.cmd, tt.baseDir)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_ShellJoin(t *testing.T) {
	tests := []struct {
		name     string
		argv     []string
		expected string
	}{
		{"simple", []string{"echo", "hello"}, "echo hello"},
		{"single arg", []string{"/bin/sh"}, "/bin/sh"},
		{"arg with space", []string{"/path/my file.py", "--flag"}, "'/path/my file.py' --flag"},
		{"arg with single quote", []string{"it's", "here"}, `'it'\''s' here`},
		{"empty arg", []string{"cmd", "", "arg"}, "cmd '' arg"},
		{"arg with backslash", []string{`path\to\file`}, `'path\to\file'`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shellJoin(tt.argv)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_ShellJoin_RoundTrip(t *testing.T) {
	// Verify that shellJoin output can be re-parsed by shlex.Split correctly
	testCases := [][]string{
		{"/path/to/script.py", "--verbose"},
		{"/path/with spaces/script.py", "--flag", "value"},
		{"cmd", "arg with spaces", "normal"},
		{"/simple/path"},
	}

	for _, argv := range testCases {
		joined := shellJoin(argv)
		reparsed, err := shlex.Split(joined)
		require.NoError(t, err, "shellJoin output should be parseable: %q", joined)
		assert.Equal(t, argv, reparsed, "round-trip failed for %v -> %q", argv, joined)
	}
}

func TestUT_ResolveHookPaths(t *testing.T) {
	h := &types.HooksConfig{
		PreScaffold:  []string{"hooks/pre.sh"},
		PostScaffold: []string{"hooks/post.py", "echo done"},
	}

	resolved := ResolveHookPaths(h, "/templates/mytemplate")

	assert.Equal(t, "/templates/mytemplate/hooks/pre.sh", resolved.PreScaffold[0])
	assert.Equal(t, "/templates/mytemplate/hooks/post.py", resolved.PostScaffold[0])
	assert.Equal(t, "echo done", resolved.PostScaffold[1]) // bare command unchanged
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

// --- Unit Tests for BuildVarEnv ---

func TestUT_BuildVarEnv_BasicVariables(t *testing.T) {
	vars := map[string]any{
		"project_name": "my_project",
		"author":       "Jane",
	}

	env := BuildVarEnv(vars, io.Discard)

	assertEnvContains(t, env, "TAG_PROJECT_NAME=my_project")
	assertEnvContains(t, env, "TAG_VAR_PROJECT_NAME=my_project")
	assertEnvContains(t, env, "TAG_VAR_AUTHOR=Jane")

	// Should NOT contain TAG_TEMPLATE_DIR or TAG_OUTPUT_DIR
	for _, e := range env {
		assert.False(t, strings.HasPrefix(e, "TAG_TEMPLATE_DIR="),
			"BuildVarEnv should not set TAG_TEMPLATE_DIR")
		assert.False(t, strings.HasPrefix(e, "TAG_OUTPUT_DIR="),
			"BuildVarEnv should not set TAG_OUTPUT_DIR")
	}
}

func TestUT_BuildVarEnv_EmptyVars(t *testing.T) {
	env := BuildVarEnv(map[string]any{}, io.Discard)

	// Should still include system environment
	assert.NotEmpty(t, env)

	// No TAG_VAR_ entries
	for _, e := range env {
		assert.False(t, strings.HasPrefix(e, "TAG_VAR_"),
			"empty vars should produce no TAG_VAR_ entries")
	}
}

func TestUT_BuildVarEnv_NilVars(t *testing.T) {
	env := BuildVarEnv(nil, io.Discard)
	assert.NotEmpty(t, env, "nil vars should still include system env")
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

// --- Output format snapshot tests ---

func TestUT_PrintHookSection_EmptyCommands(t *testing.T) {
	var buf strings.Builder
	hasNotFound := printHookSection("Pre-scaffold", nil, "/tmp", &buf)
	assert.False(t, hasNotFound)
	assert.Empty(t, buf.String(), "empty section should produce no output")
}

func TestUT_PrintHookSection_SingleBareCommand(t *testing.T) {
	var buf strings.Builder
	hasNotFound := printHookSection("Pre-scaffold", []string{"make build"}, "/tmp", &buf)
	assert.False(t, hasNotFound)
	assert.Equal(t, "  Pre-scaffold:\n    - make build\n", buf.String())
}

func TestUT_PrintHookSection_MultipleCommands(t *testing.T) {
	var buf strings.Builder
	hasNotFound := printHookSection("Post-scaffold", []string{"make test", "make lint"}, "/tmp", &buf)
	assert.False(t, hasNotFound)
	assert.Equal(t, "  Post-scaffold:\n    - make test\n    - make lint\n", buf.String())
}

func TestUT_DisplayHookList_PreAndPost(t *testing.T) {
	hooks := &types.HooksConfig{
		PreScaffold:  []string{"make deps"},
		PostScaffold: []string{"make test"},
	}
	var buf strings.Builder
	displayHookList(hooks, "/tmp", &buf)
	out := buf.String()
	assert.Contains(t, out, "This template defines the following hooks:")
	assert.Contains(t, out, "  Pre-scaffold:")
	assert.Contains(t, out, "    - make deps")
	assert.Contains(t, out, "  Post-scaffold:")
	assert.Contains(t, out, "    - make test")
	assert.NotContains(t, out, "WARNING")
}

func TestUT_DisplayHookList_PreOnly(t *testing.T) {
	hooks := &types.HooksConfig{
		PreScaffold: []string{"make deps"},
	}
	var buf strings.Builder
	displayHookList(hooks, "/tmp", &buf)
	out := buf.String()
	assert.Contains(t, out, "  Pre-scaffold:")
	assert.NotContains(t, out, "Post-scaffold:")
}

func TestUT_DisplayHookList_PostOnly(t *testing.T) {
	hooks := &types.HooksConfig{
		PostScaffold: []string{"make test"},
	}
	var buf strings.Builder
	displayHookList(hooks, "/tmp", &buf)
	out := buf.String()
	assert.NotContains(t, out, "Pre-scaffold:")
	assert.Contains(t, out, "  Post-scaffold:")
}

func TestUT_DisplayHookList_Empty(t *testing.T) {
	hooks := &types.HooksConfig{}
	var buf strings.Builder
	displayHookList(hooks, "/tmp", &buf)
	out := buf.String()
	assert.Contains(t, out, "This template defines the following hooks:")
	assert.NotContains(t, out, "Pre-scaffold:")
	assert.NotContains(t, out, "Post-scaffold:")
	assert.NotContains(t, out, "WARNING")
}

func TestUT_PrintHookResults_NoOutput(t *testing.T) {
	var buf strings.Builder
	PrintHookResults([]HookResult{{Command: "make build", Output: "", ExitCode: 0}}, &buf)
	assert.Empty(t, buf.String())
}

func TestUT_PrintHookResults_WithOutput(t *testing.T) {
	var buf strings.Builder
	PrintHookResults([]HookResult{{Command: "make build", Output: "ok\n", ExitCode: 0}}, &buf)
	assert.Equal(t, "ok\n", buf.String())
}

func TestUT_PrintHookResults_EnsuresTrailingNewline(t *testing.T) {
	var buf strings.Builder
	PrintHookResults([]HookResult{{Command: "echo hi", Output: "hi", ExitCode: 0}}, &buf)
	assert.Equal(t, "hi\n", buf.String())
}

func TestUT_PrintHookResults_EmptyResults(t *testing.T) {
	var buf strings.Builder
	PrintHookResults(nil, &buf)
	assert.Empty(t, buf.String())
}

func TestUT_PrintHookResults_MultipleResults(t *testing.T) {
	var buf strings.Builder
	PrintHookResults([]HookResult{
		{Command: "step1", Output: "step1 done\n"},
		{Command: "step2", Output: ""},
		{Command: "step3", Output: "step3 done"},
	}, &buf)
	assert.Equal(t, "step1 done\nstep3 done\n", buf.String())
}
