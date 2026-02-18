package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_ResolveGenerateTarget_GeneratorOnly(t *testing.T) {
	tmpDir := t.TempDir()

	templateContent := "---\nto: test.go\n---\npackage test\n"
	createGenerator(t, tmpDir, "mygen", templateContent)

	cfg := createTestConfig(t, tmpDir)

	got, err := resolveGenerateTarget(cfg, "mygen", "_bundles")

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.False(t, got.IsBundle)
	assert.Contains(t, got.GenDir, "mygen")
	assert.Empty(t, got.BundlePath)
}

func TestUT_ResolveGenerateTarget_BundleOnly(t *testing.T) {
	tmpDir := t.TempDir()

	bundleJSON := `{"name":"mybundle","generators":[{"name":"gen1"}]}`
	createBundle(t, tmpDir, "mybundle", bundleJSON)

	cfg := createTestConfig(t, tmpDir)

	got, err := resolveGenerateTarget(cfg, "mybundle", "_bundles")

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.True(t, got.IsBundle)
	assert.Contains(t, got.BundlePath, "mybundle.json")
	assert.Empty(t, got.GenDir)
}

func TestUT_ResolveGenerateTarget_BothExist_GeneratorWins(t *testing.T) {
	tmpDir := t.TempDir()

	templateContent := "---\nto: test.go\n---\npackage test\n"
	createGenerator(t, tmpDir, "both", templateContent)

	bundleJSON := `{"name":"both","generators":[{"name":"gen1"}]}`
	createBundle(t, tmpDir, "both", bundleJSON)

	cfg := createTestConfig(t, tmpDir)

	got, err := resolveGenerateTarget(cfg, "both", "_bundles")

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.False(t, got.IsBundle, "generator should win when both exist")
	assert.Contains(t, got.GenDir, "both")
	assert.Empty(t, got.BundlePath)
}

func TestUT_ResolveGenerateTarget_NeitherExists(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := createTestConfig(t, tmpDir)

	got, err := resolveGenerateTarget(cfg, "nonexistent", "_bundles")

	require.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), `generator "nonexistent" not found`)
}
