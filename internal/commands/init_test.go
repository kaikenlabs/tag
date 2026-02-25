package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/types/flags"
)

func TestUT_InitAction_CreatesDirectories(t *testing.T) {
	tmpDir := setupTempDir(t)

	// Change to temp dir for config file creation
	oldDir, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Chdir(oldDir)
		os.Remove(config.File) // Clean up config file
	})

	ctx := createTestCLIContext(t, []string{}, map[string]any{
		flags.PathFlag:       ".tag",
		flags.SharedPathFlag: "_shared",
		flags.BundlePathFlag: "_bundles",
	})

	err = initAction(ctx)

	require.NoError(t, err)

	// Verify shared directory was created
	sharedDir := filepath.Join(tmpDir, ".tag", "_shared")
	info, err := os.Stat(sharedDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Verify bundle directory was created
	bundleDir := filepath.Join(tmpDir, ".tag", "_bundles")
	info, err = os.Stat(bundleDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestUT_InitAction_CreatesConfigFile(t *testing.T) {
	tmpDir := setupTempDir(t)

	oldDir, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Chdir(oldDir)
	})

	ctx := createTestCLIContext(t, []string{}, map[string]any{
		flags.PathFlag:       ".tag",
		flags.SharedPathFlag: "_shared",
		flags.BundlePathFlag: "_bundles",
	})

	err = initAction(ctx)

	require.NoError(t, err)

	// Verify config file was created
	configPath := filepath.Join(tmpDir, config.File)
	require.FileExists(t, configPath)

	// Verify config file can be loaded
	cfg, err := config.LoadConfigFile(".")
	require.NoError(t, err)
	assert.Equal(t, ".tag", cfg.Env.Path)
	assert.Equal(t, "_shared", cfg.Env.SharedPath)
}

func TestUT_InitAction_IdempotentExecution(t *testing.T) {
	tmpDir := setupTempDir(t)

	oldDir, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Chdir(oldDir)
	})

	ctx := createTestCLIContext(t, []string{}, map[string]any{
		flags.PathFlag:       ".tag",
		flags.SharedPathFlag: "_shared",
		flags.BundlePathFlag: "_bundles",
	})

	// First init
	err = initAction(ctx)
	require.NoError(t, err)

	// Second init should also succeed (idempotent)
	err = initAction(ctx)
	require.NoError(t, err)
}

func TestUT_InitAction_CustomPaths(t *testing.T) {
	tmpDir := setupTempDir(t)

	oldDir, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Chdir(oldDir)
	})

	ctx := createTestCLIContext(t, []string{}, map[string]any{
		flags.PathFlag:       "custom_templates",
		flags.SharedPathFlag: "custom_shared",
		flags.BundlePathFlag: "custom_bundles",
	})

	err = initAction(ctx)

	require.NoError(t, err)

	// Verify custom shared directory was created
	sharedDir := filepath.Join(tmpDir, "custom_templates", "custom_shared")
	info, err := os.Stat(sharedDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Verify custom bundle directory was created
	bundleDir := filepath.Join(tmpDir, "custom_templates", "custom_bundles")
	info, err = os.Stat(bundleDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestUT_InitCommand_ReturnsValidCommand(t *testing.T) {
	cmd := templateInitCommand()

	require.NotNil(t, cmd)
	assert.Equal(t, "init", cmd.Name)
	assert.NotEmpty(t, cmd.Usage)
	assert.NotEmpty(t, cmd.Description)
	assert.NotNil(t, cmd.Action)
}
