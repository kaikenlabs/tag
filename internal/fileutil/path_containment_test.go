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
