package testrunner_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/testrunner"
)

func TestUT_RunValidationCommands_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	result := testrunner.RunValidationCommands(
		context.Background(),
		dir,
		[]string{"echo hello", "true"},
		nil,
		30*time.Second,
	)
	assert.Nil(t, result, "all commands pass → nil result")
}

func TestUT_RunValidationCommands_Failure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	result := testrunner.RunValidationCommands(
		context.Background(),
		dir,
		[]string{"echo ok", "false", "echo unreachable"},
		nil,
		30*time.Second,
	)
	require.NotNil(t, result)
	assert.Equal(t, "false", result.Command)
	assert.NotZero(t, result.ExitCode)
}

func TestUT_RunValidationCommands_WorkingDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	marker := filepath.Join(dir, "marker.txt")
	require.NoError(t, os.WriteFile(marker, []byte("found"), 0o644))

	result := testrunner.RunValidationCommands(
		context.Background(),
		dir,
		[]string{"cat marker.txt"},
		nil,
		30*time.Second,
	)
	assert.Nil(t, result)
}

func TestUT_RunValidationCommands_Env(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	result := testrunner.RunValidationCommands(
		context.Background(),
		dir,
		[]string{"test \"$MY_VAR\" = hello"},
		map[string]string{"MY_VAR": "hello"},
		30*time.Second,
	)
	assert.Nil(t, result)
}

func TestUT_RunValidationCommands_Timeout(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	result := testrunner.RunValidationCommands(
		context.Background(),
		dir,
		[]string{"sleep 60"},
		nil,
		100*time.Millisecond,
	)
	require.NotNil(t, result)
	assert.Equal(t, -1, result.ExitCode)
}

func TestUT_TruncateOutput(t *testing.T) {
	t.Parallel()

	short := "hello\nworld"
	assert.Equal(t, short, testrunner.TruncateOutput(short, 100))

	long := "line1\nline2\nline3\nline4\nline5"
	truncated := testrunner.TruncateOutput(long, 15)
	assert.Contains(t, truncated, "truncated")
	assert.Contains(t, truncated, "line5")
}
