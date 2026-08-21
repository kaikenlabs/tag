package remote

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
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

	assert.NoDirExists(t, cacheDir)

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("content"), 0o644))

	cachedPath, err := cache.Set("some-key", srcDir, &CacheMeta{})
	require.NoError(t, err)

	assert.DirExists(t, cacheDir)
	content, err := os.ReadFile(filepath.Join(cachedPath, "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "content", string(content))
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

	entries, err := cache.List()
	require.NoError(t, err)
	assert.Len(t, entries, 2)

	keyNames := make([]string, len(entries))
	for i, e := range entries {
		keyNames[i] = e.Key
	}
	assert.Contains(t, keyNames, "key1")
	assert.Contains(t, keyNames, "key2")
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
	_, err = cache.Cleanup()
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

func TestUT_FSCache_FailedSetPreservesPreviousEntry(t *testing.T) {
	cacheDir := t.TempDir()
	cache, err := NewFSCache(cacheDir)
	require.NoError(t, err)

	goodSrc := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(goodSrc, "file.txt"), []byte("v1-content"), 0o644))

	cachedPath, err := cache.Set("some-key", goodSrc, &CacheMeta{})
	require.NoError(t, err)

	path, found, err := cache.Get("some-key")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, cachedPath, path)
	content, err := os.ReadFile(filepath.Join(path, "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "v1-content", string(content))

	_, err = cache.Set("some-key", "/nonexistent/path/xyz", &CacheMeta{})
	assert.Error(t, err)

	path, found, err = cache.Get("some-key")
	require.NoError(t, err)
	assert.True(t, found, "previous entry must survive a failed Set")
	content, err = os.ReadFile(filepath.Join(path, "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "v1-content", string(content), "previous entry content must be unchanged after a failed Set")
}

func TestUT_FSCache_FailedSetMidCopyLeavesNoNewFiles(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("chmod-based permission denial is not meaningful on Windows or as root")
	}

	cacheDir := t.TempDir()
	cache, err := NewFSCache(cacheDir)
	require.NoError(t, err)

	v1Src := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(v1Src, "file.txt"), []byte("v1-content"), 0o644))

	_, err = cache.Set("some-key", v1Src, &CacheMeta{})
	require.NoError(t, err)

	v2Src := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(v2Src, "a_readable.txt"), []byte("v2-a"), 0o644))
	blocked := filepath.Join(v2Src, "b_blocked.txt")
	require.NoError(t, os.WriteFile(blocked, []byte("v2-b"), 0o644))
	require.NoError(t, os.Chmod(blocked, 0o000))
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o644) })
	require.NoError(t, os.WriteFile(filepath.Join(v2Src, "c_readable.txt"), []byte("v2-c"), 0o644))

	_, err = cache.Set("some-key", v2Src, &CacheMeta{})
	assert.Error(t, err)

	entryPath := cache.Path("some-key")
	entries, err := os.ReadDir(entryPath)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotEqual(t, "a_readable.txt", e.Name(), "no v2-only file should have leaked into the entry")
		assert.NotEqual(t, "b_blocked.txt", e.Name(), "no v2-only file should have leaked into the entry")
		assert.NotEqual(t, "c_readable.txt", e.Name(), "no v2-only file should have leaked into the entry")
	}

	content, err := os.ReadFile(filepath.Join(entryPath, "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "v1-content", string(content), "v1 content must be intact after a mid-copy failure")
}

func TestUT_FSCache_ConcurrentSet_NoDurableCorruption(t *testing.T) {
	const rounds = 8
	const numFiles = 8

	for round := range rounds {
		cacheDir := t.TempDir()
		cache, err := NewFSCache(cacheDir)
		require.NoError(t, err)

		srcA := t.TempDir()
		srcB := t.TempDir()
		for i := range numFiles {
			name := fmt.Sprintf("content_%02d.txt", i)
			require.NoError(t, os.WriteFile(filepath.Join(srcA, name), []byte("A"), 0o644))
			require.NoError(t, os.WriteFile(filepath.Join(srcB, name), []byte("B"), 0o644))
		}
		require.NoError(t, os.WriteFile(filepath.Join(srcA, "only_A.txt"), []byte("A"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(srcB, "only_B.txt"), []byte("B"), 0o644))

		var wg sync.WaitGroup
		wg.Go(func() {
			_, _ = cache.Set("concurrent-key", srcA, &CacheMeta{Version: "vA"})
		})
		wg.Go(func() {
			_, _ = cache.Set("concurrent-key", srcB, &CacheMeta{Version: "vB"})
		})
		wg.Wait()

		entryPath := cache.Path("concurrent-key")
		entries, err := os.ReadDir(entryPath)
		require.NoError(t, err)

		hasOnlyA := false
		hasOnlyB := false
		for _, e := range entries {
			if e.Name() == "only_A.txt" {
				hasOnlyA = true
			}
			if e.Name() == "only_B.txt" {
				hasOnlyB = true
			}
		}
		require.NotEqual(t, hasOnlyA, hasOnlyB, "round %d: exactly one marker file must be present, got only_A=%v only_B=%v", round, hasOnlyA, hasOnlyB)
		winner := "A"
		if hasOnlyB {
			winner = "B"
		}

		for i := range numFiles {
			name := fmt.Sprintf("content_%02d.txt", i)
			content, readErr := os.ReadFile(filepath.Join(entryPath, name))
			require.NoError(t, readErr, "round %d: %s must exist", round, name)
			assert.Equal(t, winner, string(content), "round %d: %s must belong entirely to the winning source", round, name)
		}

		meta, err := cache.readMeta("concurrent-key")
		require.NoError(t, err)
		expectedVersion := "vA"
		if winner == "B" {
			expectedVersion = "vB"
		}
		assert.Equal(t, expectedVersion, meta.Version, "round %d: _meta.json must agree with the winning source", round)
	}
}

func TestUT_FSCache_ConcurrentReader_EntryNeverMutatedInPlace(t *testing.T) {
	const numFiles = 24
	const writerRounds = 40

	cacheDir := t.TempDir()
	cache, err := NewFSCache(cacheDir)
	require.NoError(t, err)

	srcA := t.TempDir()
	srcB := t.TempDir()
	for i := range numFiles {
		name := fmt.Sprintf("content_%02d.txt", i)
		require.NoError(t, os.WriteFile(filepath.Join(srcA, name), []byte("A"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(srcB, name), []byte("B"), 0o644))
	}

	_, err = cache.Set("rw-key", srcA, &CacheMeta{Version: "vA"})
	require.NoError(t, err)

	var wg sync.WaitGroup
	writerDone := make(chan struct{})

	// Two writers, not one: a single writer's RemoveAll+MkdirAll allocates a
	// fresh inode each round, so a pinned reader only ever sees a retired
	// entry. Interleaving two writers is what produces the mixed tree that a
	// pinned reader can actually observe being mutated in place.
	var writers sync.WaitGroup
	for w := range 2 {
		writers.Go(func() {
			for i := range writerRounds {
				src, version := srcA, "vA"
				if (i+w)%2 == 1 {
					src, version = srcB, "vB"
				}
				_, _ = cache.Set("rw-key", src, &CacheMeta{Version: version})
				time.Sleep(time.Millisecond)
			}
		})
	}
	wg.Go(func() {
		defer close(writerDone)
		writers.Wait()
	})

	failures := 0
	retired := 0
	snapshots := 0

readLoop:
	for {
		select {
		case <-writerDone:
			break readLoop
		default:
		}

		path := cache.Path("rw-key")
		root, openErr := os.OpenRoot(path)
		if openErr != nil {
			retired++
			continue
		}

		var contents [numFiles]string
		mismatched := false
		for i := range numFiles {
			name := fmt.Sprintf("content_%02d.txt", i)
			f, ferr := root.Open(name)
			if ferr != nil {
				if os.IsNotExist(ferr) {
					retired++
					mismatched = true
					break
				}
				require.NoError(t, ferr)
			}
			data, rerr := io.ReadAll(f)
			f.Close()
			require.NoError(t, rerr)
			contents[i] = string(data)
		}
		if mismatched {
			root.Close()
			continue
		}

		metaFile, merr := root.Open(metaFileName)
		if merr != nil {
			if os.IsNotExist(merr) {
				retired++
				root.Close()
				continue
			}
			require.NoError(t, merr)
		}
		metaData, rerr := io.ReadAll(metaFile)
		metaFile.Close()
		require.NoError(t, rerr)
		root.Close()

		var meta CacheMeta
		require.NoError(t, json.Unmarshal(metaData, &meta))

		snapshots++
		for i := range numFiles {
			expected := "A"
			if meta.Version == "vB" {
				expected = "B"
			}
			if contents[i] != expected {
				failures++
				t.Errorf("snapshot inconsistency: meta.Version=%s but content_%02d.txt=%q (expected %q) - entry mutated in place", meta.Version, i, contents[i], expected)
			}
		}
	}

	wg.Wait()

	t.Logf("snapshots=%d retired=%d failures=%d", snapshots, retired, failures)
	assert.Zero(t, failures, "a pinned-root reader must never observe a mixed snapshot")
	assert.Positive(t, snapshots, "the reader must have observed at least one consistent snapshot")
}

func TestUT_FSCache_SetLeavesNoStagingDir(t *testing.T) {
	cacheDir := t.TempDir()
	cache, err := NewFSCache(cacheDir)
	require.NoError(t, err)

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("content"), 0o644))

	_, err = cache.Set("some-key", srcDir, &CacheMeta{})
	require.NoError(t, err)

	entries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.False(t, strings.HasPrefix(e.Name(), stagingPrefix), "no staging dir should remain after a successful Set, found %q", e.Name())
	}
}

func TestUT_FSCache_StagingDirsAreNotCacheEntries(t *testing.T) {
	cacheDir := t.TempDir()
	cache, err := NewFSCache(cacheDir)
	require.NoError(t, err)

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("content"), 0o644))

	_, err = cache.Set("real-key", srcDir, &CacheMeta{FetchedAt: time.Now(), Version: "v1"})
	require.NoError(t, err)

	plantStagingDir := func(t *testing.T, name string) string {
		t.Helper()
		dir := filepath.Join(cacheDir, name)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		data, marshalErr := json.MarshalIndent(&CacheMeta{FetchedAt: time.Now(), Version: "staged"}, "", "  ")
		require.NoError(t, marshalErr)
		require.NoError(t, os.WriteFile(filepath.Join(dir, metaFileName), data, 0o600))
		return dir
	}

	stagingDir := plantStagingDir(t, stagingPrefix+"planted")

	entries, err := cache.List()
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "real-key", entries[0].Key)

	removed, err := cache.ClearAll()
	require.NoError(t, err)
	assert.Equal(t, 1, removed, "ClearAll must count only the real entry, not the staging dir")
	assert.NoDirExists(t, stagingDir, "ClearAll must still remove staging dirs from disk")
	assert.NoDirExists(t, cache.Path("real-key"))

	_, err = cache.Set("real-key-2", srcDir, &CacheMeta{FetchedAt: time.Now(), Version: "v1"})
	require.NoError(t, err)

	staleDir := plantStagingDir(t, stagingPrefix+"stale")
	require.NoError(t, os.Chtimes(staleDir, time.Now().Add(-2*time.Hour), time.Now().Add(-2*time.Hour)))

	freshDir := plantStagingDir(t, stagingPrefix+"fresh")

	removedCount, err := cache.Cleanup()
	require.NoError(t, err)
	assert.Zero(t, removedCount, "Cleanup must not count reaped staging dirs as removed cache entries")
	assert.NoDirExists(t, staleDir, "Cleanup must reap a stale staging dir")
	assert.DirExists(t, freshDir, "Cleanup must leave a fresh staging dir alone")
	assert.DirExists(t, cache.Path("real-key-2"), "Cleanup must not touch a live, non-expired entry")
}
