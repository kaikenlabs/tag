package fileutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_ValidatePathContainment_ValidPaths(t *testing.T) {
	base := t.TempDir()

	tests := []struct {
		name string
		path string
	}{
		{"simple file", filepath.Join(base, "file.txt")},
		{"nested file", filepath.Join(base, "sub", "dir", "file.txt")},
		{"dot in name", filepath.Join(base, ".hidden", "file.txt")},
		{"cleaned relative", filepath.Join(base, "sub", "..", "sub", "file.txt")},
		{"path equals base", base},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePathContainment(base, tt.path)
			assert.NoError(t, err)
		})
	}
}

func TestUT_ValidatePathContainment_PathTraversal(t *testing.T) {
	base := t.TempDir()

	tests := []struct {
		name string
		path string
	}{
		{"parent traversal", filepath.Join(base, "..", "escape.txt")},
		{"double parent traversal", filepath.Join(base, "..", "..", "escape.txt")},
		{"absolute path outside", "/tmp/evil.txt"},
		{"prefix collision", base + "X/file.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePathContainment(base, tt.path)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "escapes base directory")
		})
	}
}

func TestUT_ValidatePathContainment_NonExistentPath(t *testing.T) {
	base := t.TempDir()

	t.Run("non-existent file within base", func(t *testing.T) {
		err := ValidatePathContainment(base, filepath.Join(base, "does", "not", "exist.txt"))
		assert.NoError(t, err)
	})

	t.Run("non-existent file outside base", func(t *testing.T) {
		err := ValidatePathContainment(base, filepath.Join(base, "..", "outside.txt"))
		assert.Error(t, err)
	})
}

func TestUT_ValidatePathContainment_EmptyPath(t *testing.T) {
	base := t.TempDir()
	// Empty path resolves to cwd via filepath.Abs, which is outside base
	err := ValidatePathContainment(base, "")
	assert.Error(t, err)
}

func TestUT_ValidatePathContainment_SymlinkResolution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	base := t.TempDir()
	external := t.TempDir()

	// Create a real file within base
	realFile := filepath.Join(base, "real.txt")
	require.NoError(t, os.WriteFile(realFile, []byte("real"), 0o644))

	// Create a symlink within base pointing to external directory
	symlinkPath := filepath.Join(base, "escape_link")
	require.NoError(t, os.Symlink(external, symlinkPath))

	t.Run("symlink escaping base is rejected", func(t *testing.T) {
		err := ValidatePathContainment(base, filepath.Join(symlinkPath, "file.txt"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "escapes base directory")
	})

	// Create a symlink within base pointing to another location within base
	subDir := filepath.Join(base, "subdir")
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	internalLink := filepath.Join(base, "internal_link")
	require.NoError(t, os.Symlink(subDir, internalLink))

	t.Run("symlink within base is allowed", func(t *testing.T) {
		err := ValidatePathContainment(base, filepath.Join(internalLink, "file.txt"))
		assert.NoError(t, err)
	})
}

func TestUT_ValidatePathContainment_BaseWithDotComponents(t *testing.T) {
	base := t.TempDir()
	subDir := filepath.Join(base, "a", "b")
	require.NoError(t, os.MkdirAll(subDir, 0o755))

	// Base with .. components that resolve to same directory
	baseWithDots := filepath.Join(base, "a", "..", "a", "b")
	target := filepath.Join(subDir, "file.txt")

	err := ValidatePathContainment(baseWithDots, target)
	assert.NoError(t, err)
}

func TestUT_ValidatePathContainment_TrailingSeparator(t *testing.T) {
	base := t.TempDir()

	// Base with trailing separator should work the same
	err := ValidatePathContainment(base+string(filepath.Separator), filepath.Join(base, "file.txt"))
	assert.NoError(t, err)
}

func TestUT_ValidatePathContainment_RelativeTargetUnderSymlinkedCwd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("os.Getwd does not consult PWD on Windows, so a symlinked cwd cannot be reproduced")
	}

	realDir := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(realDir, link))

	t.Chdir(link)

	cwd, err := os.Getwd()
	require.NoError(t, err)
	resolved, err := filepath.EvalSymlinks(cwd)
	require.NoError(t, err)
	require.NotEqual(t, resolved, cwd, "fixture did not reproduce a symlinked cwd")

	tests := []struct {
		name string
		path string
	}{
		{"relative file", "blood"},
		{"relative nested file", filepath.Join("sub", "dir", "file.txt")},
		{"relative dot prefix", filepath.Join(".", "blood")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, ValidatePathContainment(cwd, tt.path))
		})
	}
}

func mustEvalSymlinks(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	require.NoError(t, err)
	return resolved
}

// TestUT_ValidatePathContainment_DanglingSymlinkFailsClosed pins the #418
// fix: a path that resolveNonExistent cannot fully resolve now fails CLOSED
// (an error) instead of silently returning the unresolved path with a nil
// error. The NotContains assertion on "escapes base directory" is load-
// bearing: it proves the rejection comes from the new fail-closed resolve
// error, not from a coincidental containment-check mismatch (see the
// mis-fixture note on TestUT_ValidatePathContainment_RelativeTargetEscapes).
func TestUT_ValidatePathContainment_DanglingSymlinkFailsClosed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	tests := []struct {
		name  string
		setup func(t *testing.T, base, outside string) (target string)
	}{
		{
			name: "nearest existing ancestor is a dangling symlink",
			setup: func(t *testing.T, base, outside string) string {
				t.Helper()
				require.NoError(t, os.Symlink(filepath.Join(outside, "nodir"), filepath.Join(base, "linkdir")))
				return filepath.Join(base, "linkdir", "child")
			},
		},
		{
			name: "target itself is a dangling symlink pointing outside base",
			setup: func(t *testing.T, base, outside string) string {
				t.Helper()
				link := filepath.Join(base, "evil")
				require.NoError(t, os.Symlink(filepath.Join(outside, "pwned"), link))
				return link
			},
		},
		{
			name: "target itself is a dangling symlink pointing inside base",
			setup: func(t *testing.T, base, _ string) string {
				t.Helper()
				link := filepath.Join(base, "selfdangle")
				require.NoError(t, os.Symlink(filepath.Join(base, "never-created"), link))
				return link
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := mustEvalSymlinks(t, t.TempDir())
			require.Equal(t, base, mustEvalSymlinks(t, base), "fixture base must resolve to itself or the prefix comparison fails for the wrong reason")
			outside := mustEvalSymlinks(t, t.TempDir())

			target := tt.setup(t, base, outside)

			err := ValidatePathContainment(base, target)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "escapes base directory")
		})
	}
}

// TestUT_ValidatePathContainment_FailClosedStillAllowsOrdinaryTargets is the
// positive-control counterpart to the fail-closed test above: an ordinary
// non-existent path (no symlinks involved) must still resolve, and a LIVE
// (non-dangling) escaping symlink must still be rejected with the original
// "escapes base directory" wording — proving the escape verdict was not
// silently relabelled as a generic resolve failure.
func TestUT_ValidatePathContainment_FailClosedStillAllowsOrdinaryTargets(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	base := mustEvalSymlinks(t, t.TempDir())
	require.Equal(t, base, mustEvalSymlinks(t, base), "fixture base must resolve to itself or the prefix comparison fails for the wrong reason")

	t.Run("ordinary non-existent nested path", func(t *testing.T) {
		err := ValidatePathContainment(base, filepath.Join(base, "sub", "deep", "file.txt"))
		assert.NoError(t, err)
	})

	t.Run("ordinary deep non-existent path", func(t *testing.T) {
		err := ValidatePathContainment(base, filepath.Join(base, "a", "b", "c", "d", "e", "f", "g.txt"))
		assert.NoError(t, err)
	})

	t.Run("live escaping symlink is still rejected with escape wording", func(t *testing.T) {
		outside := mustEvalSymlinks(t, t.TempDir())
		require.NoError(t, os.WriteFile(filepath.Join(outside, "target.txt"), []byte("x"), 0o644))

		link := filepath.Join(base, "live_escape")
		require.NoError(t, os.Symlink(outside, link))

		err := ValidatePathContainment(base, filepath.Join(link, "target.txt"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "escapes base directory")
	})
}

func TestUT_ValidatePathContainment_RelativeTargetEscapes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("os.Getwd does not consult PWD on Windows, so a symlinked cwd cannot be reproduced")
	}

	root := t.TempDir()
	outside := filepath.Join(root, "outside")
	realDir := filepath.Join(root, "realDir")
	require.NoError(t, os.MkdirAll(outside, 0o755))
	require.NoError(t, os.MkdirAll(realDir, 0o755))

	link := filepath.Join(root, "link")
	require.NoError(t, os.Symlink(realDir, link))
	require.NoError(t, os.Symlink(outside, filepath.Join(realDir, "abs_escape")))
	require.NoError(t, os.Symlink(filepath.Join("..", "outside"), filepath.Join(realDir, "rel_escape")))

	t.Chdir(link)

	cwd, err := os.Getwd()
	require.NoError(t, err)
	resolved, err := filepath.EvalSymlinks(cwd)
	require.NoError(t, err)
	require.NotEqual(t, resolved, cwd, "fixture did not reproduce a symlinked cwd")

	tests := []struct {
		name string
		path string
	}{
		{"relative parent traversal", filepath.Join("..", "outside", "file.txt")},
		{"ancestor is symlink with absolute body", filepath.Join("abs_escape", "file.txt")},
		{"ancestor is symlink with relative body", filepath.Join("rel_escape", "file.txt")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePathContainment(cwd, tt.path)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "escapes base directory")
		})
	}
}
