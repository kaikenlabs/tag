package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/engine"
	"github.com/kaikenlabs/tag/internal/types/flags"
	"github.com/kaikenlabs/tag/pkg/app"
)

func TestUT_BundleAction_MissingName(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.BundlePathFlag: "_bundles",
	})

	err := bundleAction(ctx, cfg)

	require.Error(t, err)
	var cmdErr *app.CommandError
	require.ErrorAs(t, err, &cmdErr)
	assert.Contains(t, err.Error(), "please provide the bundle name")
}

func TestUT_BundleAction_NoConfig(t *testing.T) {
	ctx := createTestCLIContext(t, []string{"mybundle"}, nil)

	err := bundleAction(ctx, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "init")
}

func TestUT_BundleAction_ValidBundleCreation(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"mybundle"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.BundlePathFlag: "_bundles",
	})

	err := bundleAction(ctx, cfg)

	require.NoError(t, err)

	// Verify bundle file was created
	bundlePath := filepath.Join(tmpDir, "_bundles", "mybundle", "mybundle.json")
	require.FileExists(t, bundlePath)

	// Verify content
	data, err := os.ReadFile(bundlePath)
	require.NoError(t, err)

	var bundle engine.Bundle
	err = json.Unmarshal(data, &bundle)
	require.NoError(t, err)

	assert.Equal(t, "mybundle", bundle.Name)
	assert.Len(t, bundle.Generators, 1)
	assert.Equal(t, "myGenerator", bundle.Generators[0].Name)
}

func TestUT_BundleAction_CreatesDirectory(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"newbundle"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.BundlePathFlag: "_bundles",
	})

	// Ensure bundle directory doesn't exist yet
	bundleDir := filepath.Join(tmpDir, "_bundles", "newbundle")
	_, err := os.Stat(bundleDir)
	require.True(t, os.IsNotExist(err))

	err = bundleAction(ctx, cfg)
	require.NoError(t, err)

	// Verify directory was created
	info, err := os.Stat(bundleDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestUT_BundleAction_LibFlag_ValidCreation(t *testing.T) {
	templateDir := setupFakeLibrary(t, "my-template")
	tmpDir := setupTempDir(t)
	cfg := createTestConfigWithLib(t, tmpDir, "my-template")

	ctx := createTestCLIContext(t, []string{"mybundle"}, map[string]any{
		flags.LibFlag: true,
	})

	err := bundleAction(ctx, cfg)

	require.NoError(t, err)

	// Verify bundle was created inside the library template's .tag/_bundles directory
	bundlePath := filepath.Join(templateDir, ".tag", "_bundles", "mybundle", "mybundle.json")
	require.FileExists(t, bundlePath)

	data, readErr := os.ReadFile(bundlePath)
	require.NoError(t, readErr)

	var bundle engine.Bundle
	require.NoError(t, json.Unmarshal(data, &bundle))
	assert.Equal(t, "mybundle", bundle.Name)
}

func TestUT_BundleAction_LibFlag_NonExistentTemplate(t *testing.T) {
	setupFakeLibrary(t, "existing-template")
	tmpDir := setupTempDir(t)
	cfg := createTestConfigWithLib(t, tmpDir, "nonexistent")

	ctx := createTestCLIContext(t, []string{"mybundle"}, map[string]any{
		flags.LibFlag: true,
	})

	err := bundleAction(ctx, cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUT_BundleAction_LibFlag_NoTemplateOrigin(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"mybundle"}, map[string]any{
		flags.LibFlag: true,
	})

	err := bundleAction(ctx, cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no library template configured")
}

func TestUT_BundleCommand_ReturnsValidCommand(t *testing.T) {
	cfg := createTestConfig(t, ".tag")
	cmd := templateNewBundleCommand(cfg)

	require.NotNil(t, cmd)
	assert.Equal(t, "bundle", cmd.Name)
	assert.NotEmpty(t, cmd.Usage)
	assert.NotEmpty(t, cmd.Description)
	assert.NotNil(t, cmd.Action)
	assert.True(t, cmd.Args)
	assert.Equal(t, "<bundle-name>", cmd.ArgsUsage)

	// Verify lib flag exists
	var hasLibFlag bool
	for _, f := range cmd.Flags {
		if bf, ok := f.(*cli.BoolFlag); ok && bf.Name == flags.LibFlag {
			hasLibFlag = true
			break
		}
	}
	assert.True(t, hasLibFlag, "should have lib flag")

	// Verify self-contained flag exists
	var hasSelfContainedFlag bool
	for _, f := range cmd.Flags {
		if bf, ok := f.(*cli.BoolFlag); ok && bf.Name == flags.SelfContainedFlag {
			hasSelfContainedFlag = true
			break
		}
	}
	assert.True(t, hasSelfContainedFlag, "should have self-contained flag")
}

func TestUT_GetBundleTemplate(t *testing.T) {
	data, err := getBundleTemplate("testbundle", false)

	require.NoError(t, err)
	require.NotNil(t, data)

	var bundle engine.Bundle
	err = json.Unmarshal(data, &bundle)
	require.NoError(t, err)

	assert.Equal(t, "testbundle", bundle.Name)
	assert.False(t, bundle.SelfContained)
	assert.Len(t, bundle.Generators, 1)
	assert.Equal(t, "myGenerator", bundle.Generators[0].Name)
}

func TestUT_GetBundleTemplate_SelfContained(t *testing.T) {
	data, err := getBundleTemplate("testbundle", true)

	require.NoError(t, err)
	require.NotNil(t, data)

	var bundle engine.Bundle
	err = json.Unmarshal(data, &bundle)
	require.NoError(t, err)

	assert.Equal(t, "testbundle", bundle.Name)
	assert.True(t, bundle.SelfContained)
}

func TestUT_BundleAction_SelfContainedFlag(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"mybundle"}, map[string]any{
		flags.PathFlag:          tmpDir,
		flags.BundlePathFlag:    "_bundles",
		flags.SelfContainedFlag: true,
	})

	err := bundleAction(ctx, cfg)

	require.NoError(t, err)

	// Verify bundle file was created
	bundlePath := filepath.Join(tmpDir, "_bundles", "mybundle", "mybundle.json")
	require.FileExists(t, bundlePath)

	// Verify content has self_contained: true
	data, err := os.ReadFile(bundlePath)
	require.NoError(t, err)

	var bundle engine.Bundle
	err = json.Unmarshal(data, &bundle)
	require.NoError(t, err)

	assert.Equal(t, "mybundle", bundle.Name)
	assert.True(t, bundle.SelfContained)
}
