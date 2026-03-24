package fileutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_CopyDir_BasicCopy(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := filepath.Join(t.TempDir(), "output")

	// Create source structure
	require.NoError(t, os.MkdirAll(filepath.Join(srcDir, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "root.txt"), []byte("root"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "sub", "nested.txt"), []byte("nested"), 0o644))

	err := CopyDir(srcDir, dstDir, 0o755)
	require.NoError(t, err)

	// Verify files were copied
	content, err := os.ReadFile(filepath.Join(dstDir, "root.txt"))
	require.NoError(t, err)
	assert.Equal(t, "root", string(content))

	content, err = os.ReadFile(filepath.Join(dstDir, "sub", "nested.txt"))
	require.NoError(t, err)
	assert.Equal(t, "nested", string(content))
}

func TestUT_CopyDir_SkipsSymlinks(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := filepath.Join(t.TempDir(), "output")

	// Create a regular file and a symlink
	realFile := filepath.Join(srcDir, "real.txt")
	require.NoError(t, os.WriteFile(realFile, []byte("real"), 0o644))
	require.NoError(t, os.Symlink(realFile, filepath.Join(srcDir, "link.txt")))

	err := CopyDir(srcDir, dstDir, 0o755)
	require.NoError(t, err)

	// Real file should be copied
	assert.FileExists(t, filepath.Join(dstDir, "real.txt"))

	// Symlink should be skipped
	_, statErr := os.Lstat(filepath.Join(dstDir, "link.txt"))
	assert.True(t, os.IsNotExist(statErr), "symlink should not be copied")
}

func TestUT_CopyDir_SourceNotExist(t *testing.T) {
	t.Parallel()

	err := CopyDir("/nonexistent/source", t.TempDir(), 0o755)
	assert.Error(t, err)
}

func TestUT_CopyFile_DestDirNotExist(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "src.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("data"), 0o644))

	// Destination directory doesn't exist
	dstFile := filepath.Join(t.TempDir(), "missing-dir", "dst.txt")
	err := CopyFile(srcFile, dstFile)
	assert.Error(t, err)
}

func TestUT_IsTextContent_LargeTextFile(t *testing.T) {
	t.Parallel()

	// Create content larger than TextSampleSize
	content := make([]byte, TextSampleSize+100)
	for i := range content {
		content[i] = 'a'
	}
	assert.True(t, IsTextContent(content))
}

func TestUT_IsTextContent_NullByteInSample(t *testing.T) {
	t.Parallel()

	content := make([]byte, 100)
	for i := range content {
		content[i] = 'a'
	}
	content[50] = 0x00
	assert.False(t, IsTextContent(content))
}
