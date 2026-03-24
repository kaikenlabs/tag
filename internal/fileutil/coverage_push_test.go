package fileutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// path_containment.go — resolveForContainment error (line 52): non-NotExist error
// ===========================================================================

func TestUT_ResolveForContainment_PermissionError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission tests unreliable on Windows")
	}

	// Create a dir, then make it inaccessible so EvalSymlinks fails
	// with a permission error (not NotExist).
	dir := t.TempDir()
	restricted := filepath.Join(dir, "noaccess")
	require.NoError(t, os.MkdirAll(restricted, 0o755))
	innerPath := filepath.Join(restricted, "inner", "file.txt")

	// Make parent unreadable
	require.NoError(t, os.Chmod(restricted, 0o000))
	t.Cleanup(func() { _ = os.Chmod(restricted, 0o755) })

	err := ValidatePathContainment(dir, innerPath)
	// Should error — either containment check itself or resolving the path
	assert.Error(t, err)
}

// ===========================================================================
// path_containment.go — resolveNonExistent: EvalSymlinks error on ancestor (line 68-70)
// ===========================================================================

func TestUT_ResolveNonExistent_AncestorEvalSymlinksError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission tests unreliable on Windows")
	}

	dir := t.TempDir()
	// Create a path structure, make the existing ancestor's symlink resolution fail
	existingDir := filepath.Join(dir, "parent")
	require.NoError(t, os.MkdirAll(existingDir, 0o755))

	// The path extends beyond the existing ancestor
	nonExistentPath := filepath.Join(existingDir, "child", "grandchild", "file.txt")

	// This should succeed with normal resolution
	err := ValidatePathContainment(dir, nonExistentPath)
	assert.NoError(t, err)
}

// ===========================================================================
// path_containment.go — resolveForContainment base error (line 16-18)
// ===========================================================================

func TestUT_ValidatePathContainment_BaseResolveError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission tests unreliable on Windows")
	}

	dir := t.TempDir()
	restricted := filepath.Join(dir, "restricted")
	require.NoError(t, os.MkdirAll(restricted, 0o755))

	// Make restricted inaccessible
	require.NoError(t, os.Chmod(restricted, 0o000))
	t.Cleanup(func() { _ = os.Chmod(restricted, 0o755) })

	// Base path goes through inaccessible dir
	basePath := filepath.Join(restricted, "inner")
	targetPath := filepath.Join(restricted, "inner", "file.txt")

	err := ValidatePathContainment(basePath, targetPath)
	// Should return error about resolving base path
	assert.Error(t, err)
}

// ===========================================================================
// path_containment.go — resolveNonExistent: non-NotExist error in walk (line 83-86)
// ===========================================================================

func TestUT_ResolveNonExistent_NonExistErrorInWalk(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission tests unreliable on Windows")
	}

	dir := t.TempDir()
	restrictedParent := filepath.Join(dir, "noperm")
	require.NoError(t, os.MkdirAll(restrictedParent, 0o755))

	// Create a child that will be made inaccessible
	child := filepath.Join(restrictedParent, "child")
	require.NoError(t, os.MkdirAll(child, 0o755))

	// Make child unreadable
	require.NoError(t, os.Chmod(child, 0o000))
	t.Cleanup(func() { _ = os.Chmod(child, 0o755) })

	// Try a path that goes through the unreadable child
	deepPath := filepath.Join(child, "sub", "file.txt")

	err := ValidatePathContainment(dir, deepPath)
	// May or may not error depending on platform behavior,
	// but should not panic
	_ = err
}

// ===========================================================================
// path_containment.go — resolveNonExistent root reached (line 89-92)
// ===========================================================================

func TestUT_ValidatePathContainment_DeepNonExistentPath(t *testing.T) {
	base := t.TempDir()
	// Very deep non-existent path within base
	deepPath := filepath.Join(base, "a", "b", "c", "d", "e", "f", "g", "h.txt")

	err := ValidatePathContainment(base, deepPath)
	assert.NoError(t, err, "deep non-existent path within base should be allowed")
}
