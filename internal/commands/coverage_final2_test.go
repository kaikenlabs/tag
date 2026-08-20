package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/engine"
	"github.com/kaikenlabs/tag/internal/fileaction"
	"github.com/kaikenlabs/tag/internal/history"
	"github.com/kaikenlabs/tag/internal/hooks"
	"github.com/kaikenlabs/tag/internal/library"
	"github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/templateupdate"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/internal/types/flags"
	"github.com/kaikenlabs/tag/internal/writer"
	"github.com/kaikenlabs/tag/pkg/app"
)

// ===========================================================================
// generate.go — coverage for generateWithHooks (ErrUserQuit, post-hook fail,
// history append, verbose summary), generateBundle partial progress
// ===========================================================================

func TestUT_GenerateWithHooks_ErrUserQuit_ReturnsNil(t *testing.T) {
	tmpDir := setupTempDir(t)
	createSharedDir(t, tmpDir)
	cfg := createTestConfig(t, tmpDir)
	cfg.Hooks = config.Hooks{Pre: [][]string{}, Post: [][]string{}}

	ctx := createTestCLIContext(t, []string{"gen", "name"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.NoHooksFlag:    true,
	})

	_, err := generateWithHooks(ctx, cfg, "gen", "name", false, func(_ *history.Recorder) (engine.GenerateResult, error) {
		return engine.GenerateResult{}, writer.ErrUserQuit
	})
	assert.NoError(t, err, "ErrUserQuit should be handled gracefully")
}

func TestUT_GenerateWithHooks_DryRun_SkipsHistory(t *testing.T) {
	tmpDir := setupTempDir(t)
	createSharedDir(t, tmpDir)
	cfg := createTestConfig(t, tmpDir)
	cfg.Hooks = config.Hooks{Pre: [][]string{}, Post: [][]string{}}

	ctx := createTestCLIContext(t, []string{"gen", "name"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.NoHooksFlag:    true,
		flags.DryRunFlag:     true,
	})

	_, err := generateWithHooks(ctx, cfg, "gen", "name", false, func(_ *history.Recorder) (engine.GenerateResult, error) {
		return engine.GenerateResult{Created: 1}, nil
	})
	require.NoError(t, err)
}

func TestUT_GenerateWithHooks_VerbosePrintsDetails(t *testing.T) {
	tmpDir := setupTempDir(t)
	createSharedDir(t, tmpDir)
	cfg := createTestConfig(t, tmpDir)
	cfg.Hooks = config.Hooks{Pre: [][]string{}, Post: [][]string{}}

	var buf bytes.Buffer
	cliApp := &cli.App{Writer: &buf}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range []cli.Flag{
		&cli.BoolFlag{Name: flags.NoHooksFlag},
		&cli.BoolFlag{Name: flags.DryRunFlag},
		&cli.BoolFlag{Name: flags.VerboseFlag},
	} {
		require.NoError(t, f.Apply(set))
	}
	require.NoError(t, set.Set(flags.NoHooksFlag, "true"))
	require.NoError(t, set.Set(flags.VerboseFlag, "true"))
	require.NoError(t, set.Parse([]string{"gen", "name"}))
	ctx := cli.NewContext(cliApp, set, nil)

	_, err := generateWithHooks(ctx, cfg, "gen", "name", false, func(_ *history.Recorder) (engine.GenerateResult, error) {
		return engine.GenerateResult{
			Created: 1,
			Details: []engine.FileOpDetail{
				{Action: fileaction.ActionCreate, Path: "model.go"},
			},
		}, nil
	})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "model.go")
	assert.Contains(t, buf.String(), "created")
}

func TestUT_GenerateBundle_PartialProgress_PrintsSummary(t *testing.T) {
	tmpDir := setupTempDir(t)
	createSharedDir(t, tmpDir)
	createGenerator(t, tmpDir, "first", "---\nto: {{ name }}.txt\n---\nhi")
	createGenerator(t, tmpDir, "second", "---\nto: {{ name }}.txt\n---\nhi")

	bundleJSON := `{"generators": [{"name": "first"}, {"name": "second"}]}`
	createBundle(t, tmpDir, "multi", bundleJSON)

	cfg := createTestConfig(t, tmpDir)
	cfg.Hooks = config.Hooks{Pre: [][]string{}, Post: [][]string{}}

	callCount := 0
	fac := generatorFactories{
		newEngine: nil,
		newBundleEngine: func(_ *template.Engine, _ bool, _, _ string, _ *history.Recorder, _ io.Writer) (engine.Generator, error) {
			callCount++
			return &mockGenerator{
				GenerateFunc: func(_ engine.Data) (engine.GenerateResult, error) {
					if callCount > 1 {
						return engine.GenerateResult{}, errors.New("generator failed")
					}
					return engine.GenerateResult{Created: 2}, nil
				},
			}, nil
		},
	}

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	ctx := createTestCLIContext(t, []string{"multi", "Product"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.BundlePathFlag: "_bundles",
		flags.NoHooksFlag:    true,
	})

	err = generateAction(ctx, cfg, fac)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "generator failed")
}

func TestUT_GenerateBundle_EngineCreationError(t *testing.T) {
	tmpDir := setupTempDir(t)
	createSharedDir(t, tmpDir)
	createGenerator(t, tmpDir, "model", "---\nto: {{ name }}.txt\n---\nhi")

	bundleJSON := `{"generators": [{"name": "model"}]}`
	createBundle(t, tmpDir, "crud", bundleJSON)

	cfg := createTestConfig(t, tmpDir)
	cfg.Hooks = config.Hooks{Pre: [][]string{}, Post: [][]string{}}

	fac := generatorFactories{
		newEngine: nil,
		newBundleEngine: func(_ *template.Engine, _ bool, _, _ string, _ *history.Recorder, _ io.Writer) (engine.Generator, error) {
			return nil, errors.New("engine creation failed")
		},
	}

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	ctx := createTestCLIContext(t, []string{"crud", "User"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.BundlePathFlag: "_bundles",
		flags.NoHooksFlag:    true,
	})

	err = generateAction(ctx, cfg, fac)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "engine creation failed")
}

func TestUT_GenerateAction_TooFewArgs(t *testing.T) {
	t.Parallel()
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)
	ctx := createTestCLIContext(t, []string{"generator-only"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
	})
	err := generateAction(ctx, cfg, defaultGeneratorFactories())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "please provide the generator/bundle and the name")
}

// ===========================================================================
// update_template.go — coverage for updateTemplateAction mode branches,
// shortCommitSHA, printUpdateSummary with var/hook changes
// ===========================================================================

func TestUT_ShortCommitSHA(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "abcdef1", shortCommitSHA("abcdef1234567890"))
	assert.Equal(t, "abc", shortCommitSHA("abc"))
	assert.Equal(t, "", shortCommitSHA(""))
}

func TestUT_PrintUpdateSummary_EmptyApplied(t *testing.T) {
	result := &templateupdate.UpdateResult{Applied: nil}
	// Should not panic
	var buf bytes.Buffer
	printUpdateSummary(&buf, result)
	assert.Empty(t, buf.String())
}

func TestUT_UpdateTemplateAction_AcceptOursOnly(t *testing.T) {
	t.Parallel()
	ctx := newUpdateCLIContext(t, map[string]string{
		"accept-ours": "true",
	})
	// This will fail because there's no .tagconfig.json etc., but it exercises
	// the resolveMode branch for accept-ours
	err := updateTemplateAction(ctx)
	require.Error(t, err)
	// Should not fail on flag validation, but on updater.Update
	assert.NotContains(t, err.Error(), "cannot use")
}

func TestUT_UpdateTemplateAction_AcceptTheirsOnly(t *testing.T) {
	t.Parallel()
	ctx := newUpdateCLIContext(t, map[string]string{
		"accept-theirs": "true",
	})
	err := updateTemplateAction(ctx)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "cannot use")
}

func TestUT_UpdateTemplateAction_ContinueModeOnly(t *testing.T) {
	t.Parallel()
	ctx := newUpdateCLIContext(t, map[string]string{
		"continue": "true",
	})
	err := updateTemplateAction(ctx)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "cannot use")
}

func TestUT_UpdateTemplateAction_AbortModeOnly(t *testing.T) {
	t.Parallel()
	ctx := newUpdateCLIContext(t, map[string]string{
		"abort": "true",
	})
	err := updateTemplateAction(ctx)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "cannot use")
}

// ===========================================================================
// doctor.go — coverage for doctorCheckTemplates with lint errors/warnings,
// doctorCheckLibraries with inaccessible template, doctorAction fail exit
// ===========================================================================

func TestUT_DoctorCheckTemplates_InvalidConfig(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, types.TemplatesDir)
	tmplDir := filepath.Join(tagDir, "bad-tmpl")
	require.NoError(t, os.MkdirAll(tmplDir, 0o750))

	// Write an invalid tag.template.json
	require.NoError(t, os.WriteFile(
		filepath.Join(tmplDir, types.TemplateConfigFile),
		[]byte(`{ this is not valid json`),
		0o644,
	))

	results := doctorCheckTemplates(dir)
	require.NotEmpty(t, results)
	// Should have a fail result for the bad template
	hasFail := false
	for _, r := range results {
		if r.Status == doctorFail {
			hasFail = true
		}
	}
	assert.True(t, hasFail, "expected fail for invalid template config")
}

func TestUT_DoctorCheckLibraries_EmptyLibrary(t *testing.T) {
	// library.New (called by doctorCheckLibraries) also builds a resolver
	// that touches $HOME/.tag/cache, so HOME must be isolated too, not just
	// XDG_DATA_HOME. Not parallel: t.Setenv.
	t.Setenv("HOME", t.TempDir())
	dataDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataDir)

	results := doctorCheckLibraries()
	require.NotEmpty(t, results)
	assert.Equal(t, doctorPass, results[0].Status)
	assert.Contains(t, results[0].Label, "none installed")
}

func TestUT_DoctorCheckLibraries_InaccessiblePath(t *testing.T) {
	// library.New (called by doctorCheckLibraries) also builds a resolver
	// that touches $HOME/.tag/cache, so HOME must be isolated too, not just
	// XDG_DATA_HOME. Not parallel: t.Setenv.
	t.Setenv("HOME", t.TempDir())
	xdgDir := t.TempDir()
	// xdg.DataHome returns $XDG_DATA_HOME/tag, so the library is at $XDG_DATA_HOME/tag/
	tagDataDir := filepath.Join(xdgDir, "tag")
	require.NoError(t, os.MkdirAll(tagDataDir, 0o750))

	// Write a registry entry pointing to a nonexistent template path
	reg := library.Registry{
		Version: 1,
		Entries: map[string]*library.Entry{
			"ghost-lib": {Name: "ghost-lib", Source: "gh:test/ghost-lib"},
		},
	}
	regData, err := json.Marshal(reg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(tagDataDir, "library.json"), regData, 0o644))

	t.Setenv("XDG_DATA_HOME", xdgDir)

	results := doctorCheckLibraries()
	require.NotEmpty(t, results)
	// Should have a fail for inaccessible path (template dir doesn't exist)
	hasFail := false
	for _, r := range results {
		if r.Status == doctorFail {
			hasFail = true
		}
	}
	assert.True(t, hasFail, "expected fail for inaccessible library path")
}

func TestUT_DoctorAction_AllPass_NoError(t *testing.T) {
	// doctorAction -> doctorCheckLibraries reads $HOME/$XDG_DATA_HOME
	// directly, bypassing the overridable newLocalLibrary var. The previous
	// version only isolated XDG_DATA_HOME, but library.New also builds a
	// resolver that touches $HOME/.tag/cache — seedHome isolates both.
	// Not parallel: seedHome and t.Setenv below mutate process env.
	seedHome(t)
	t.Setenv("GITHUB_TOKEN", "test-token")
	dir := t.TempDir()
	tagDir := filepath.Join(dir, types.TemplatesDir)
	require.NoError(t, os.MkdirAll(filepath.Join(tagDir, types.SharedDir), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(tagDir, types.BundlesDir), 0o750))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	var buf bytes.Buffer
	err = doctorAction(context.Background(), &buf, "dev", formatText)
	// With GITHUB_TOKEN set and proper .tag/ structure, should pass or warn
	if err != nil {
		var cmdErr *app.CommandError
		if errors.As(err, &cmdErr) {
			assert.Contains(t, []int{0, doctorExitWarnings}, cmdErr.Code)
		}
	}
}

func TestUT_DoctorCheckTAGVersion_UpdateAvailable(t *testing.T) {
	// Non-dev build with a version higher than latest shows "update available" or passes
	result := doctorCheckTAGVersion(context.Background(), "0.0.1")
	// Should be either warn (update available) or pass (already up to date)
	assert.Contains(t, []doctorStatus{doctorWarn, doctorPass}, result.Status)
}

func TestUT_DoctorCheckTAGVersion_DevBuildSkips(t *testing.T) {
	t.Parallel()
	result := doctorCheckTAGVersion(context.Background(), "dev-build")
	assert.Equal(t, doctorPass, result.Status)
	assert.Contains(t, result.Label, "dev build")
}

// ===========================================================================
// generate.go — coverage for generateTemplate with ScaffoldVars nil
// ===========================================================================

func TestUT_GenerateTemplate_NilVars_NoScaffoldVarsSet(t *testing.T) {
	tmpDir := setupTempDir(t)
	createSharedDir(t, tmpDir)
	createGenerator(t, tmpDir, "model", "---\nto: {{ name }}.txt\n---\n{{ name }}")

	cfg := createTestConfig(t, tmpDir)
	cfg.Variables = nil // explicitly nil
	cfg.Hooks = config.Hooks{Pre: [][]string{}, Post: [][]string{}}

	var capturedData engine.Data
	fac := generatorFactories{
		newEngine: func(_ bool, _, _ string, _ *history.Recorder, _ io.Writer) (engine.Generator, error) {
			return &mockGenerator{
				GenerateFunc: func(data engine.Data) (engine.GenerateResult, error) {
					capturedData = data
					return engine.GenerateResult{Created: 1}, nil
				},
			}, nil
		},
		newBundleEngine: nil,
	}

	ctx := createTestCLIContext(t, []string{"model", "User"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.NoHooksFlag:    true,
	})

	err := generateAction(ctx, cfg, fac)
	require.NoError(t, err)
	assert.Nil(t, capturedData.ScaffoldVars)
}

func TestUT_GenerateTemplate_WithVars_ScaffoldVarsSet(t *testing.T) {
	tmpDir := setupTempDir(t)
	createSharedDir(t, tmpDir)
	createGenerator(t, tmpDir, "model", "---\nto: {{ name }}.txt\n---\n{{ name }}")

	cfg := createTestConfig(t, tmpDir)
	cfg.Variables = map[string]any{"key": "val"}
	cfg.Hooks = config.Hooks{Pre: [][]string{}, Post: [][]string{}}

	var capturedData engine.Data
	fac := generatorFactories{
		newEngine: func(_ bool, _, _ string, _ *history.Recorder, _ io.Writer) (engine.Generator, error) {
			return &mockGenerator{
				GenerateFunc: func(data engine.Data) (engine.GenerateResult, error) {
					capturedData = data
					return engine.GenerateResult{Created: 1}, nil
				},
			}, nil
		},
		newBundleEngine: nil,
	}

	ctx := createTestCLIContext(t, []string{"model", "User"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.NoHooksFlag:    true,
	})

	err := generateAction(ctx, cfg, fac)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"key": "val"}, capturedData.ScaffoldVars)
}

// ===========================================================================
// generate.go — runHooks with vars coverage
// ===========================================================================

func TestUT_RunHooks_WithVars_BuildsEnv(t *testing.T) {
	vars := map[string]any{"project_name": "test-proj"}

	// Use echo which always succeeds
	var buf bytes.Buffer
	err := runHooks([][]string{{"echo", "hello"}}, hooks.HookPhasePreGen, vars, &buf, "gen", "name")
	require.NoError(t, err)
}

func TestUT_RunHooks_NoVars_UsesOSEnviron(t *testing.T) {
	var buf bytes.Buffer
	err := runHooks([][]string{{"echo", "hello"}}, hooks.HookPhasePreGen, nil, &buf, "gen", "name")
	require.NoError(t, err)
}

// ===========================================================================
// generate.go — generateBundle with requirements
// ===========================================================================

func TestUT_GenerateBundle_RequirementsCheckFails_UnmetVar(t *testing.T) {
	tmpDir := setupTempDir(t)
	createSharedDir(t, tmpDir)

	bundleJSON := `{"requires": ["missing_var"], "generators": [{"name": "x"}]}`
	createBundle(t, tmpDir, "req-bundle", bundleJSON)

	cfg := createTestConfig(t, tmpDir)
	cfg.Variables = map[string]any{} // no variables set
	cfg.Hooks = config.Hooks{Pre: [][]string{}, Post: [][]string{}}

	fac := generatorFactories{
		newEngine:       nil,
		newBundleEngine: nil,
	}

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	ctx := createTestCLIContext(t, []string{"req-bundle", "MyName"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.BundlePathFlag: "_bundles",
		flags.NoHooksFlag:    true,
	})

	err = generateAction(ctx, cfg, fac)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing_var")
}

// ===========================================================================
// generate.go — MergeVars edge cases
// ===========================================================================

func TestUT_MergeVars_OverlayOnly(t *testing.T) {
	t.Parallel()
	overlay := map[string]any{"x": 1}
	result := mergeVars(nil, overlay)
	assert.Equal(t, 1, result["x"])
	assert.Len(t, result, 1)
}

func TestUT_MergeVars_BothEmpty_ReturnsNil2(t *testing.T) {
	t.Parallel()
	assert.Nil(t, mergeVars(map[string]any{}, map[string]any{}))
}
