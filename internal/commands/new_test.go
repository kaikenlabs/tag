package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/types/flags"
	"github.com/kaikenlabs/tag/pkg/app"
)

func TestUT_NewAction_MissingGeneratorName(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{}, map[string]any{
		flags.PathFlag: tmpDir,
	})

	err := newAction(ctx, cfg)

	require.Error(t, err)
	var cmdErr *app.CommandError
	require.ErrorAs(t, err, &cmdErr)
	assert.Contains(t, err.Error(), "please provide the generator name")
}

func TestUT_NewAction_NoConfig(t *testing.T) {
	ctx := createTestCLIContext(t, []string{"myGenerator"}, nil)

	err := newAction(ctx, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "init")
}

func TestUT_NewAction_ValidGeneratorCreation(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"myGenerator"}, map[string]any{
		flags.PathFlag: tmpDir,
		"package":      "mypackage",
	})

	err := newAction(ctx, cfg)

	require.NoError(t, err)

	// Verify generator file was created
	generatorPath := filepath.Join(tmpDir, "myGenerator", "myGenerator.go")
	require.FileExists(t, generatorPath)

	// Verify content contains expected template structure
	data, err := os.ReadFile(generatorPath)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "---")
	assert.Contains(t, content, "to:")
	assert.Contains(t, content, "mypackage")
}

func TestUT_NewAction_CustomPackage(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"myGenerator"}, map[string]any{
		flags.PathFlag: tmpDir,
		"package":      "custompackage",
	})

	err := newAction(ctx, cfg)

	require.NoError(t, err)

	generatorPath := filepath.Join(tmpDir, "myGenerator", "myGenerator.go")
	data, err := os.ReadFile(generatorPath)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "custompackage")
}

func TestUT_NewAction_CreatesDirectory(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"newgenerator"}, map[string]any{
		flags.PathFlag: tmpDir,
	})

	// Ensure generator directory doesn't exist yet
	genDir := filepath.Join(tmpDir, "newgenerator")
	_, err := os.Stat(genDir)
	require.True(t, os.IsNotExist(err))

	err = newAction(ctx, cfg)
	require.NoError(t, err)

	// Verify directory was created
	info, err := os.Stat(genDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestUT_NewAction_TemplateContent(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"myGenerator"}, map[string]any{
		flags.PathFlag: tmpDir,
		"package":      "testpkg",
	})

	err := newAction(ctx, cfg)

	require.NoError(t, err)

	generatorPath := filepath.Join(tmpDir, "myGenerator", "myGenerator.go")
	data, err := os.ReadFile(generatorPath)
	require.NoError(t, err)

	content := string(data)

	// Verify template has metadata block
	assert.True(t, strings.HasPrefix(content, "---"), "should start with metadata delimiter")
	assert.Contains(t, content, "to:")

	// Verify template has package declaration
	assert.Contains(t, content, "package testpkg")

	// Verify template uses Gonja name variable with snake filter
	assert.Contains(t, content, "{{ name | snake }}")
}

func TestUT_NewAction_LibFlag_ValidCreation(t *testing.T) {
	templateDir := setupFakeLibrary(t, "my-template")
	tmpDir := setupTempDir(t)
	cfg := createTestConfigWithLib(t, tmpDir, "my-template")

	ctx := createTestCLIContext(t, []string{"myGenerator"}, map[string]any{
		flags.LibFlag: true,
		"package":     "mypackage",
	})

	err := newAction(ctx, cfg)

	require.NoError(t, err)

	// Verify generator was created inside the library template's .tag directory
	generatorPath := filepath.Join(templateDir, ".tag", "myGenerator", "myGenerator.go")
	require.FileExists(t, generatorPath)

	data, readErr := os.ReadFile(generatorPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "mypackage")
}

func TestUT_NewAction_LibFlag_NonExistentTemplate(t *testing.T) {
	setupFakeLibrary(t, "existing-template")
	tmpDir := setupTempDir(t)
	cfg := createTestConfigWithLib(t, tmpDir, "nonexistent")

	ctx := createTestCLIContext(t, []string{"myGenerator"}, map[string]any{
		flags.LibFlag: true,
		"package":     "mypackage",
	})

	err := newAction(ctx, cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUT_NewAction_LibFlag_NoTemplateOrigin(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"myGenerator"}, map[string]any{
		flags.LibFlag: true,
		"package":     "mypackage",
	})

	err := newAction(ctx, cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no library template configured")
}

func TestUT_NewAction_BundleFlag(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	// Create the bundle directory first (simulates tag new-bundle was run)
	bundleDir := filepath.Join(tmpDir, "_bundles", "mybundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0o750))

	ctx := createTestCLIContext(t, []string{"mygen"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.BundlePathFlag: "_bundles",
		flags.InBundleFlag:   "mybundle",
		"package":            "mypackage",
	})

	err := newAction(ctx, cfg)

	require.NoError(t, err)

	// Verify generator was created inside the bundle directory
	generatorPath := filepath.Join(bundleDir, "mygen", "mygen.go")
	require.FileExists(t, generatorPath)

	data, err := os.ReadFile(generatorPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "mypackage")
}

func TestUT_NewAction_BundleFlag_BundleNotFound(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"mygen"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.BundlePathFlag: "_bundles",
		flags.InBundleFlag:   "nonexistent",
		"package":            "mypackage",
	})

	err := newAction(ctx, cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
	assert.Contains(t, err.Error(), "tag new-bundle nonexistent")
}

func TestUT_NewCommand_ReturnsValidCommand(t *testing.T) {
	cfg := createTestConfig(t, ".tag")
	cmd := NewCommand(cfg)

	require.NotNil(t, cmd)
	assert.Equal(t, "new", cmd.Name)
	assert.NotEmpty(t, cmd.Usage)
	assert.NotNil(t, cmd.Action)
	assert.True(t, cmd.Args)
	assert.Equal(t, "<generator-name>", cmd.ArgsUsage)

	// Verify package flag exists
	var hasPackageFlag bool
	for _, f := range cmd.Flags {
		if sf, ok := f.(*cli.StringFlag); ok && sf.Name == "package" {
			hasPackageFlag = true
			assert.Equal(t, "mypackage", sf.Value)
			break
		}
	}
	assert.True(t, hasPackageFlag, "should have package flag")

	// Verify lib flag exists
	var hasLibFlag bool
	for _, f := range cmd.Flags {
		if bf, ok := f.(*cli.BoolFlag); ok && bf.Name == flags.LibFlag {
			hasLibFlag = true
			break
		}
	}
	assert.True(t, hasLibFlag, "should have lib flag")

	// Verify bundle flag exists
	var hasBundleFlag bool
	for _, f := range cmd.Flags {
		if sf, ok := f.(*cli.StringFlag); ok && sf.Name == flags.InBundleFlag {
			hasBundleFlag = true
			break
		}
	}
	assert.True(t, hasBundleFlag, "should have bundle flag")
}
