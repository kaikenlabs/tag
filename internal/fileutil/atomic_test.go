package fileutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The modes below are compared against what os.WriteFile/os.MkdirAll produce
// rather than against a literal, because the invariant is "identical to the
// call this replaced" under whatever umask the run happens to have. Asserting
// a literal 0644 would pass for a chmod-based implementation, which is the
// regression these tests exist to catch, and would fail under a non-default
// umask for the correct one.
func TestUT_WriteFileAtomic_ModeMatchesWriteFile(t *testing.T) {
	t.Parallel()

	for _, perm := range []os.FileMode{0o600, 0o644, 0o666} {
		dir := t.TempDir()

		reference := filepath.Join(dir, "reference")
		require.NoError(t, os.WriteFile(reference, []byte("x"), perm))
		want, err := os.Stat(reference)
		require.NoError(t, err)

		target := filepath.Join(dir, "target")
		require.NoError(t, WriteFileAtomic(target, []byte("x"), perm))
		got, err := os.Stat(target)
		require.NoError(t, err)

		assert.Equal(t, want.Mode().Perm(), got.Mode().Perm(),
			"perm %v must match os.WriteFile, which applies the umask", perm)
	}
}

func TestUT_MkdirUnique_ModeMatchesMkdirAll(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	reference := filepath.Join(dir, "reference")
	require.NoError(t, os.MkdirAll(reference, 0o777))
	want, err := os.Stat(reference)
	require.NoError(t, err)

	got, err := MkdirUnique(dir, ".staging-", 0o777)
	require.NoError(t, err)
	info, err := os.Stat(got)
	require.NoError(t, err)

	assert.Equal(t, want.Mode().Perm(), info.Mode().Perm(),
		"must match os.MkdirAll, which applies the umask")
}

func TestUT_WriteFileAtomic_LeavesNoTempBehind(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "data.json")
	require.NoError(t, WriteFileAtomic(target, []byte("{}"), 0o600))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "data.json", entries[0].Name())
}

func TestUT_WriteFileAtomic_FailureLeavesTargetAndDirClean(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "sub", "data.json")

	require.Error(t, WriteFileAtomic(target, []byte("{}"), 0o600))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "a failed write must not leave a temp file behind")
}

func TestUT_MkdirUnique_ReturnsDistinctPaths(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seen := make(map[string]bool)

	for range 20 {
		path, err := MkdirUnique(dir, ".staging-", 0o755)
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(filepath.Base(path), ".staging-"))
		require.False(t, seen[path], "MkdirUnique returned a duplicate path")
		seen[path] = true
	}
}
