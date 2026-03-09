package remote

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_Resolver_NewResolver(t *testing.T) {
	resolver, err := NewResolver()
	require.NoError(t, err)
	assert.NotNil(t, resolver)
	assert.NotNil(t, resolver.cache)
	assert.NotNil(t, resolver.auth)
	assert.NotNil(t, resolver.fetchers)
}

func TestUT_Resolver_ResolveLocalDir(t *testing.T) {
	// Create a local template directory
	tmpDir := t.TempDir()
	templateDir := filepath.Join(tmpDir, "my-template")
	require.NoError(t, os.MkdirAll(templateDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(templateDir, "tag.template.json"),
		[]byte(`{"name": "test"}`),
		0o644,
	))

	resolver, err := NewResolverWithOptions(filepath.Join(tmpDir, "cache"), nil)
	require.NoError(t, err)

	ctx := context.Background()
	result, err := resolver.Resolve(ctx, templateDir, ResolveOptions{})
	require.NoError(t, err)

	// Should return the same path (local dirs don't get cached)
	assert.Equal(t, templateDir, result.Path)
	// Local dirs have no commit SHA
	assert.Empty(t, result.CommitSHA)
}

func TestUT_Resolver_ResolveLocalZip(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test zip
	zipPath := filepath.Join(tmpDir, "template.zip")
	createTestZip(t, zipPath, map[string]string{
		"tag.template.json": `{"name": "test"}`,
		"README.md":         "# Test",
	})

	resolver, err := NewResolverWithOptions(filepath.Join(tmpDir, "cache"), nil)
	require.NoError(t, err)

	ctx := context.Background()
	result, err := resolver.Resolve(ctx, zipPath, ResolveOptions{})
	require.NoError(t, err)
	defer os.RemoveAll(result.Path)

	// Should be extracted to a temp directory
	assert.NotEqual(t, zipPath, result.Path)

	// Verify content was extracted
	_, err = os.Stat(filepath.Join(result.Path, "tag.template.json"))
	assert.NoError(t, err)

	// Zip sources have no commit SHA
	assert.Empty(t, result.CommitSHA)
}

func TestUT_Resolver_CacheHit(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")

	// Pre-populate cache
	cache, err := NewFSCache(cacheDir)
	require.NoError(t, err)

	// Create source content
	srcDir := filepath.Join(tmpDir, "src")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "tag.template.json"), []byte(`{}`), 0o644))

	// Cache it under the pinned key with a commit SHA
	_, err = cache.Set("gh_user_repo@v1.0.0", srcDir, &CacheMeta{
		OriginalRef: "gh:user/repo@v1.0.0",
		FetchedAt:   time.Now(),
		Version:     "v1.0.0",
		CommitSHA:   "abc123def456789012345678901234567890abcd",
	})
	require.NoError(t, err)

	// Create resolver
	resolver, err := NewResolverWithOptions(cacheDir, nil)
	require.NoError(t, err)

	// Mock fetcher that should NOT be called for a pinned ref
	mockFetcher := &mockFetcher{
		fetchFunc: func(_ context.Context, _ *Reference) (*FetchResult, error) {
			t.Fatal("Fetcher should not be called for pinned cache hit")
			return nil, nil //nolint:nilnil // unreachable after t.Fatal
		},
	}
	resolver.fetchers[ReferenceTypeGit] = mockFetcher

	// Resolve pinned ref - should hit cache
	ctx := context.Background()
	result, err := resolver.Resolve(ctx, "gh:user/repo@v1.0.0", ResolveOptions{})
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(cacheDir, "gh_user_repo@v1.0.0"), result.Path)
	assert.Equal(t, "abc123def456789012345678901234567890abcd", result.CommitSHA)
	assert.Equal(t, "v1.0.0", result.Version)
}

func TestUT_Resolver_CacheHit_BackwardCompatMeta(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")

	// Pre-populate cache with old-style meta (no commit_sha field)
	cache, err := NewFSCache(cacheDir)
	require.NoError(t, err)

	srcDir := filepath.Join(tmpDir, "src")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "tag.template.json"), []byte(`{}`), 0o644))

	_, err = cache.Set("gh_user_repo@v1.0.0", srcDir, &CacheMeta{
		OriginalRef: "gh:user/repo@v1.0.0",
		FetchedAt:   time.Now(),
		Version:     "v1.0.0",
		// No CommitSHA — simulates old cache entries
	})
	require.NoError(t, err)

	resolver, err := NewResolverWithOptions(cacheDir, nil)
	require.NoError(t, err)

	mockFetcher := &mockFetcher{
		fetchFunc: func(_ context.Context, _ *Reference) (*FetchResult, error) {
			t.Fatal("Fetcher should not be called for pinned cache hit")
			return nil, nil //nolint:nilnil // unreachable after t.Fatal
		},
	}
	resolver.fetchers[ReferenceTypeGit] = mockFetcher

	ctx := context.Background()
	result, err := resolver.Resolve(ctx, "gh:user/repo@v1.0.0", ResolveOptions{})
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(cacheDir, "gh_user_repo@v1.0.0"), result.Path)
	assert.Empty(t, result.CommitSHA, "old cache entries should deserialize with empty CommitSHA")
}

func TestUT_Resolver_FloatingRefAlwaysFetches(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")

	// Pre-populate cache as if a previous fetch happened
	cache, err := NewFSCache(cacheDir)
	require.NoError(t, err)

	srcDir := filepath.Join(tmpDir, "src")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "tag.template.json"), []byte(`{}`), 0o644))

	_, err = cache.Set("gh_user_repo", srcDir, &CacheMeta{
		OriginalRef: "gh:user/repo",
		FetchedAt:   time.Now(),
	})
	require.NoError(t, err)

	resolver, err := NewResolverWithOptions(cacheDir, nil)
	require.NoError(t, err)

	fetchCalled := false
	newContent := filepath.Join(tmpDir, "new")
	require.NoError(t, os.MkdirAll(newContent, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(newContent, "tag.template.json"), []byte(`{}`), 0o644))

	mockFetcher := &mockFetcher{
		fetchFunc: func(_ context.Context, _ *Reference) (*FetchResult, error) {
			fetchCalled = true
			return &FetchResult{Path: newContent, CommitSHA: "abc123"}, nil
		},
	}
	resolver.fetchers[ReferenceTypeGit] = mockFetcher

	// Floating ref (no version) - must always fetch even with a cached entry
	ctx := context.Background()
	_, err = resolver.Resolve(ctx, "gh:user/repo", ResolveOptions{})
	require.NoError(t, err)

	assert.True(t, fetchCalled, "Fetcher must be called for floating refs regardless of cache")
}

func TestUT_Resolver_ForceUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")

	// Pre-populate cache
	cache, err := NewFSCache(cacheDir)
	require.NoError(t, err)

	srcDir := filepath.Join(tmpDir, "src")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("old"), 0o644))

	_, err = cache.Set("gh_user_repo", srcDir, &CacheMeta{
		OriginalRef: "gh:user/repo",
		FetchedAt:   time.Now(),
		Version:     "v1.0.0",
	})
	require.NoError(t, err)

	// Create resolver with mock fetcher
	resolver, err := NewResolverWithOptions(cacheDir, nil)
	require.NoError(t, err)

	fetchCalled := false
	newContent := filepath.Join(tmpDir, "new")
	require.NoError(t, os.MkdirAll(newContent, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(newContent, "file.txt"), []byte("new"), 0o644))

	mockFetcher := &mockFetcher{
		fetchFunc: func(_ context.Context, _ *Reference) (*FetchResult, error) {
			fetchCalled = true
			return &FetchResult{Path: newContent, CommitSHA: "def456"}, nil
		},
	}
	resolver.fetchers[ReferenceTypeGit] = mockFetcher

	// Resolve with ForceUpdate - should call fetcher
	ctx := context.Background()
	_, err = resolver.Resolve(ctx, "gh:user/repo", ResolveOptions{ForceUpdate: true})
	require.NoError(t, err)

	assert.True(t, fetchCalled, "Fetcher should be called with ForceUpdate")
}

func TestUT_Resolver_OfflineMode(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")

	resolver, err := NewResolverWithOptions(cacheDir, nil)
	require.NoError(t, err)

	// Resolve in offline mode when not cached
	ctx := context.Background()
	_, err = resolver.Resolve(ctx, "gh:user/repo", ResolveOptions{Offline: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "offline")
}

func TestUT_Resolver_InvalidReference(t *testing.T) {
	resolver, err := NewResolver()
	require.NoError(t, err)

	ctx := context.Background()
	_, err = resolver.Resolve(ctx, "", ResolveOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}

func TestUT_ApplySubPath_Containment(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")

	resolver, err := NewResolverWithOptions(cacheDir, nil)
	require.NoError(t, err)

	// Create a base directory with a subdirectory
	baseDir := filepath.Join(tmpDir, "base")
	subDir := filepath.Join(baseDir, "templates")
	require.NoError(t, os.MkdirAll(subDir, 0o755))

	t.Run("valid subpath returns correct path", func(t *testing.T) {
		path, err := resolver.applySubPath(baseDir, "templates")
		require.NoError(t, err)
		assert.Equal(t, subDir, path)
	})

	t.Run("empty subpath returns base path", func(t *testing.T) {
		path, err := resolver.applySubPath(baseDir, "")
		require.NoError(t, err)
		assert.Equal(t, baseDir, path)
	})

	t.Run("dotdot traversal is rejected", func(t *testing.T) {
		_, err := resolver.applySubPath(baseDir, "../../etc/passwd")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path traversal")
	})

	t.Run("nonexistent subpath returns error", func(t *testing.T) {
		_, err := resolver.applySubPath(baseDir, "nonexistent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestUT_IsLocal(t *testing.T) {
	// Create temp directory for local path test
	tmpDir := t.TempDir()

	tests := []struct {
		input    string
		expected bool
	}{
		{tmpDir, true},
		{"gh:user/repo", false},
		{"https://example.com/template.zip", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsLocal(tt.input))
		})
	}
}

func TestUT_Resolver_CommitSHA_StoredInCache(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")

	resolver, err := NewResolverWithOptions(cacheDir, nil)
	require.NoError(t, err)

	newContent := filepath.Join(tmpDir, "content")
	require.NoError(t, os.MkdirAll(newContent, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(newContent, "file.txt"), []byte("data"), 0o644))

	expectedSHA := "abc123def456789012345678901234567890abcd"
	mockFetcher := &mockFetcher{
		fetchFunc: func(_ context.Context, _ *Reference) (*FetchResult, error) {
			return &FetchResult{
				Path:      newContent,
				CommitSHA: expectedSHA,
				Version:   "v2.0.0",
			}, nil
		},
	}
	resolver.fetchers[ReferenceTypeGit] = mockFetcher

	ctx := context.Background()
	result, err := resolver.Resolve(ctx, "gh:user/repo@v2.0.0", ResolveOptions{})
	require.NoError(t, err)

	assert.Equal(t, expectedSHA, result.CommitSHA)

	// Verify SHA is persisted in cache metadata
	fsCache := resolver.cache.(*FSCache)
	meta, err := fsCache.readMeta("gh_user_repo@v2.0.0")
	require.NoError(t, err)
	assert.Equal(t, expectedSHA, meta.CommitSHA)
}

// mockFetcher is a test double for Fetcher
type mockFetcher struct {
	fetchFunc func(ctx context.Context, ref *Reference) (*FetchResult, error)
}

func (m *mockFetcher) Fetch(ctx context.Context, ref *Reference) (*FetchResult, error) {
	if m.fetchFunc != nil {
		return m.fetchFunc(ctx, ref)
	}
	return &FetchResult{}, nil
}
