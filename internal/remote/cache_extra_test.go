package remote

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_FSCacheExtra_NewFSCache_UsesHomeWhenBaseDirEmpty(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cache, err := NewFSCache("")
	require.NoError(t, err)
	require.NotNil(t, cache)

	expected := filepath.Join(home, DefaultCacheDir)
	assert.Equal(t, expected, cache.baseDir)
	assert.NoDirExists(t, expected)
}

func TestUT_FSCacheExtra_ClearExpired_RemovesOnlyExpiredEntries(t *testing.T) {
	t.Parallel()

	cache, err := NewFSCache(t.TempDir())
	require.NoError(t, err)

	src := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(src, "file.txt"), []byte("x"), 0o644))

	expiredAt := time.Now().Add(-time.Minute)
	_, err = cache.Set("expired", src, &CacheMeta{FetchedAt: time.Now().Add(-2 * time.Hour), ExpiresAt: &expiredAt})
	require.NoError(t, err)
	_, err = cache.Set("pinned", src, &CacheMeta{FetchedAt: time.Now(), Version: "v1.0.0"})
	require.NoError(t, err)

	removed, err := cache.ClearExpired()
	require.NoError(t, err)
	assert.Equal(t, 1, removed)

	_, foundExpired, err := cache.Get("expired")
	require.NoError(t, err)
	assert.False(t, foundExpired)

	_, foundPinned, err := cache.Get("pinned")
	require.NoError(t, err)
	assert.True(t, foundPinned)
}

func TestUT_FSCacheExtra_ClearAll_RemovesDirectoriesOnly(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	cache, err := NewFSCache(base)
	require.NoError(t, err)

	src := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(src, "file.txt"), []byte("x"), 0o644))
	_, err = cache.Set("one", src, &CacheMeta{FetchedAt: time.Now(), Version: "v1"})
	require.NoError(t, err)
	_, err = cache.Set("two", src, &CacheMeta{FetchedAt: time.Now(), Version: "v2"})
	require.NoError(t, err)

	// Non-directory entries should be ignored by ClearAll.
	require.NoError(t, os.WriteFile(filepath.Join(base, "note.txt"), []byte("ignore"), 0o644))

	removed, err := cache.ClearAll()
	require.NoError(t, err)
	assert.Equal(t, 2, removed)

	entries, err := os.ReadDir(base)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "note.txt", entries[0].Name())
}

func TestUT_FSCacheExtra_List_ReturnsNilMetaForCorruptOrMissingMeta(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	cache, err := NewFSCache(base)
	require.NoError(t, err)

	validSrc := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(validSrc, "f.txt"), []byte("ok"), 0o644))
	_, err = cache.Set("valid", validSrc, &CacheMeta{FetchedAt: time.Now(), Version: "v1"})
	require.NoError(t, err)

	missingMetaDir := filepath.Join(base, "missing-meta")
	require.NoError(t, os.MkdirAll(missingMetaDir, 0o755))

	corruptMetaDir := filepath.Join(base, "corrupt-meta")
	require.NoError(t, os.MkdirAll(corruptMetaDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(corruptMetaDir, "_meta.json"), []byte("{"), 0o644))

	entries, err := cache.List()
	require.NoError(t, err)
	require.Len(t, entries, 3)

	metaByKey := map[string]*CacheMeta{}
	for _, e := range entries {
		metaByKey[e.Key] = e.Meta
	}

	require.NotNil(t, metaByKey["valid"])
	assert.Nil(t, metaByKey["missing-meta"])
	assert.Nil(t, metaByKey["corrupt-meta"])
}
