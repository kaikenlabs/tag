package templateupdate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_ReadProjectFiles_Success(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Hello"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "src"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "src", "main.go"), []byte("package main"), 0o644))

	files, err := ReadProjectFiles(dir)
	require.NoError(t, err)
	assert.Len(t, files, 2)
	assert.Contains(t, files, "README.md")
	assert.Contains(t, files, "src/main.go")
	assert.Equal(t, []byte("# Hello"), files["README.md"].Content)
	assert.False(t, files["README.md"].IsBinary)
}

func TestUT_ReadProjectFiles_SkipsGitDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git", "objects"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref: refs/heads/main"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0o644))

	files, err := ReadProjectFiles(dir)
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Contains(t, files, "file.txt")
}

func TestUT_ReadProjectFiles_SkipsTagConfigFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tagconfig.json"), []byte("{}"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "app.go"), []byte("package app"), 0o644))

	files, err := ReadProjectFiles(dir)
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Contains(t, files, "app.go")
	assert.NotContains(t, files, ".tagconfig.json")
}

func TestUT_ReadProjectFiles_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	files, err := ReadProjectFiles(dir)
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestUT_ReadProjectFiles_BinaryDetection(t *testing.T) {
	dir := t.TempDir()
	binary := []byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0x00, 0x01}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "image.png"), binary, 0o644))

	files, err := ReadProjectFiles(dir)
	require.NoError(t, err)
	assert.True(t, files["image.png"].IsBinary)
}

func TestUT_ReadProjectFiles_NormalizesPaths(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a", "b", "c.txt"), []byte("x"), 0o644))

	files, err := ReadProjectFiles(dir)
	require.NoError(t, err)
	assert.Contains(t, files, "a/b/c.txt")
}

func TestUT_ToPointerMap(t *testing.T) {
	m := map[string]RenderedFile{
		"a.txt": {Content: []byte("a"), Mode: 0o644},
		"b.txt": {Content: []byte("b"), Mode: 0o755},
	}

	pm := ToPointerMap(m)
	assert.Len(t, pm, 2)
	assert.Equal(t, []byte("a"), pm["a.txt"].Content)
	assert.Equal(t, []byte("b"), pm["b.txt"].Content)
}
