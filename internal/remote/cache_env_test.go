package remote

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func listTree(t *testing.T, root string) []string {
	t.Helper()

	var got []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if path == root {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		got = append(got, filepath.ToSlash(rel))
		return nil
	})
	require.NoError(t, err)
	sort.Strings(got)
	return got
}

// TestUT_FSCache_BaseDirResolution asserts the resolved baseDir field and
// error for every precedence combination of the explicit argument, the
// TAG_CACHE_DIR env var, and the HOME fallback. This only proves the field
// was set correctly, not that the filesystem behaves accordingly — that is
// anchored by TestUT_FSCache_CacheDirEnv_WritesUnderEnvDir (env dir is
// actually used for I/O) and by the inverted TestUT_FSCache_NewFSCache
// (construction alone creates nothing) below, so do not delete those as
// "redundant" with this table.
func TestUT_FSCache_BaseDirResolution(t *testing.T) {
	tests := []struct {
		name      string
		arg       string
		env       string
		envSet    bool
		home      string
		homeSet   bool
		wantErr   string
		wantEqual func(home string) string
	}{
		{
			name:      "explicit absolute arg used as-is",
			arg:       "/explicit/absolute/cache",
			wantEqual: func(string) string { return "/explicit/absolute/cache" },
		},
		{
			name:      "explicit relative arg accepted for compat",
			arg:       "relative/cache",
			wantEqual: func(string) string { return "relative/cache" },
		},
		{
			name:      "env absolute wins over empty arg",
			env:       "/env/absolute/cache",
			envSet:    true,
			wantEqual: func(string) string { return "/env/absolute/cache" },
		},
		{
			name:    "env relative errors naming TAG_CACHE_DIR",
			env:     "relative/env/cache",
			envSet:  true,
			wantErr: "TAG_CACHE_DIR",
		},
		{
			name:      "env empty string treated as unset falls back to home",
			env:       "",
			envSet:    true,
			home:      "/home/user",
			homeSet:   true,
			wantEqual: func(home string) string { return filepath.Join(home, DefaultCacheDir) },
		},
		{
			name:      "both arg and env set, arg wins",
			arg:       "/explicit/wins",
			env:       "/env/loses",
			envSet:    true,
			wantEqual: func(string) string { return "/explicit/wins" },
		},
		{
			name:      "neither set falls back to home default",
			home:      "/home/user",
			homeSet:   true,
			wantEqual: func(home string) string { return filepath.Join(home, DefaultCacheDir) },
		},
		{
			name:      "env set and HOME empty still succeeds",
			env:       "/env/only",
			envSet:    true,
			home:      "",
			homeSet:   true,
			wantEqual: func(string) string { return "/env/only" },
		},
		{
			name:    "neither set and HOME empty errors",
			home:    "",
			homeSet: true,
			wantErr: "home directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envSet {
				t.Setenv("TAG_CACHE_DIR", tt.env)
			} else {
				t.Setenv("TAG_CACHE_DIR", "")
				require.NoError(t, os.Unsetenv("TAG_CACHE_DIR"))
			}
			if tt.homeSet {
				t.Setenv("HOME", tt.home)
			}

			cache, err := NewFSCache(tt.arg)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cache)
			assert.Equal(t, tt.wantEqual(tt.home), cache.baseDir)
		})
	}
}

// TestUT_FSCache_CacheDirEnv_WritesUnderEnvDir anchors
// TestUT_FSCache_BaseDirResolution in real filesystem behavior: TAG_CACHE_DIR
// must be where writes actually land, and HOME must be untouched.
func TestUT_FSCache_CacheDirEnv_WritesUnderEnvDir(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(home, 0o755))
	cacheDir := t.TempDir()

	t.Setenv("HOME", home)
	t.Setenv("TAG_CACHE_DIR", cacheDir)

	before := listTree(t, home)

	cache, err := NewFSCache("")
	require.NoError(t, err)
	require.NotNil(t, cache)

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("hello"), 0o644))

	cachedPath, err := cache.Set("some-key", srcDir, &CacheMeta{})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(cachedPath, "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "hello", string(content))
	assert.True(t, filepath.IsAbs(cachedPath))
	assert.Equal(t, cacheDir, filepath.Dir(cachedPath))

	after := listTree(t, home)
	assert.Equal(t, before, after, "HOME tree must be unchanged")
}

// TestUT_Resolver_Construction_CreatesNoCacheDir lives in package remote to
// reach the unexported cache field.
func TestUT_Resolver_Construction_CreatesNoCacheDir(t *testing.T) {
	home := t.TempDir()
	envRoot := t.TempDir()
	envDir := filepath.Join(envRoot, "does-not-exist-yet")

	t.Setenv("HOME", home)
	t.Setenv("TAG_CACHE_DIR", envDir)

	before := listTree(t, home)

	r, err := NewResolverWithOptions("", nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	fsCache, ok := r.cache.(*FSCache)
	require.True(t, ok)
	assert.Equal(t, envDir, fsCache.baseDir)

	assert.NoDirExists(t, envDir)

	after := listTree(t, home)
	assert.Equal(t, before, after, "HOME tree must be unchanged")
}
