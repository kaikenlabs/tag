package fileutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// path_containment.go — resolveForContainment / resolveNonExistent branches
// ===========================================================================

func TestUT_ResolveForContainment_ExistingPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	resolved, err := resolveForContainment(dir)
	require.NoError(t, err)
	assert.NotEmpty(t, resolved)
}

func TestUT_ResolveForContainment_NonExistentPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	nonExistent := filepath.Join(dir, "a", "b", "c")
	resolved, err := resolveForContainment(nonExistent)
	require.NoError(t, err)
	assert.Contains(t, resolved, "a")
	assert.Contains(t, resolved, "b")
	assert.Contains(t, resolved, "c")
}

func TestUT_ResolveNonExistent_MultipleSegments(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	deepPath := filepath.Join(dir, "x", "y", "z")
	resolved, err := resolveNonExistent(filepath.Clean(deepPath))
	require.NoError(t, err)
	assert.Contains(t, resolved, "x")
	assert.Contains(t, resolved, "y")
	assert.Contains(t, resolved, "z")
}

func TestUT_ValidatePathContainment_BothNonExistent(t *testing.T) {
	t.Parallel()
	// Both base and target are non-existent but target is under base
	base := filepath.Join(t.TempDir(), "nonexistent-base")
	target := filepath.Join(base, "sub", "file.txt")

	err := ValidatePathContainment(base, target)
	assert.NoError(t, err)
}

func TestUT_ValidatePathContainment_NestedExistingSymlink(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	sub := filepath.Join(base, "sub")
	require.NoError(t, os.MkdirAll(sub, 0o755))

	// Create a symlink inside sub that points to another dir inside base
	target := filepath.Join(base, "target")
	require.NoError(t, os.MkdirAll(target, 0o755))
	link := filepath.Join(sub, "link")
	require.NoError(t, os.Symlink(target, link))

	err := ValidatePathContainment(base, filepath.Join(link, "file.txt"))
	assert.NoError(t, err, "symlink within base should be allowed")
}
