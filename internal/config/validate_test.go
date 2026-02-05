package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_Validate_NilConfig(t *testing.T) {
	var cfg *Config
	err := cfg.Validate()

	require.Error(t, err)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "config", valErr.Field)
	assert.Contains(t, valErr.Message, "nil")
}

func TestUT_Validate_EmptyConfig(t *testing.T) {
	cfg := &Config{}
	err := cfg.Validate()

	require.NoError(t, err)
}

func TestUT_Validate_ValidPath(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Env: Env{
			Path: tmpDir,
		},
	}
	err := cfg.Validate()

	require.NoError(t, err)
}

func TestUT_Validate_InvalidPath(t *testing.T) {
	cfg := &Config{
		Env: Env{
			Path: "/nonexistent/path/that/does/not/exist",
		},
	}
	err := cfg.Validate()

	require.Error(t, err)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "env.TAG_PATH", valErr.Field)
	assert.Contains(t, valErr.Message, "does not exist")
}

func TestUT_Validate_PathIsFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "file.txt")
	err := os.WriteFile(tmpFile, []byte("content"), 0o644)
	require.NoError(t, err)

	cfg := &Config{
		Env: Env{
			Path: tmpFile,
		},
	}
	err = cfg.Validate()

	require.Error(t, err)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "env.TAG_PATH", valErr.Field)
	assert.Contains(t, valErr.Message, "not a directory")
}

func TestUT_Validate_ValidHookFile(t *testing.T) {
	tmpDir := t.TempDir()
	hookPath := filepath.Join(tmpDir, "hook.sh")
	err := os.WriteFile(hookPath, []byte("#!/bin/bash\nexit 0"), 0o755)
	require.NoError(t, err)

	cfg := &Config{
		Hooks: Hooks{
			Pre: [][]string{{hookPath}},
		},
	}
	err = cfg.Validate()

	require.NoError(t, err)
}

func TestUT_Validate_ValidHookInPath(t *testing.T) {
	// "echo" should be available in PATH on all systems
	cfg := &Config{
		Hooks: Hooks{
			Pre: [][]string{{"echo", "hello"}},
		},
	}
	err := cfg.Validate()

	require.NoError(t, err)
}

func TestUT_Validate_InvalidHookPath(t *testing.T) {
	cfg := &Config{
		Hooks: Hooks{
			Pre: [][]string{{"/nonexistent/hook/script.sh"}},
		},
	}
	err := cfg.Validate()

	require.Error(t, err)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "hooks.pre[0]", valErr.Field)
	assert.Contains(t, valErr.Message, "not found")
}

func TestUT_Validate_InvalidHookCommand(t *testing.T) {
	cfg := &Config{
		Hooks: Hooks{
			Pre: [][]string{{"nonexistent_command_xyz_123"}},
		},
	}
	err := cfg.Validate()

	require.Error(t, err)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "hooks.pre[0]", valErr.Field)
	assert.Contains(t, valErr.Message, "not found in PATH")
}

func TestUT_Validate_EmptyHookEntry(t *testing.T) {
	cfg := &Config{
		Hooks: Hooks{
			Pre: [][]string{{}},
		},
	}
	err := cfg.Validate()

	require.Error(t, err)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "hooks.pre[0]", valErr.Field)
	assert.Contains(t, valErr.Message, "empty")
}

func TestUT_Validate_EmptyHookCommand(t *testing.T) {
	cfg := &Config{
		Hooks: Hooks{
			Pre: [][]string{{""}},
		},
	}
	err := cfg.Validate()

	require.Error(t, err)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "hooks.pre[0]", valErr.Field)
	assert.Contains(t, valErr.Message, "empty")
}

func TestUT_Validate_WhitespaceOnlyHookCommand(t *testing.T) {
	cfg := &Config{
		Hooks: Hooks{
			Pre: [][]string{{"   "}},
		},
	}
	err := cfg.Validate()

	require.Error(t, err)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "hooks.pre[0]", valErr.Field)
	assert.Contains(t, valErr.Message, "whitespace")
}

func TestUT_Validate_LeadingWhitespaceHookCommand(t *testing.T) {
	cfg := &Config{
		Hooks: Hooks{
			Pre: [][]string{{"  echo"}},
		},
	}
	err := cfg.Validate()

	require.Error(t, err)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "hooks.pre[0]", valErr.Field)
	assert.Contains(t, valErr.Message, "whitespace")
}

func TestUT_Validate_TrailingWhitespaceHookCommand(t *testing.T) {
	cfg := &Config{
		Hooks: Hooks{
			Pre: [][]string{{"echo  "}},
		},
	}
	err := cfg.Validate()

	require.Error(t, err)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "hooks.pre[0]", valErr.Field)
	assert.Contains(t, valErr.Message, "whitespace")
}

func TestUT_Validate_HookWithArgs(t *testing.T) {
	tmpDir := t.TempDir()
	hookPath := filepath.Join(tmpDir, "hook.sh")
	err := os.WriteFile(hookPath, []byte("#!/bin/bash\nexit 0"), 0o755)
	require.NoError(t, err)

	cfg := &Config{
		Hooks: Hooks{
			Pre: [][]string{{hookPath, "arg1", "arg2"}},
		},
	}
	err = cfg.Validate()

	require.NoError(t, err)
}

func TestUT_Validate_NilHooksSlice(t *testing.T) {
	cfg := &Config{
		Hooks: Hooks{
			Pre:  nil,
			Post: nil,
		},
	}
	err := cfg.Validate()

	require.NoError(t, err)
}

func TestUT_Validate_PreHookFailsBeforePost(t *testing.T) {
	cfg := &Config{
		Hooks: Hooks{
			Pre:  [][]string{{"nonexistent_pre_command_xyz"}},
			Post: [][]string{{"nonexistent_post_command_xyz"}},
		},
	}
	err := cfg.Validate()

	require.Error(t, err)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	// Should fail on pre hook, not post
	assert.Equal(t, "hooks.pre[0]", valErr.Field)
}

func TestUT_Validate_RelativePathHook(t *testing.T) {
	tmpDir := t.TempDir()
	hookPath := filepath.Join(tmpDir, "hook.sh")
	err := os.WriteFile(hookPath, []byte("#!/bin/bash\nexit 0"), 0o755)
	require.NoError(t, err)

	// Change to tmpDir to test relative path
	oldDir, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(oldDir) })

	cfg := &Config{
		Hooks: Hooks{
			Pre: [][]string{{"./hook.sh"}},
		},
	}
	err = cfg.Validate()

	require.NoError(t, err)
}

func TestUT_Validate_MultipleHooksFirstFails(t *testing.T) {
	tmpDir := t.TempDir()
	validHook := filepath.Join(tmpDir, "valid.sh")
	err := os.WriteFile(validHook, []byte("#!/bin/bash\nexit 0"), 0o755)
	require.NoError(t, err)

	cfg := &Config{
		Hooks: Hooks{
			Pre: [][]string{
				{"/nonexistent/first.sh"},
				{validHook},
			},
		},
	}
	err = cfg.Validate()

	require.Error(t, err)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "hooks.pre[0]", valErr.Field)
}

func TestUT_Validate_MultipleHooksSecondFails(t *testing.T) {
	tmpDir := t.TempDir()
	validHook := filepath.Join(tmpDir, "valid.sh")
	err := os.WriteFile(validHook, []byte("#!/bin/bash\nexit 0"), 0o755)
	require.NoError(t, err)

	cfg := &Config{
		Hooks: Hooks{
			Pre: [][]string{
				{validHook},
				{"/nonexistent/second.sh"},
			},
		},
	}
	err = cfg.Validate()

	require.Error(t, err)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "hooks.pre[1]", valErr.Field)
}

func TestUT_Validate_HookPathIsDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Hooks: Hooks{
			Pre: [][]string{{tmpDir}},
		},
	}
	err := cfg.Validate()

	require.Error(t, err)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "hooks.pre[0]", valErr.Field)
	assert.Contains(t, valErr.Message, "directory")
}

func TestUT_Validate_HookFileNotExecutable(t *testing.T) {
	tmpDir := t.TempDir()
	hookPath := filepath.Join(tmpDir, "hook.sh")
	// Create file without execute permission (0644)
	err := os.WriteFile(hookPath, []byte("#!/bin/bash\nexit 0"), 0o644)
	require.NoError(t, err)

	cfg := &Config{
		Hooks: Hooks{
			Pre: [][]string{{hookPath}},
		},
	}
	err = cfg.Validate()

	require.Error(t, err)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "hooks.pre[0]", valErr.Field)
	assert.Contains(t, valErr.Message, "not executable")
}

func TestUT_ValidationError_Error(t *testing.T) {
	err := &ValidationError{
		Field:   "test.field",
		Message: "test message",
	}

	assert.Equal(t, "test.field: test message", err.Error())
}
