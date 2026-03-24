package templateupdate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_ShouldSkipDir(t *testing.T) {
	t.Parallel()
	assert.True(t, shouldSkipDir(".git"))
	assert.True(t, shouldSkipDir(".tag"))
	assert.False(t, shouldSkipDir("src"))
	assert.False(t, shouldSkipDir("internal"))
}

func TestUT_IsProjectMetaFile(t *testing.T) {
	t.Parallel()
	assert.True(t, isProjectMetaFile(".tagconfig.json"))
	assert.True(t, isProjectMetaFile(".tagignore"))
	assert.False(t, isProjectMetaFile("src/.tagconfig.json"))
	assert.False(t, isProjectMetaFile("README.md"))
}

func TestUT_ReadProjectFiles_SkipsSymlinks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "real.txt"), []byte("data"), 0o644))
	require.NoError(t, os.Symlink(filepath.Join(dir, "real.txt"), filepath.Join(dir, "link.txt")))

	files, err := ReadProjectFiles(dir)
	require.NoError(t, err)
	assert.Contains(t, files, "real.txt")
	assert.NotContains(t, files, "link.txt")
}

func TestUT_ReadProjectFiles_SkipsTagDirAndGit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git", "objects"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tag"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tag", "gen.go"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tagconfig.json"), []byte("{}"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tagignore"), []byte("*.tmp"), 0o644))

	files, err := ReadProjectFiles(dir)
	require.NoError(t, err)
	assert.Contains(t, files, "main.go")
	assert.NotContains(t, files, ".git/HEAD")
	assert.NotContains(t, files, ".tag/gen.go")
	assert.NotContains(t, files, ".tagconfig.json")
	assert.NotContains(t, files, ".tagignore")
}

func TestUT_ToPointerMap_MultipleEntries(t *testing.T) {
	t.Parallel()
	m := map[string]RenderedFile{
		"a.go": {Content: []byte("a"), Mode: 0o644},
		"b.go": {Content: []byte("b"), Mode: 0o755},
	}
	pm := ToPointerMap(m)
	assert.Len(t, pm, 2)
	assert.Equal(t, []byte("a"), pm["a.go"].Content)
	assert.Equal(t, []byte("b"), pm["b.go"].Content)
}
