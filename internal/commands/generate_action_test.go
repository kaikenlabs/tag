package commands

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/engine"
	"github.com/kaikenlabs/tag/internal/history"
	"github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/internal/types/flags"
	"github.com/kaikenlabs/tag/pkg/app"
)

func TestUT_GenerateAction_InvalidGeneratorName(t *testing.T) {
	t.Parallel()

	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)
	ctx := createTestCLIContext(t, []string{"../escape", "myName"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
	})

	err := generateAction(ctx, cfg, defaultGeneratorFactories())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid generator/bundle name")
}

func TestUT_GenerateAction_InvalidTargetName(t *testing.T) {
	t.Parallel()

	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)
	ctx := createTestCLIContext(t, []string{"hello", "../../badname"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
	})

	err := generateAction(ctx, cfg, defaultGeneratorFactories())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid target name")
}

func TestUT_GenerateAction_InvalidOnExisting(t *testing.T) {
	t.Parallel()

	tmpDir := setupTempDir(t)
	createGenerator(t, tmpDir, "hello", "---\nto: {{ name }}.txt\n---\nhi")
	createSharedDir(t, tmpDir)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"hello", "world"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.OnExistingFlag: "invalid-policy",
	})

	err := generateAction(ctx, cfg, defaultGeneratorFactories())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --on-existing value")
}

func TestUT_GenerateAction_BundleNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := setupTempDir(t)
	createSharedDir(t, tmpDir)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"nonexistent-bundle", "myName"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.BundlePathFlag: "_bundles",
	})

	err := generateAction(ctx, cfg, defaultGeneratorFactories())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUT_GenerateAction_ConfigValidationError(t *testing.T) {
	t.Parallel()

	cfg := createTestConfig(t, "/nonexistent/path")
	cfg.Env.Path = ""

	ctx := createTestCLIContext(t, []string{"gen", "name"}, nil)

	err := generateAction(ctx, cfg, defaultGeneratorFactories())
	require.Error(t, err)
}

func TestUT_GenerateAction_BundleExecution(t *testing.T) {
	tmpDir := setupTempDir(t)
	createSharedDir(t, tmpDir)

	createGenerator(t, tmpDir, "model", "---\nto: {{ name }}.txt\n---\n{{ name }}")

	bundleJSON := `{
  "generators": [
    {"name": "model"}
  ]
}`
	createBundle(t, tmpDir, "crud", bundleJSON)

	cfg := createTestConfig(t, tmpDir)
	cfg.Hooks = config.Hooks{Pre: [][]string{}, Post: [][]string{}}

	var generatedData []engine.Data
	fac := generatorFactories{
		newEngine: func(_ bool, _, _ string, _ *history.Recorder, _ io.Writer) (engine.Generator, error) {
			return &mockGenerator{
				GenerateFunc: func(data engine.Data) (engine.GenerateResult, error) {
					generatedData = append(generatedData, data)
					return engine.GenerateResult{Created: 1}, nil
				},
			}, nil
		},
		newBundleEngine: func(_ *template.Engine, _ bool, _, _ string, _ *history.Recorder, _ io.Writer) (engine.Generator, error) {
			return &mockGenerator{
				GenerateFunc: func(data engine.Data) (engine.GenerateResult, error) {
					generatedData = append(generatedData, data)
					return engine.GenerateResult{Created: 1}, nil
				},
			}, nil
		},
	}

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	ctx := createTestCLIContext(t, []string{"crud", "Product"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.BundlePathFlag: "_bundles",
		flags.NoHooksFlag:    true,
	})

	err = generateAction(ctx, cfg, fac)
	require.NoError(t, err)
	require.Len(t, generatedData, 1)
	assert.Equal(t, "Product", generatedData[0].Name)
}

func TestUT_WarnVersionMismatch_NoVersion(t *testing.T) {
	t.Parallel()

	cfg := createTestConfigWithLib(t, "/tmp", "test-tmpl")
	cfg.Template.Version = ""
	warnVersionMismatch(cfg, t.TempDir())
}

func TestUT_WarnVersionMismatch_MatchingVersion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := createTestConfigWithLib(t, "/tmp", "test-tmpl")
	cfg.Template.Version = "1.0.0"

	require.NoError(t, os.WriteFile(
		filepath.Join(dir, types.TemplateConfigFile),
		[]byte(`{"version": "1.0.0", "variables": []}`),
		0o644,
	))

	warnVersionMismatch(cfg, dir)
}

func TestUT_PrintGenerateSummary_NonVerbose(t *testing.T) {
	t.Parallel()

	result := engine.GenerateResult{
		Created: 3,
		Details: []engine.FileOpDetail{
			{Op: "created", Path: "a.go"},
			{Op: "created", Path: "b.go"},
			{Op: "created", Path: "c.go"},
		},
	}

	var buf bytes.Buffer
	printGenerateSummary(&buf, result, false)

	out := buf.String()
	assert.Contains(t, out, "3 created")
	assert.NotContains(t, out, "a.go")
}

func TestUT_RunGenerate_ConflictError(t *testing.T) {
	t.Parallel()

	mock := &mockGenerator{
		GenerateFunc: func(_ engine.Data) (engine.GenerateResult, error) {
			return engine.GenerateResult{}, &engine.ConflictError{Files: []string{"existing.go"}}
		},
	}

	_, err := runGenerate(mock, engine.Data{Name: "test"})
	require.Error(t, err)

	var cmdErr *app.CommandError
	require.True(t, errors.As(err, &cmdErr))
	assert.Contains(t, err.Error(), "existing.go")
}

func TestUT_RunGenerate_GenericError(t *testing.T) {
	t.Parallel()

	mock := &mockGenerator{
		GenerateFunc: func(_ engine.Data) (engine.GenerateResult, error) {
			return engine.GenerateResult{}, errors.New("unexpected failure")
		},
	}

	_, err := runGenerate(mock, engine.Data{Name: "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error when generating template")
}

func TestUT_MergeVars_BothEmpty(t *testing.T) {
	t.Parallel()
	assert.Nil(t, mergeVars(nil, nil))
}

func TestUT_MergeVars_OverlayTakesPrecedence(t *testing.T) {
	t.Parallel()

	base := map[string]any{"key": "base-val", "only-base": "x"}
	overlay := map[string]any{"key": "overlay-val", "only-overlay": "y"}

	result := mergeVars(base, overlay)
	assert.Equal(t, "overlay-val", result["key"])
	assert.Equal(t, "x", result["only-base"])
	assert.Equal(t, "y", result["only-overlay"])
}

func TestUT_GenerateAction_LibraryGenerator(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	templateDir := setupFakeLibrary(t, "my-tmpl")

	tagDir := filepath.Join(templateDir, types.TemplatesDir)
	genDir := filepath.Join(tagDir, "model")
	require.NoError(t, os.MkdirAll(genDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(genDir, "model.go"),
		[]byte("---\nto: {{ name }}.go\n---\npackage main"),
		0o644,
	))
	require.NoError(t, os.MkdirAll(filepath.Join(tagDir, types.SharedDir), 0o750))

	tmpDir := setupTempDir(t)
	cfg := createTestConfigWithLib(t, tmpDir, "my-tmpl")

	var capturedData engine.Data
	fac := defaultGeneratorFactories()
	fac.newEngine = func(_ bool, _ string, _ string, _ *history.Recorder, _ io.Writer) (engine.Generator, error) {
		return &mockGenerator{
			GenerateFunc: func(data engine.Data) (engine.GenerateResult, error) {
				capturedData = data
				return engine.GenerateResult{Created: 1}, nil
			},
		}, nil
	}

	ctx := createTestCLIContext(t, []string{"model", "User"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.NoHooksFlag:    true,
	})

	err := generateAction(ctx, cfg, fac)
	require.NoError(t, err)
	assert.Equal(t, "User", capturedData.Name)
}
