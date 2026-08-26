package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/types"
)

func TestUT_ResolveGeneratorPaths_TagDirExistsWithoutGenerator(t *testing.T) {
	templateDir := setupLibEntryRoots(t, "roots-t1", map[string]string{
		filepath.Join(types.TemplatesDir, types.SharedDir, "shared.tmpl"): "shared",
		filepath.Join(types.GeneratorsDir, "mygen", "generator.tmpl"):     "gen",
	})
	localDir := t.TempDir()
	cfg := createTestConfigWithLib(t, localDir, "roots-t1")

	genDir, sharedDir, err := resolveGeneratorPaths(cfg, "mygen")

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(templateDir, types.GeneratorsDir, "mygen"), genDir)
	assert.Equal(t, filepath.Join(templateDir, types.TemplatesDir, types.SharedDir), sharedDir)
	assert.NoDirExists(t, filepath.Join(templateDir, types.TemplatesDir, "mygen"))
	assert.NoDirExists(t, filepath.Join(localDir, "mygen"))
}

func TestUT_ResolveGeneratorPaths_TagDirBeatsGeneratorsDir(t *testing.T) {
	// no-change guard: passes both before and after the #431 fix.
	templateDir := setupLibEntryRoots(t, "roots-t2", map[string]string{
		filepath.Join(types.TemplatesDir, "mygen", "generator.tmpl"):  "tag content",
		filepath.Join(types.GeneratorsDir, "mygen", "generator.tmpl"): "generators content",
	})
	cfg := createTestConfigWithLib(t, t.TempDir(), "roots-t2")

	genDir, _, err := resolveGeneratorPaths(cfg, "mygen")

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(templateDir, types.TemplatesDir, "mygen"), genDir)
}

func TestUT_ResolveFromLibrary_SharedDirProbedIndependently(t *testing.T) {
	t.Run("tag-shared-wins", func(t *testing.T) {
		templateDir := setupLibEntryRoots(t, "roots-t3a", map[string]string{
			filepath.Join(types.GeneratorsDir, "mygen", "generator.tmpl"): "gen",
			filepath.Join(types.TemplatesDir, types.SharedDir, "x.tmpl"):  "shared",
		})
		cfg := createTestConfigWithLib(t, t.TempDir(), "roots-t3a")

		_, sharedDir, found, err := resolveFromLibrary(cfg, "mygen")

		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, filepath.Join(templateDir, types.TemplatesDir, types.SharedDir), sharedDir)
	})

	t.Run("generators-shared", func(t *testing.T) {
		templateDir := setupLibEntryRoots(t, "roots-t3b", map[string]string{
			filepath.Join(types.TemplatesDir, "mygen", "generator.tmpl"):  "gen",
			filepath.Join(types.GeneratorsDir, types.SharedDir, "x.tmpl"): "shared",
		})
		cfg := createTestConfigWithLib(t, t.TempDir(), "roots-t3b")

		_, sharedDir, found, err := resolveFromLibrary(cfg, "mygen")

		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, filepath.Join(templateDir, types.GeneratorsDir, types.SharedDir), sharedDir)
	})

	t.Run("defaults-to-matched-root", func(t *testing.T) {
		templateDir := setupLibEntryRoots(t, "roots-t3c", map[string]string{
			filepath.Join(types.GeneratorsDir, "mygen", "generator.tmpl"): "gen",
		})
		cfg := createTestConfigWithLib(t, t.TempDir(), "roots-t3c")

		_, sharedDir, found, err := resolveFromLibrary(cfg, "mygen")

		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, filepath.Join(templateDir, types.GeneratorsDir, types.SharedDir), sharedDir)
		assert.NotEmpty(t, sharedDir)
	})
}

func TestUT_ResolveBundlePath_GeneratorsDirRoot(t *testing.T) {
	templateDir := setupLibEntryRoots(t, "roots-t4", map[string]string{
		filepath.Join(types.GeneratorsDir, types.BundlesDir, "foo", "foo.json"): `{"name":"foo"}`,
	})
	cfg := createTestConfigWithLib(t, t.TempDir(), "roots-t4")

	path, err := resolveBundlePath(cfg, "foo", "_bundles")

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(templateDir, types.GeneratorsDir, types.BundlesDir, "foo", "foo.json"), path)
}

func TestUT_ResolveGenerateTarget_GeneratorsDirGeneratorBeatsLibraryBundle(t *testing.T) {
	templateDir := setupLibEntryRoots(t, "roots-t5", map[string]string{
		filepath.Join(types.GeneratorsDir, "foo", "generator.tmpl"):            "gen",
		filepath.Join(types.TemplatesDir, types.BundlesDir, "foo", "foo.json"): `{"name":"foo"}`,
	})
	cfg := createTestConfigWithLib(t, t.TempDir(), "roots-t5")

	target, err := resolveGenerateTarget(cfg, "foo", "_bundles")

	require.NoError(t, err)
	require.NotNil(t, target)
	assert.False(t, target.IsBundle)
	assert.Equal(t, filepath.Join(templateDir, types.GeneratorsDir, "foo"), target.GenDir)
}

func TestUT_ResolveGeneratorPaths_LibraryGeneratorsDirBeatsProjectLocal(t *testing.T) {
	templateDir := setupLibEntryRoots(t, "roots-t9", map[string]string{
		filepath.Join(types.GeneratorsDir, "foo", "generator.tmpl"): "library-marker",
	})
	localDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(localDir, "foo"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(localDir, "foo", "generator.tmpl"), []byte("local-marker"), 0o644))
	cfg := createTestConfigWithLib(t, localDir, "roots-t9")

	genDir, _, err := resolveGeneratorPaths(cfg, "foo")

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(templateDir, types.GeneratorsDir, "foo"), genDir)
}
