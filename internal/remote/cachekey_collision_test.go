package remote

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUT_CacheKey_DistinctReferencesNeverCollide pins the property the readable
// prefix alone cannot provide. Owner and repo are validated only for
// path-traversal safety, so "_" — the prefix separator — is legal inside both,
// and sanitizeForPath folds several more characters onto it. Every pair below
// flattens to an identical prefix and must still key differently.
func TestUT_CacheKey_DistinctReferencesNeverCollide(t *testing.T) {
	t.Parallel()

	pairs := [][2]string{
		{"gl:a_b/c@v1.0.0", "gl:a/b_c@v1.0.0"},
		{"gh:x_y/z", "gh:x/y_z"},
		{"gh:user/repo@v1.0.0", "gh:user/repo@v2.0.0"},
		{"gh:user/repo", "gl:user/repo"},
	}

	for _, pair := range pairs {
		left, err := Parse(pair[0])
		require.NoError(t, err, pair[0])
		right, err := Parse(pair[1])
		require.NoError(t, err, pair[1])

		assert.NotEqual(t, left.CacheKey(), right.CacheKey(),
			"%q and %q are different repositories and must not share a cache entry", pair[0], pair[1])
	}
}

// TestUT_CacheKey_IsStableForTheSameReference guards the other direction: a
// digest that varied per call would make every lookup a miss and silently turn
// the cache off.
func TestUT_CacheKey_IsStableForTheSameReference(t *testing.T) {
	t.Parallel()

	ref, err := Parse("gh:user/repo@v1.0.0")
	require.NoError(t, err)

	again, err := Parse("gh:user/repo@v1.0.0")
	require.NoError(t, err)

	assert.Equal(t, ref.CacheKey(), again.CacheKey())
	assert.Equal(t, ref.CacheKey(), ref.CacheKey())
}

// TestUT_Resolver_CollidingRefsDoNotServeEachOthersTemplate is the end-to-end
// half: before the identity digest, the second Resolve returned the FIRST
// repository's cached directory — wrong file content, and a wrong CommitSHA
// surfacing as `template info`'s resolved_commit.
func TestUT_Resolver_CollidingRefsDoNotServeEachOthersTemplate(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()
	cache, err := NewFSCache(cacheDir)
	require.NoError(t, err)

	seed := func(ref string, marker, sha string) {
		parsed, parseErr := Parse(ref)
		require.NoError(t, parseErr)

		src := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(src, "tag.template.json"),
			[]byte(`{"name":"`+marker+`"}`), 0o600))

		_, setErr := cache.Set(parsed.CacheKey(), src, &CacheMeta{
			OriginalRef: ref,
			Version:     parsed.Version,
			CommitSHA:   sha,
		})
		require.NoError(t, setErr)
	}

	seed("gl:a_b/c@v1.0.0", "first", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	seed("gl:a/b_c@v1.0.0", "second", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")

	resolver, err := NewResolverWithOptions(cacheDir, nil)
	require.NoError(t, err)
	resolver.fetchers[ReferenceTypeGit] = &mockFetcher{
		fetchFunc: func(_ context.Context, _ *Reference) (*FetchResult, error) {
			t.Fatal("both refs are pinned and cached; no fetch should happen")
			return nil, nil //nolint:nilnil // unreachable after t.Fatal
		},
	}

	second, err := resolver.Resolve(context.Background(), "gl:a/b_c@v1.0.0", ResolveOptions{})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(second.Path, "tag.template.json"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "second",
		"resolving gl:a/b_c must not serve gl:a_b/c's cached template")
	assert.Equal(t, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", second.CommitSHA,
		"a colliding key would surface the other repository's commit as resolved_commit")
}
