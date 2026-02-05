package remote

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_FSCache_NewFSCache(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")

	cache, err := NewFSCache(cacheDir)
	require.NoError(t, err)
	assert.NotNil(t, cache)

	// Verify directory was created
	info, err := os.Stat(cacheDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestUT_FSCache_GetMiss(t *testing.T) {
	cache, err := NewFSCache(t.TempDir())
	require.NoError(t, err)

	path, found, err := cache.Get("nonexistent_key")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Empty(t, path)
}

func TestUT_FSCache_SetAndGet(t *testing.T) {
	cacheDir := t.TempDir()
	cache, err := NewFSCache(cacheDir)
	require.NoError(t, err)

	// Create source directory with content
	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("content"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(srcDir, "subdir"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "subdir", "nested.txt"), []byte("nested"), 0o644))

	// Set cache entry
	meta := &CacheMeta{
		OriginalRef: "gh:user/repo",
		ResolvedURL: "https://github.com/user/repo.git",
		Version:     "v1.0.0",
		FetchedAt:   time.Now(),
	}

	cachedPath, err := cache.Set("gh_user_repo@v1.0.0", srcDir, meta)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(cacheDir, "gh_user_repo@v1.0.0"), cachedPath)

	// Verify files were copied
	content, err := os.ReadFile(filepath.Join(cachedPath, "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "content", string(content))

	nested, err := os.ReadFile(filepath.Join(cachedPath, "subdir", "nested.txt"))
	require.NoError(t, err)
	assert.Equal(t, "nested", string(nested))

	// Get should return the cached path
	path, found, err := cache.Get("gh_user_repo@v1.0.0")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, cachedPath, path)
}

func TestUT_FSCache_Invalidate(t *testing.T) {
	cache, err := NewFSCache(t.TempDir())
	require.NoError(t, err)

	// Create and set a cache entry
	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("content"), 0o644))

	_, err = cache.Set("test_key", srcDir, &CacheMeta{
		OriginalRef: "test",
		FetchedAt:   time.Now(),
	})
	require.NoError(t, err)

	// Verify it exists
	_, found, err := cache.Get("test_key")
	require.NoError(t, err)
	assert.True(t, found)

	// Invalidate
	err = cache.Invalidate("test_key")
	require.NoError(t, err)

	// Verify it's gone
	_, found, err = cache.Get("test_key")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestUT_FSCache_MetadataWritten(t *testing.T) {
	cacheDir := t.TempDir()
	cache, err := NewFSCache(cacheDir)
	require.NoError(t, err)

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("content"), 0o644))

	meta := &CacheMeta{
		OriginalRef: "gh:user/repo@v1.0.0",
		ResolvedURL: "https://github.com/user/repo.git",
		Version:     "v1.0.0",
		FetchedAt:   time.Now(),
	}

	_, err = cache.Set("gh_user_repo@v1.0.0", srcDir, meta)
	require.NoError(t, err)

	// Read metadata directly
	readMeta, err := cache.readMeta("gh_user_repo@v1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "gh:user/repo@v1.0.0", readMeta.OriginalRef)
	assert.Equal(t, "https://github.com/user/repo.git", readMeta.ResolvedURL)
	assert.Equal(t, "v1.0.0", readMeta.Version)
}

func TestUT_FSCache_ExpiredEntry(t *testing.T) {
	cache, err := NewFSCache(t.TempDir())
	require.NoError(t, err)

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("content"), 0o644))

	// Set with already-expired time
	expired := time.Now().Add(-1 * time.Hour)
	meta := &CacheMeta{
		OriginalRef: "gh:user/repo",
		FetchedAt:   time.Now().Add(-2 * time.Hour),
		ExpiresAt:   &expired,
	}

	_, err = cache.Set("expired_key", srcDir, meta)
	require.NoError(t, err)

	// Get should return not found (expired)
	_, found, err := cache.Get("expired_key")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestUT_FSCache_PinnedVersionNeverExpires(t *testing.T) {
	cache, err := NewFSCache(t.TempDir())
	require.NoError(t, err)

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("content"), 0o644))

	// Pinned version (with Version set)
	meta := &CacheMeta{
		OriginalRef: "gh:user/repo@v1.0.0",
		Version:     "v1.0.0",
		FetchedAt:   time.Now().Add(-30 * 24 * time.Hour), // 30 days ago
	}

	_, err = cache.Set("pinned_key", srcDir, meta)
	require.NoError(t, err)

	// Read metadata to verify no expiration was set
	readMeta, err := cache.readMeta("pinned_key")
	require.NoError(t, err)
	assert.Nil(t, readMeta.ExpiresAt)

	// Get should still return found
	_, found, err := cache.Get("pinned_key")
	require.NoError(t, err)
	assert.True(t, found)
}

func TestUT_FSCache_NonPinnedGetsExpiration(t *testing.T) {
	cache, err := NewFSCache(t.TempDir())
	require.NoError(t, err)
	cache.SetTTL(1 * time.Hour)

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("content"), 0o644))

	// Non-pinned version (no Version set)
	meta := &CacheMeta{
		OriginalRef: "gh:user/repo",
		FetchedAt:   time.Now(),
	}

	_, err = cache.Set("nonpinned_key", srcDir, meta)
	require.NoError(t, err)

	// Read metadata to verify expiration was set
	readMeta, err := cache.readMeta("nonpinned_key")
	require.NoError(t, err)
	require.NotNil(t, readMeta.ExpiresAt)

	// Expiration should be roughly 1 hour from now
	expectedExpiry := time.Now().Add(1 * time.Hour)
	assert.WithinDuration(t, expectedExpiry, *readMeta.ExpiresAt, 5*time.Second)
}

func TestUT_FSCache_Path(t *testing.T) {
	cacheDir := t.TempDir()
	cache, err := NewFSCache(cacheDir)
	require.NoError(t, err)

	path := cache.Path("gh_user_repo@v1.0.0")
	assert.Equal(t, filepath.Join(cacheDir, "gh_user_repo@v1.0.0"), path)
}

func TestUT_FSCache_List(t *testing.T) {
	cacheDir := t.TempDir()
	cache, err := NewFSCache(cacheDir)
	require.NoError(t, err)

	// Create some cache entries
	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("content"), 0o644))

	_, err = cache.Set("key1", srcDir, &CacheMeta{FetchedAt: time.Now(), Version: "v1"})
	require.NoError(t, err)
	_, err = cache.Set("key2", srcDir, &CacheMeta{FetchedAt: time.Now(), Version: "v2"})
	require.NoError(t, err)

	keys, err := cache.List()
	require.NoError(t, err)
	assert.Len(t, keys, 2)
	assert.Contains(t, keys, "key1")
	assert.Contains(t, keys, "key2")
}

func TestUT_FSCache_Cleanup(t *testing.T) {
	cache, err := NewFSCache(t.TempDir())
	require.NoError(t, err)

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("content"), 0o644))

	// Create an expired entry
	expired := time.Now().Add(-1 * time.Hour)
	_, err = cache.Set("expired", srcDir, &CacheMeta{
		FetchedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: &expired,
	})
	require.NoError(t, err)

	// Create a valid entry
	_, err = cache.Set("valid", srcDir, &CacheMeta{
		FetchedAt: time.Now(),
		Version:   "v1.0.0", // Pinned, never expires
	})
	require.NoError(t, err)

	// Run cleanup
	err = cache.Cleanup()
	require.NoError(t, err)

	// Expired should be gone
	_, found, _ := cache.Get("expired")
	assert.False(t, found)

	// Valid should still exist
	_, found, _ = cache.Get("valid")
	assert.True(t, found)
}

func TestUT_FSCache_OverwritesExisting(t *testing.T) {
	cache, err := NewFSCache(t.TempDir())
	require.NoError(t, err)

	// First version
	srcDir1 := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir1, "file.txt"), []byte("version1"), 0o644))

	cachedPath, err := cache.Set("key", srcDir1, &CacheMeta{FetchedAt: time.Now(), Version: "v1"})
	require.NoError(t, err)

	// Verify first version
	content, err := os.ReadFile(filepath.Join(cachedPath, "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "version1", string(content))

	// Second version (overwrites)
	srcDir2 := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir2, "file.txt"), []byte("version2"), 0o644))

	_, err = cache.Set("key", srcDir2, &CacheMeta{FetchedAt: time.Now(), Version: "v2"})
	require.NoError(t, err)

	// Verify second version
	content, err = os.ReadFile(filepath.Join(cachedPath, "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "version2", string(content))
}
