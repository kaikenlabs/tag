package remote

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_FSCache_Get_NotADirectory(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	cache, err := NewFSCache(base)
	require.NoError(t, err)

	// Create a file where a cache directory is expected
	require.NoError(t, os.WriteFile(filepath.Join(base, "file-key"), []byte("not a dir"), 0o644))

	_, found, err := cache.Get("file-key")
	assert.Error(t, err, "should error when cache entry is not a directory")
	assert.False(t, found)

	var cacheErr *CacheError
	assert.ErrorAs(t, err, &cacheErr)
	assert.Equal(t, "get", cacheErr.Op)
}

func TestUT_FSCache_Get_CorruptMetadata(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	cache, err := NewFSCache(base)
	require.NoError(t, err)

	// Create a cache directory with corrupt metadata
	cacheDir := filepath.Join(base, "corrupt-key")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "_meta.json"), []byte("{invalid"), 0o644))

	// Should treat as cache miss (not an error)
	_, found, err := cache.Get("corrupt-key")
	require.NoError(t, err)
	assert.False(t, found, "corrupt metadata should be treated as cache miss")
}

func TestUT_FSCache_Get_MissingMetadata(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	cache, err := NewFSCache(base)
	require.NoError(t, err)

	// Create a cache directory without metadata
	cacheDir := filepath.Join(base, "no-meta-key")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))

	// Should treat as cache miss
	_, found, err := cache.Get("no-meta-key")
	require.NoError(t, err)
	assert.False(t, found, "missing metadata should be treated as cache miss")
}

func TestUT_FSCache_Invalidate_Nonexistent(t *testing.T) {
	t.Parallel()
	cache, err := NewFSCache(t.TempDir())
	require.NoError(t, err)

	// Invalidating a nonexistent key should not error
	err = cache.Invalidate("nonexistent-key")
	assert.NoError(t, err)
}

func TestUT_FSCache_Cleanup_EmptyCache(t *testing.T) {
	t.Parallel()
	cache, err := NewFSCache(t.TempDir())
	require.NoError(t, err)

	removed, err := cache.Cleanup()
	require.NoError(t, err)
	assert.Equal(t, 0, removed)
}

func TestUT_FSCache_Cleanup_SkipsNonDirEntries(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	cache, err := NewFSCache(base)
	require.NoError(t, err)

	// Create a regular file in cache dir (should be skipped)
	require.NoError(t, os.WriteFile(filepath.Join(base, "orphan.txt"), []byte("x"), 0o644))

	removed, err := cache.Cleanup()
	require.NoError(t, err)
	assert.Equal(t, 0, removed)
}

func TestUT_FSCache_Cleanup_SkipsEntryWithUnreadableMeta(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	cache, err := NewFSCache(base)
	require.NoError(t, err)

	// Directory without _meta.json
	require.NoError(t, os.MkdirAll(filepath.Join(base, "no-meta"), 0o755))

	removed, err := cache.Cleanup()
	require.NoError(t, err)
	assert.Equal(t, 0, removed, "should skip entries without readable metadata")
}

func TestUT_FSCache_Cleanup_NonexistentBaseDir(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	cache, err := NewFSCache(base)
	require.NoError(t, err)

	// Remove the base dir to simulate non-existent
	require.NoError(t, os.RemoveAll(base))

	removed, err := cache.Cleanup()
	require.NoError(t, err)
	assert.Equal(t, 0, removed)
}

func TestUT_FSCache_ClearAll_NonexistentBaseDir(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	cache, err := NewFSCache(base)
	require.NoError(t, err)

	require.NoError(t, os.RemoveAll(base))

	removed, err := cache.ClearAll()
	require.NoError(t, err)
	assert.Equal(t, 0, removed)
}

func TestUT_FSCache_List_NonexistentBaseDir(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	cache, err := NewFSCache(base)
	require.NoError(t, err)

	require.NoError(t, os.RemoveAll(base))

	entries, err := cache.List()
	require.NoError(t, err)
	assert.NotNil(t, entries, "List must return an empty slice, not nil, so JSON serialises as []")
	assert.Empty(t, entries)
}

func TestUT_FSCache_SetTTL(t *testing.T) {
	t.Parallel()
	cache, err := NewFSCache(t.TempDir())
	require.NoError(t, err)

	cache.SetTTL(2 * time.Hour)

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("data"), 0o644))

	// Non-pinned entry should get the new TTL
	_, err = cache.Set("ttl-key", srcDir, &CacheMeta{
		FetchedAt: time.Now(),
	})
	require.NoError(t, err)

	meta, err := cache.readMeta("ttl-key")
	require.NoError(t, err)
	require.NotNil(t, meta.ExpiresAt)

	expectedExpiry := time.Now().Add(2 * time.Hour)
	assert.WithinDuration(t, expectedExpiry, *meta.ExpiresAt, 5*time.Second)
}

func TestUT_FSCache_MetaPath(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	cache, err := NewFSCache(base)
	require.NoError(t, err)

	expected := filepath.Join(base, "test-key", "_meta.json")
	assert.Equal(t, expected, cache.metaPath("test-key"))
}
