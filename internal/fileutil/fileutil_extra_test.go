package fileutil

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_CopyDir_RecursivelyCopiesFilesAndSkipsSymlinks(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	dst := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(src, "nested"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "root.txt"), []byte("root"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(src, "nested", "child.txt"), []byte("child"), 0o600))
	require.NoError(t, os.Symlink(filepath.Join(src, "root.txt"), filepath.Join(src, "nested", "link.txt")))

	require.NoError(t, CopyDir(src, dst, 0o755))

	root, err := os.ReadFile(filepath.Join(dst, "root.txt"))
	require.NoError(t, err)
	assert.Equal(t, "root", string(root))

	child, err := os.ReadFile(filepath.Join(dst, "nested", "child.txt"))
	require.NoError(t, err)
	assert.Equal(t, "child", string(child))

	_, err = os.Lstat(filepath.Join(dst, "nested", "link.txt"))
	assert.True(t, os.IsNotExist(err))
}

func TestUT_CopyFile_OverwritesDestinationContent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")

	require.NoError(t, os.WriteFile(src, []byte("new-data"), 0o640))
	require.NoError(t, os.WriteFile(dst, []byte("old-data"), 0o644))

	require.NoError(t, CopyFile(src, dst))

	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "new-data", string(got))
}

func TestUT_IsTextContent_UsesSampleWindowAndPrintableThreshold(t *testing.T) {
	t.Parallel()

	longText := []byte(strings.Repeat("a", TextSampleSize) + "\x00")
	assert.True(t, IsTextContent(longText), "null byte after sample window should not affect result")

	mostlyControl := make([]byte, 20)
	for i := range mostlyControl {
		mostlyControl[i] = byte(i + 1)
	}
	assert.False(t, IsTextContent(mostlyControl), "control chars beyond threshold should be binary")
}

func TestUT_SanitizeFileMode_RemovesDangerousBitsKeepsTypeAndPerms(t *testing.T) {
	t.Parallel()

	mode := fs.ModeDir | fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky | 0o755
	sanitized := SanitizeFileMode(mode)

	assert.Equal(t, fs.ModeDir, sanitized&fs.ModeType)
	assert.Equal(t, fs.FileMode(0o755), sanitized.Perm())
	assert.Zero(t, sanitized&(fs.ModeSetuid|fs.ModeSetgid|fs.ModeSticky))
}
