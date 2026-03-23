package writer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_FileWrite_WriteFile_CreatesDirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fw := &fileWrite{}

	target := filepath.Join(dir, "sub", "deep", "file.txt")
	err := fw.WriteFile(target, []byte("hello"), 0o644)
	require.NoError(t, err)

	content, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(content))
}

func TestUT_FileWrite_ReadFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "read-test.txt")
	require.NoError(t, os.WriteFile(target, []byte("read me"), 0o644))

	fw := &fileWrite{}
	data, err := fw.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "read me", string(data))
}

func TestUT_FileWrite_ReadFile_NotFound(t *testing.T) {
	t.Parallel()
	fw := &fileWrite{}
	_, err := fw.ReadFile(filepath.Join(t.TempDir(), "nonexistent.txt"))
	assert.Error(t, err)
}

func TestUT_FileWrite_OpenFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "open-test.txt")
	require.NoError(t, os.WriteFile(target, []byte("existing"), 0o644))

	fw := &fileWrite{}
	f, err := fw.OpenFile(target, os.O_RDONLY, 0o644)
	require.NoError(t, err)
	defer f.Close()
	assert.NotNil(t, f)
}

func TestUT_FileWrite_Write(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "write-test.txt")
	f, err := os.Create(target)
	require.NoError(t, err)
	defer f.Close()

	fw := &fileWrite{}
	n, err := fw.Write(f, []byte("written"))
	require.NoError(t, err)
	assert.Equal(t, 7, n)
}

func TestUT_FileWrite_ImplementsInterface(t *testing.T) {
	t.Parallel()
	var _ fileReadWrite = (*fileWrite)(nil)
}

func TestUT_Write_StructWithMock(t *testing.T) {
	t.Parallel()
	// Verify Write struct can be constructed with a mock
	mock := &fileReadWriteMock{}
	cwd := t.TempDir()
	w := Write{fs: mock, cwd: cwd}
	assert.NotNil(t, w.fs)
	assert.Equal(t, cwd, w.cwd)
}
