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

// TestUT_ResolveNonExistent_NonExistErrorInWalk exercises resolveNonExistent's
// fail-closed stat-error branch (#418): an ancestor that exists but cannot be
// searched (Lstat on a descendant fails with permission-denied, not
// not-exist) must now abort with an error rather than silently falling back
// to the unresolved path. chmod 0o000 is a no-op under root (some CI images
// run as root), so the test probes for real permission enforcement first and
// skips rather than asserting blindly when it isn't observed.
func TestUT_ResolveNonExistent_NonExistErrorInWalk(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission tests unreliable on Windows")
	}

	dir := t.TempDir()
	restrictedParent := filepath.Join(dir, "noperm")
	require.NoError(t, os.MkdirAll(restrictedParent, 0o755))

	child := filepath.Join(restrictedParent, "child")
	require.NoError(t, os.MkdirAll(child, 0o755))

	require.NoError(t, os.Chmod(child, 0o000))
	t.Cleanup(func() { _ = os.Chmod(child, 0o755) })

	if _, err := os.Lstat(filepath.Join(child, "probe")); !os.IsPermission(err) {
		t.Skip("permission enforcement not observed (likely running as root); cannot exercise the stat-error branch")
	}

	deepPath := filepath.Join(child, "sub", "file.txt")

	err := ValidatePathContainment(dir, deepPath)
	require.Error(t, err, "an unreadable ancestor must fail closed rather than silently falling back to the unresolved path")
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
