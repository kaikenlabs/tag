package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaikenlabs/tag/internal/types/flags"
	"github.com/kaikenlabs/tag/pkg/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestUT_NewAction_MissingGeneratorName(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{}, map[string]interface{}{
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

	ctx := createTestCLIContext(t, []string{"myGenerator"}, map[string]interface{}{
		flags.PathFlag: tmpDir,
		"package":      "mypackage",
	})

	err := newAction(ctx, cfg)

	require.NoError(t, err)

	// Verify generator file was created
	generatorPath := filepath.Join(tmpDir, "myGenerator", "myGenerator.tmpl")
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

	ctx := createTestCLIContext(t, []string{"myGenerator"}, map[string]interface{}{
		flags.PathFlag: tmpDir,
		"package":      "custompackage",
	})

	err := newAction(ctx, cfg)

	require.NoError(t, err)

	generatorPath := filepath.Join(tmpDir, "myGenerator", "myGenerator.tmpl")
	data, err := os.ReadFile(generatorPath)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "custompackage")
}

func TestUT_NewAction_CreatesDirectory(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"newgenerator"}, map[string]interface{}{
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

func TestUT_NewAction_UsesConfigExtension(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)
	cfg.Env.Extension = ".gotmpl" // Custom extension

	ctx := createTestCLIContext(t, []string{"myGenerator"}, map[string]interface{}{
		flags.PathFlag: tmpDir,
	})

	err := newAction(ctx, cfg)

	require.NoError(t, err)

	// Verify generator file uses custom extension
	generatorPath := filepath.Join(tmpDir, "myGenerator", "myGenerator.gotmpl")
	require.FileExists(t, generatorPath)
}

func TestUT_NewAction_TemplateContent(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"myGenerator"}, map[string]interface{}{
		flags.PathFlag: tmpDir,
		"package":      "testpkg",
	})

	err := newAction(ctx, cfg)

	require.NoError(t, err)

	generatorPath := filepath.Join(tmpDir, "myGenerator", "myGenerator.tmpl")
	data, err := os.ReadFile(generatorPath)
	require.NoError(t, err)

	content := string(data)

	// Verify template has metadata block
	assert.True(t, strings.HasPrefix(content, "---"), "should start with metadata delimiter")
	assert.Contains(t, content, "to:")

	// Verify template has package declaration
	assert.Contains(t, content, "package testpkg")

	// Verify template uses .Name template variable
	assert.Contains(t, content, "{{ .Name")
}

func TestUT_NewCommand_ReturnsValidCommand(t *testing.T) {
	cfg := createTestConfig(t, "_templates")
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
}
