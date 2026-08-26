package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/internal/types/flags"
)

func TestUT_ResolveBasePath_Lib_ChoosesExistingRoot(t *testing.T) {
	t.Run("prefers-existing-tag-dir", func(t *testing.T) {
		// no-change guard: passes both before and after the #431 fix.
		templateDir := setupFakeLibrary(t, "basepath-both")
		require.NoError(t, os.MkdirAll(filepath.Join(templateDir, types.TemplatesDir), 0o750))
		require.NoError(t, os.MkdirAll(filepath.Join(templateDir, types.GeneratorsDir), 0o750))

		cfg := createTestConfigWithLib(t, t.TempDir(), "basepath-both")
		ctx := createTestCLIContext(t, nil, map[string]any{flags.LibFlag: true})

		basePath, err := resolveBasePath(ctx, cfg)

		require.NoError(t, err)
		assert.Equal(t, filepath.Join(templateDir, types.TemplatesDir), basePath)
	})

	t.Run("uses-generators-dir", func(t *testing.T) {
		templateDir := setupFakeLibrary(t, "basepath-gens-only")
		require.NoError(t, os.MkdirAll(filepath.Join(templateDir, types.GeneratorsDir), 0o750))
		require.NoDirExists(t, filepath.Join(templateDir, types.TemplatesDir))

		cfg := createTestConfigWithLib(t, t.TempDir(), "basepath-gens-only")
		ctx := createTestCLIContext(t, nil, map[string]any{flags.LibFlag: true})

		basePath, err := resolveBasePath(ctx, cfg)

		require.NoError(t, err)
		assert.Equal(t, filepath.Join(templateDir, types.GeneratorsDir), basePath)
		assert.NoDirExists(t, filepath.Join(templateDir, types.TemplatesDir))
	})

	t.Run("creates-tag-dir-when-neither-exists", func(t *testing.T) {
		// no-change guard: passes both before and after the #431 fix.
		templateDir := setupFakeLibrary(t, "basepath-neither")

		cfg := createTestConfigWithLib(t, t.TempDir(), "basepath-neither")
		ctx := createTestCLIContext(t, nil, map[string]any{flags.LibFlag: true})

		basePath, err := resolveBasePath(ctx, cfg)

		require.NoError(t, err)
		assert.Equal(t, filepath.Join(templateDir, types.TemplatesDir), basePath)
		assert.DirExists(t, basePath)
	})

	t.Run("bundle-action-uses-same-root", func(t *testing.T) {
		templateDir := setupFakeLibrary(t, "basepath-bundle-gens-only")
		require.NoError(t, os.MkdirAll(filepath.Join(templateDir, types.GeneratorsDir), 0o750))

		cfg := createTestConfigWithLib(t, t.TempDir(), "basepath-bundle-gens-only")
		ctx := createTestCLIContext(t, []string{"mybundle"}, map[string]any{flags.LibFlag: true})

		err := bundleAction(ctx, cfg)

		require.NoError(t, err)
		assert.FileExists(t, filepath.Join(templateDir, types.GeneratorsDir, types.BundlesDir, "mybundle", "mybundle.json"))
	})
}

func TestUT_ResolveBasePath_Lib_SkipsRootThatIsAFile(t *testing.T) {
	templateDir := setupFakeLibrary(t, "basepath-tag-is-file")
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, types.TemplatesDir), []byte("not a directory"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(templateDir, types.GeneratorsDir), 0o750))

	cfg := createTestConfigWithLib(t, t.TempDir(), "basepath-tag-is-file")
	ctx := createTestCLIContext(t, nil, map[string]any{flags.LibFlag: true})

	basePath, err := resolveBasePath(ctx, cfg)

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(templateDir, types.GeneratorsDir), basePath)
}
