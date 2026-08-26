package commands

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestUT_ResolveGeneratorPaths_LibraryFileDoesNotShadowProjectLocal(t *testing.T) {
	setupLibEntryRoots(t, "roots-t11", map[string]string{
		filepath.Join(types.GeneratorsDir, "foo"): "not a generator directory",
	})
	localDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(localDir, "foo"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(localDir, "foo", "generator.tmpl"), []byte("local"), 0o644))
	cfg := createTestConfigWithLib(t, localDir, "roots-t11")

	genDir, _, err := resolveGeneratorPaths(cfg, "foo")

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(localDir, "foo"), genDir)
}

func TestUT_ResolveBundlePath_LibraryDirectoryDoesNotShadowLocalBundle(t *testing.T) {
	setupLibEntryRoots(t, "roots-t12", map[string]string{
		filepath.Join(types.GeneratorsDir, types.BundlesDir, "foo", "foo.json", "placeholder"): "a directory, not a manifest",
	})
	localDir := t.TempDir()
	localBundle := filepath.Join(localDir, types.BundlesDir, "foo")
	require.NoError(t, os.MkdirAll(localBundle, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(localBundle, "foo.json"), []byte(`{"name":"foo"}`), 0o644))
	cfg := createTestConfigWithLib(t, localDir, "roots-t12")

	bundlePath, err := resolveBundlePath(cfg, "foo", types.BundlesDir)

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(localBundle, "foo.json"), bundlePath)
}

// TestUT_ResolveGenerateTarget_EmptyGeneratorDirDoesNotBeatBundle pins #436:
// an empty generator directory must not shadow a same-named, working bundle.
// It is checked at all three sites that decide "generator vs bundle" —
// the library .tag/ root, the library _generators/ root, and the
// project-local .tag/ fallback — each paired with a populated-dir positive
// control that passes on unfixed code too.
func TestUT_ResolveGenerateTarget_EmptyGeneratorDirDoesNotBeatBundle(t *testing.T) {
	t.Run("library .tag/ root: empty dir falls through to bundle", func(t *testing.T) {
		templateDir := setupLibEntryRoots(t, "roots-t436-a", map[string]string{
			filepath.Join(types.TemplatesDir, types.BundlesDir, "foo", "foo.json"): `{"name":"foo"}`,
		})
		genDir := filepath.Join(templateDir, types.TemplatesDir, "foo")
		require.NoError(t, os.MkdirAll(genDir, 0o750))
		entries, err := os.ReadDir(genDir)
		require.NoError(t, err)
		require.Empty(t, entries, "fixture invariant: generator dir must actually be empty")

		cfg := createTestConfigWithLib(t, t.TempDir(), "roots-t436-a")
		target, err := resolveGenerateTarget(cfg, "foo", types.BundlesDir)

		require.NoError(t, err)
		require.NotNil(t, target)
		assert.True(t, target.IsBundle, "an empty generator dir must not beat a working bundle")
	})

	t.Run("library .tag/ root: populated dir still beats bundle (positive control)", func(t *testing.T) {
		templateDir := setupLibEntryRoots(t, "roots-t436-b", map[string]string{
			filepath.Join(types.TemplatesDir, "foo", "gen.tmpl"):                   "gen",
			filepath.Join(types.TemplatesDir, types.BundlesDir, "foo", "foo.json"): `{"name":"foo"}`,
		})
		cfg := createTestConfigWithLib(t, t.TempDir(), "roots-t436-b")
		target, err := resolveGenerateTarget(cfg, "foo", types.BundlesDir)

		require.NoError(t, err)
		require.NotNil(t, target)
		assert.False(t, target.IsBundle)
		assert.Equal(t, filepath.Join(templateDir, types.TemplatesDir, "foo"), target.GenDir)
	})

	t.Run("library _generators/ root: empty dir falls through to bundle", func(t *testing.T) {
		templateDir := setupLibEntryRoots(t, "roots-t436-c", map[string]string{
			filepath.Join(types.GeneratorsDir, types.BundlesDir, "foo", "foo.json"): `{"name":"foo"}`,
		})
		genDir := filepath.Join(templateDir, types.GeneratorsDir, "foo")
		require.NoError(t, os.MkdirAll(genDir, 0o750))
		entries, err := os.ReadDir(genDir)
		require.NoError(t, err)
		require.Empty(t, entries, "fixture invariant: generator dir must actually be empty")

		cfg := createTestConfigWithLib(t, t.TempDir(), "roots-t436-c")
		target, err := resolveGenerateTarget(cfg, "foo", types.BundlesDir)

		require.NoError(t, err)
		require.NotNil(t, target)
		assert.True(t, target.IsBundle, "an empty generator dir must not beat a working bundle")
	})

	t.Run("library _generators/ root: populated dir still beats bundle (positive control)", func(t *testing.T) {
		templateDir := setupLibEntryRoots(t, "roots-t436-d", map[string]string{
			filepath.Join(types.GeneratorsDir, "foo", "gen.tmpl"):                   "gen",
			filepath.Join(types.GeneratorsDir, types.BundlesDir, "foo", "foo.json"): `{"name":"foo"}`,
		})
		cfg := createTestConfigWithLib(t, t.TempDir(), "roots-t436-d")
		target, err := resolveGenerateTarget(cfg, "foo", types.BundlesDir)

		require.NoError(t, err)
		require.NotNil(t, target)
		assert.False(t, target.IsBundle)
		assert.Equal(t, filepath.Join(templateDir, types.GeneratorsDir, "foo"), target.GenDir)
	})

	t.Run("project-local .tag/: empty dir falls through to bundle", func(t *testing.T) {
		localDir := t.TempDir()
		genDir := filepath.Join(localDir, "foo")
		require.NoError(t, os.MkdirAll(genDir, 0o750))
		entries, err := os.ReadDir(genDir)
		require.NoError(t, err)
		require.Empty(t, entries, "fixture invariant: generator dir must actually be empty")

		bundleDir := filepath.Join(localDir, types.BundlesDir, "foo")
		require.NoError(t, os.MkdirAll(bundleDir, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "foo.json"), []byte(`{"name":"foo"}`), 0o644))

		cfg := createTestConfig(t, localDir)
		target, err := resolveGenerateTarget(cfg, "foo", types.BundlesDir)

		require.NoError(t, err)
		require.NotNil(t, target)
		assert.True(t, target.IsBundle, "an empty generator dir must not beat a working bundle")
	})

	t.Run("project-local .tag/: populated dir still beats bundle (positive control)", func(t *testing.T) {
		localDir := t.TempDir()
		genDir := filepath.Join(localDir, "foo")
		require.NoError(t, os.MkdirAll(genDir, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(genDir, "gen.tmpl"), []byte("gen"), 0o644))

		bundleDir := filepath.Join(localDir, types.BundlesDir, "foo")
		require.NoError(t, os.MkdirAll(bundleDir, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "foo.json"), []byte(`{"name":"foo"}`), 0o644))

		cfg := createTestConfig(t, localDir)
		target, err := resolveGenerateTarget(cfg, "foo", types.BundlesDir)

		require.NoError(t, err)
		require.NotNil(t, target)
		assert.False(t, target.IsBundle)
		assert.Equal(t, genDir, target.GenDir)
	})
}

// TestUT_ResolveGeneratorPaths_EmptyTagDirFallsThroughToGeneratorsDir is the
// sibling of TestUT_ResolveGeneratorPaths_TagDirBeatsGeneratorsDir: when
// .tag/ wins the collision it must be because it actually holds a generator,
// not merely because it exists. The reverse mutation (probing _generators/
// first) is caught by TestUT_ResolveGeneratorPaths_TagDirBeatsGeneratorsDir,
// so both tests must remain.
func TestUT_ResolveGeneratorPaths_EmptyTagDirFallsThroughToGeneratorsDir(t *testing.T) {
	templateDir := setupLibEntryRoots(t, "roots-t436-e", map[string]string{
		filepath.Join(types.GeneratorsDir, "foo", "gen.tmpl"): "generators content",
	})
	tagFooDir := filepath.Join(templateDir, types.TemplatesDir, "foo")
	require.NoError(t, os.MkdirAll(tagFooDir, 0o750))
	entries, err := os.ReadDir(tagFooDir)
	require.NoError(t, err)
	require.Empty(t, entries, "fixture invariant: .tag/foo must actually be empty")

	cfg := createTestConfigWithLib(t, t.TempDir(), "roots-t436-e")

	genDir, _, err := resolveGeneratorPaths(cfg, "foo")

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(templateDir, types.GeneratorsDir, "foo"), genDir)
}

// TestUT_ResolveGeneratorPaths_EmptyDirWithNoBundleIsNotFound pins the
// accepted #436 behaviour change: an empty generator directory with no
// same-named bundle must now fail loudly instead of silently no-oping.
func TestUT_ResolveGeneratorPaths_EmptyDirWithNoBundleIsNotFound(t *testing.T) {
	localDir := t.TempDir()
	genDir := filepath.Join(localDir, "foo")
	require.NoError(t, os.MkdirAll(genDir, 0o750))
	entries, err := os.ReadDir(genDir)
	require.NoError(t, err)
	require.Empty(t, entries, "fixture invariant: generator dir must actually be empty")

	cfg := createTestConfig(t, localDir)

	_, _, err = resolveGeneratorPaths(cfg, "foo")

	require.Error(t, err)
	var notFound *GeneratorNotFoundError
	require.ErrorAs(t, err, &notFound)
}

// TestUT_ResolveBundlePath_EmptyDirectoryUnaffected is a no-change guard: it
// passes on both sides of the #436 fix, pinning that bundle resolution keeps
// working while a same-named empty directory exists.
//
// It does NOT discriminate dropping the `!wantDir ||` gate: a bundle candidate
// is a FILE, and HasTemplateFiles fails open on a file just as it does on an
// unreadable directory, so the ungated form is behaviourally identical at
// every call site that exists today. The gate is defensive, not load-bearing.
func TestUT_ResolveBundlePath_EmptyDirectoryUnaffected(t *testing.T) {
	templateDir := setupLibEntryRoots(t, "roots-t436-f", map[string]string{
		filepath.Join(types.GeneratorsDir, types.BundlesDir, "foo", "foo.json"): `{"name":"foo"}`,
	})
	emptyDir := filepath.Join(templateDir, types.GeneratorsDir, "unrelated-empty")
	require.NoError(t, os.MkdirAll(emptyDir, 0o750))
	entries, err := os.ReadDir(emptyDir)
	require.NoError(t, err)
	require.Empty(t, entries, "fixture invariant: unrelated dir must actually be empty")

	cfg := createTestConfigWithLib(t, t.TempDir(), "roots-t436-f")

	path, err := resolveBundlePath(cfg, "foo", types.BundlesDir)

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(templateDir, types.GeneratorsDir, types.BundlesDir, "foo", "foo.json"), path)
}

// TestUT_ResolveGenerateTarget_UnreadableDirStillBeatsBundle is a
// mutation guard: an unreadable generator directory must still resolve as
// the generator (so the loud read failure the caller expects survives),
// never silently fall through to a same-named bundle. This is the only test
// that fails if the ReadDir-error branch of HasTemplateFiles is
// "simplified" to return false.
func TestUT_ResolveGenerateTarget_UnreadableDirStillBeatsBundle(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("chmod-based permission denial is not meaningful on Windows or as root")
	}

	localDir := t.TempDir()
	genDir := filepath.Join(localDir, "foo")
	require.NoError(t, os.MkdirAll(genDir, 0o750))
	require.NoError(t, os.Chmod(genDir, 0o000))
	t.Cleanup(func() { _ = os.Chmod(genDir, 0o750) })

	_, readErr := os.ReadDir(genDir)
	require.Error(t, readErr, "fixture invariant: ReadDir must actually fail for this test to mean anything")

	bundleDir := filepath.Join(localDir, types.BundlesDir, "foo")
	require.NoError(t, os.MkdirAll(bundleDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "foo.json"), []byte(`{"name":"foo"}`), 0o644))

	cfg := createTestConfig(t, localDir)

	target, err := resolveGenerateTarget(cfg, "foo", types.BundlesDir)

	require.NoError(t, err)
	require.NotNil(t, target)
	assert.False(t, target.IsBundle, "an unreadable generator dir must still beat a same-named bundle so the loud read failure surfaces")
}

// TestUT_ResolveGenerateTarget_LocalRegularFileDoesNotBeatBundle pins that the
// project-local fallback requires a DIRECTORY. HasTemplateFiles fails open on
// a non-directory path, so without the IsDir guard a regular file named after
// a bundle resolved as a generator and the run died reading a config inside a
// file. The library path has carried this guard since #431.
func TestUT_ResolveGenerateTarget_LocalRegularFileDoesNotBeatBundle(t *testing.T) {
	localDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(localDir, "foo"), []byte("not a dir"), 0o644))

	bundleDir := filepath.Join(localDir, types.BundlesDir, "foo")
	require.NoError(t, os.MkdirAll(bundleDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "foo.json"), []byte(`{"name":"foo"}`), 0o644))

	info, statErr := os.Stat(filepath.Join(localDir, "foo"))
	require.NoError(t, statErr)
	require.False(t, info.IsDir(), "fixture invariant: the candidate must be a regular file")

	cfg := createTestConfig(t, localDir)

	target, err := resolveGenerateTarget(cfg, "foo", types.BundlesDir)

	require.NoError(t, err)
	assert.True(t, target.IsBundle, "a regular file must not resolve as a generator")
	assert.Equal(t, filepath.Join(bundleDir, "foo.json"), target.BundlePath)
}
