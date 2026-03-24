package commands

import (
	"bytes"
	"context"
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
	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/templatetest"
	"github.com/kaikenlabs/tag/internal/templateupdate"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/pkg/app"
)

// ===========================================================================
// scaffold.go — coverage for resolveAddToLib, hasSubdirScaffold,
// displayScaffoldSummary (generators branch), resolveTemplateName variants,
// handleCookiecutterDetection
// ===========================================================================

func TestUT_ResolveAddToLib_AddToLibFlagTrue(t *testing.T) {
	t.Parallel()

	ctx := newScaffoldActionCLIContext(t, nil, map[string]string{
		"add-to-lib": "true",
	})

	assert.True(t, resolveAddToLib(ctx, t.TempDir()))
}

func TestUT_ResolveAddToLib_NoGenerators(t *testing.T) {
	t.Parallel()

	ctx := newScaffoldActionCLIContext(t, nil, map[string]string{
		"no-input": "true",
	})

	// A dir with no _generators or .tag subdirs → false
	assert.False(t, resolveAddToLib(ctx, t.TempDir()))
}

func TestUT_ResolveAddToLib_NoInput_WithGenerators(t *testing.T) {
	t.Parallel()

	templateDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(templateDir, types.GeneratorsDir), 0o755))

	ctx := newScaffoldActionCLIContext(t, nil, map[string]string{
		"no-input": "true",
	})

	// Has generators but --no-input → false (safe default)
	assert.False(t, resolveAddToLib(ctx, templateDir))
}

func TestUT_HasSubdirScaffold_ExistingDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sub := "somedir"
	require.NoError(t, os.MkdirAll(filepath.Join(dir, sub), 0o755))

	assert.True(t, hasSubdirScaffold(dir, sub))
}

func TestUT_HasSubdirScaffold_NotExist(t *testing.T) {
	t.Parallel()
	assert.False(t, hasSubdirScaffold(t.TempDir(), "nonexistent"))
}

func TestUT_HasSubdirScaffold_FileIsNotDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "afile"), []byte("x"), 0o644))

	assert.False(t, hasSubdirScaffold(dir, "afile"))
}

func TestUT_DisplayScaffoldSummary_WithGenerators(t *testing.T) {
	t.Parallel()

	templateDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(templateDir, types.GeneratorsDir), 0o755))

	result := scaffold.ScaffoldResult{
		OutputDir:   "/tmp/proj",
		TemplateDir: templateDir,
		Vars:        map[string]any{},
		Opts: scaffold.Options{
			TemplateName: "my-tmpl",
			TemplateRef:  "gh:test/my-tmpl",
		},
	}

	var buf bytes.Buffer
	displayScaffoldSummary(&buf, result)

	out := buf.String()
	assert.Contains(t, out, "tag generate list")
	assert.Contains(t, out, "Template: gh:test/my-tmpl")
}

func TestUT_DisplayScaffoldSummary_NoVersion(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	result := scaffold.ScaffoldResult{
		OutputDir:   "/tmp/proj",
		TemplateDir: t.TempDir(),
		Vars:        map[string]any{},
		Opts: scaffold.Options{
			TemplateName: "test",
			TemplateRef:  "gh:test/repo",
		},
	}

	displayScaffoldSummary(&buf, result)
	out := buf.String()
	assert.Contains(t, out, "Template: gh:test/repo")
	assert.NotContains(t, out, "()")
}

func TestUT_ResolveTemplateName_FromPositionalArgs(t *testing.T) {
	t.Parallel()

	ctx := newScaffoldActionCLIContext(t, nil, nil)
	name, err := resolveTemplateName(ctx, nil, []string{"my-tmpl"})
	require.NoError(t, err)
	assert.Equal(t, "my-tmpl", name)
}

func TestUT_ResolveTemplateName_NoArgsNoTTY(t *testing.T) {
	t.Parallel()

	ctx := newScaffoldActionCLIContext(t, nil, map[string]string{
		"no-input": "true",
	})

	_, err := resolveTemplateName(ctx, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "template argument required")
}

// ===========================================================================
// library.go — coverage for editorSource, resolveEditor, splitEditorArgs,
// truncate, libSearchCommand, runLibSearch
// ===========================================================================

func TestUT_EditorSource_FlagValue(t *testing.T) {
	t.Parallel()

	s := &editorSource{}
	editor, err := s.resolve("vim")
	require.NoError(t, err)
	assert.Equal(t, "vim", editor)
}

func TestUT_EditorSource_ConfigValue(t *testing.T) {
	t.Parallel()

	s := &editorSource{
		loadConfig: func() (*config.GlobalConfig, error) {
			return &config.GlobalConfig{Editor: "code"}, nil
		},
		getenv: func(_ string) string { return "" },
	}

	editor, err := s.resolve("")
	require.NoError(t, err)
	assert.Equal(t, "code", editor)
}

func TestUT_EditorSource_VisualEnv(t *testing.T) {
	t.Parallel()

	s := &editorSource{
		loadConfig: func() (*config.GlobalConfig, error) {
			return &config.GlobalConfig{}, nil
		},
		getenv: func(key string) string {
			if key == "VISUAL" {
				return "nano"
			}
			return ""
		},
	}

	editor, err := s.resolve("")
	require.NoError(t, err)
	assert.Equal(t, "nano", editor)
}

func TestUT_EditorSource_EditorEnv(t *testing.T) {
	t.Parallel()

	s := &editorSource{
		loadConfig: func() (*config.GlobalConfig, error) {
			return &config.GlobalConfig{}, nil
		},
		getenv: func(key string) string {
			if key == "EDITOR" {
				return "vi"
			}
			return ""
		},
	}

	editor, err := s.resolve("")
	require.NoError(t, err)
	assert.Equal(t, "vi", editor)
}

func TestUT_EditorSource_NoTTY_ReturnsError(t *testing.T) {
	t.Parallel()

	s := &editorSource{
		loadConfig: func() (*config.GlobalConfig, error) {
			return &config.GlobalConfig{}, nil
		},
		getenv: func(_ string) string { return "" },
		isTTY:  func() bool { return false },
	}

	_, err := s.resolve("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no editor configured")
}

func TestUT_EditorSource_TTY_PromptEmpty(t *testing.T) {
	t.Parallel()

	s := &editorSource{
		loadConfig: func() (*config.GlobalConfig, error) {
			return &config.GlobalConfig{}, nil
		},
		getenv: func(_ string) string { return "" },
		isTTY:  func() bool { return true },
		prompt: func() (string, error) { return "", nil },
	}

	_, err := s.resolve("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no editor configured")
}

func TestUT_EditorSource_TTY_PromptSuccess(t *testing.T) {
	t.Parallel()

	saved := false
	s := &editorSource{
		loadConfig: func() (*config.GlobalConfig, error) {
			return &config.GlobalConfig{}, nil
		},
		saveConfig: func(_ *config.GlobalConfig) error {
			saved = true
			return nil
		},
		getenv: func(_ string) string { return "" },
		isTTY:  func() bool { return true },
		prompt: func() (string, error) { return "code", nil },
	}

	editor, err := s.resolve("")
	require.NoError(t, err)
	assert.Equal(t, "code", editor)
	assert.True(t, saved)
}

func TestUT_EditorSource_ConfigLoadError_Continues(t *testing.T) {
	t.Parallel()

	s := &editorSource{
		loadConfig: func() (*config.GlobalConfig, error) {
			return nil, errors.New("cannot read config")
		},
		getenv: func(key string) string {
			if key == "EDITOR" {
				return "vim"
			}
			return ""
		},
	}

	editor, err := s.resolve("")
	require.NoError(t, err)
	assert.Equal(t, "vim", editor)
}

func TestUT_SplitEditorArgs_Simple(t *testing.T) {
	t.Parallel()

	args, err := splitEditorArgs("code --wait")
	require.NoError(t, err)
	assert.Equal(t, []string{"code", "--wait"}, args)
}

func TestUT_SplitEditorArgs_Quoted(t *testing.T) {
	t.Parallel()

	args, err := splitEditorArgs(`"/path/to/my editor" --wait`)
	require.NoError(t, err)
	assert.Equal(t, []string{"/path/to/my editor", "--wait"}, args)
}

func TestUT_SaveEditorPreference_SaveError(t *testing.T) {
	t.Parallel()

	s := &editorSource{
		loadConfig: func() (*config.GlobalConfig, error) {
			return &config.GlobalConfig{}, nil
		},
		saveConfig: func(_ *config.GlobalConfig) error {
			return errors.New("disk full")
		},
	}

	// Should not panic — just prints warning to stderr
	s.saveEditorPreference("vim")
}

func TestUT_SaveEditorPreference_LoadError_StillSaves(t *testing.T) {
	t.Parallel()

	saved := false
	s := &editorSource{
		loadConfig: func() (*config.GlobalConfig, error) {
			return nil, errors.New("no file")
		},
		saveConfig: func(cfg *config.GlobalConfig) error {
			saved = true
			assert.Equal(t, "vim", cfg.Editor)
			return nil
		},
	}

	s.saveEditorPreference("vim")
	assert.True(t, saved)
}

// ===========================================================================
// update_template.go — coverage for parseSetFlags, printUpdateSummary,
// updateTemplateAction flag validation
// ===========================================================================

func TestUT_ParseSetFlags_Valid(t *testing.T) {
	t.Parallel()

	result, err := parseSetFlags([]string{"key=value", "a=b"})
	require.NoError(t, err)
	assert.Equal(t, "value", result["key"])
	assert.Equal(t, "b", result["a"])
}

func TestUT_ParseSetFlags_NilInput(t *testing.T) {
	t.Parallel()

	result, err := parseSetFlags(nil)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestUT_ParseSetFlags_Invalid(t *testing.T) {
	t.Parallel()

	_, err := parseSetFlags([]string{"invalidnoeq"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected key=value")
}

func TestUT_ParseSetFlags_ValueContainsEquals(t *testing.T) {
	t.Parallel()

	result, err := parseSetFlags([]string{"key=val=ue"})
	require.NoError(t, err)
	assert.Equal(t, "val=ue", result["key"])
}

func TestUT_PrintUpdateSummary_AllOps(t *testing.T) {
	// No t.Parallel() — captures os.Stdout via pipe.
	result := &templateupdate.UpdateResult{
		Applied: []templateupdate.MergeResult{
			{Path: "added.go", Op: templateupdate.MergeAdd},
			{Path: "changed.go", Op: templateupdate.MergeUpdate},
			{Path: "removed.go", Op: templateupdate.MergeDelete},
			{Path: "conflict.go", Op: templateupdate.MergeConflict},
		},
	}

	// Capture os.Stdout output (printUpdateSummary writes to os.Stdout)
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printUpdateSummary(result)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	out := buf.String()

	assert.Contains(t, out, "added.go")
	assert.Contains(t, out, "added")
	assert.Contains(t, out, "changed.go")
	assert.Contains(t, out, "updated")
	assert.Contains(t, out, "removed.go")
	assert.Contains(t, out, "deleted")
	assert.Contains(t, out, "conflict.go")
	assert.Contains(t, out, "conflict")
}

func TestUT_UpdateTemplateAction_MutuallyExclusiveFlags_ContinueAbort(t *testing.T) {
	t.Parallel()

	cliFlags := []cli.Flag{
		&cli.BoolFlag{Name: "continue"},
		&cli.BoolFlag{Name: "abort"},
		&cli.BoolFlag{Name: "accept-ours"},
		&cli.BoolFlag{Name: "accept-theirs"},
		&cli.StringFlag{Name: "dir", Value: "."},
		&cli.StringFlag{Name: "ref"},
		&cli.StringSliceFlag{Name: "set"},
		&cli.StringSliceFlag{Name: "skip"},
		&cli.BoolFlag{Name: "dry-run"},
		&cli.BoolFlag{Name: "backup", Value: true},
		&cli.BoolFlag{Name: "skip-hooks"},
		&cli.BoolFlag{Name: "accept-hooks"},
	}

	set := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range cliFlags {
		require.NoError(t, f.Apply(set))
	}
	require.NoError(t, set.Set("continue", "true"))
	require.NoError(t, set.Set("abort", "true"))

	cliApp := &cli.App{Writer: io.Discard, Flags: cliFlags}
	ctx := cli.NewContext(cliApp, set, nil)

	err := updateTemplateAction(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot use --continue and --abort together")
}

func TestUT_UpdateTemplateAction_MutuallyExclusiveFlags_OursTheirs(t *testing.T) {
	t.Parallel()

	cliFlags := []cli.Flag{
		&cli.BoolFlag{Name: "continue"},
		&cli.BoolFlag{Name: "abort"},
		&cli.BoolFlag{Name: "accept-ours"},
		&cli.BoolFlag{Name: "accept-theirs"},
		&cli.StringFlag{Name: "dir", Value: "."},
		&cli.StringFlag{Name: "ref"},
		&cli.StringSliceFlag{Name: "set"},
		&cli.StringSliceFlag{Name: "skip"},
		&cli.BoolFlag{Name: "dry-run"},
		&cli.BoolFlag{Name: "backup", Value: true},
		&cli.BoolFlag{Name: "skip-hooks"},
		&cli.BoolFlag{Name: "accept-hooks"},
	}

	set := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range cliFlags {
		require.NoError(t, f.Apply(set))
	}
	require.NoError(t, set.Set("accept-ours", "true"))
	require.NoError(t, set.Set("accept-theirs", "true"))

	cliApp := &cli.App{Writer: io.Discard, Flags: cliFlags}
	ctx := cli.NewContext(cliApp, set, nil)

	err := updateTemplateAction(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot use --accept-ours and --accept-theirs together")
}

// ===========================================================================
// doctor.go — coverage for printDoctorResults, doctorCheckProject branches,
// doctorCheckSubdir, doctorCheckTemplates with linting
// ===========================================================================

func TestUT_PrintDoctorResults_AllStatuses(t *testing.T) {
	t.Parallel()

	results := []doctorResult{
		doctorResultPass("check A"),
		doctorResultWarn("check B", "warning message"),
		doctorResultFail("check C", "failure message"),
	}

	var buf bytes.Buffer
	printDoctorResults(&buf, results)

	out := buf.String()
	assert.Contains(t, out, "check A")
	assert.Contains(t, out, "check B")
	assert.Contains(t, out, "warning message")
	assert.Contains(t, out, "check C")
	assert.Contains(t, out, "failure message")
}

func TestUT_DoctorCheckProject_NoTagDir_Warn(t *testing.T) {
	t.Parallel()

	results := doctorCheckProject(t.TempDir())
	require.Len(t, results, 1)
	assert.Equal(t, doctorWarn, results[0].status)
	assert.Contains(t, results[0].label, ".tag/ directory")
}

func TestUT_DoctorCheckProject_TagDirIsFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, types.TemplatesDir), []byte("x"), 0o644))

	results := doctorCheckProject(dir)
	require.Len(t, results, 1)
	assert.Equal(t, doctorFail, results[0].status)
	assert.Contains(t, results[0].message, "not a directory")
}

func TestUT_DoctorCheckProject_WithSharedAndBundles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tagDir := filepath.Join(dir, types.TemplatesDir)
	require.NoError(t, os.MkdirAll(filepath.Join(tagDir, types.SharedDir), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tagDir, types.BundlesDir), 0o755))

	results := doctorCheckProject(dir)
	require.GreaterOrEqual(t, len(results), 3)

	// First should pass (main .tag dir)
	assert.Equal(t, doctorPass, results[0].status)
}

func TestUT_DoctorCheckSubdir_ExistsPass(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))

	results := doctorCheckSubdir(dir, "sub", "sub/")
	require.Len(t, results, 1)
	assert.Equal(t, doctorPass, results[0].status)
}

func TestUT_DoctorCheckSubdir_NotExist(t *testing.T) {
	t.Parallel()

	results := doctorCheckSubdir(t.TempDir(), "missing", "missing/")
	require.Len(t, results, 1)
	assert.Equal(t, doctorWarn, results[0].status)
}

func TestUT_DoctorCheckTemplates_NoTagDir_Skipped(t *testing.T) {
	t.Parallel()

	results := doctorCheckTemplates(t.TempDir())
	require.Len(t, results, 1)
	assert.Equal(t, doctorPass, results[0].status)
	assert.Contains(t, results[0].label, "no .tag/ found")
}

func TestUT_DoctorAction_WarnExitCode(t *testing.T) {
	// Doctor with GITHUB_TOKEN unset should produce a warning
	t.Setenv("GITHUB_TOKEN", "")

	var buf bytes.Buffer
	err := doctorAction(context.Background(), &buf, "dev")
	if err != nil {
		var cmdErr *app.CommandError
		if errors.As(err, &cmdErr) {
			// Either warning or failure is acceptable here
			assert.Contains(t, []int{doctorExitWarnings, doctorExitFailures}, cmdErr.Code)
		}
	}
}

// ===========================================================================
// generate.go — coverage for mergeVars, runGenerate (ConflictError),
// printGenerateSummary
// ===========================================================================

func TestUT_MergeVars_BothNil_ReturnsNil(t *testing.T) {
	t.Parallel()

	result := mergeVars(nil, nil)
	assert.Nil(t, result)
}

func TestUT_MergeVars_BaseOnly(t *testing.T) {
	t.Parallel()

	base := map[string]any{"a": 1}
	result := mergeVars(base, nil)
	assert.Equal(t, 1, result["a"])
}

func TestUT_MergeVars_OverlayOverridesBase(t *testing.T) {
	t.Parallel()

	base := map[string]any{"a": 1, "b": 2}
	overlay := map[string]any{"b": 20, "c": 30}
	result := mergeVars(base, overlay)

	assert.Equal(t, 1, result["a"])
	assert.Equal(t, 20, result["b"])
	assert.Equal(t, 30, result["c"])
}

func TestUT_MergeVars_DoesNotMutateInputs(t *testing.T) {
	t.Parallel()

	base := map[string]any{"a": 1}
	overlay := map[string]any{"b": 2}
	_ = mergeVars(base, overlay)

	assert.Len(t, base, 1)
	assert.Len(t, overlay, 1)
}

func TestUT_RunGenerate_ConflictError_ReturnsAppError(t *testing.T) {
	t.Parallel()

	mock := &mockGenerator{
		GenerateFunc: func(_ engine.Data) (engine.GenerateResult, error) {
			return engine.GenerateResult{}, &engine.ConflictError{Files: []string{"file.go"}}
		},
	}

	_, err := runGenerate(mock, engine.Data{})
	require.Error(t, err)

	var cmdErr *app.CommandError
	require.ErrorAs(t, err, &cmdErr)
}

func TestUT_RunGenerate_GenericError_WrapsMessage(t *testing.T) {
	t.Parallel()

	mock := &mockGenerator{
		GenerateFunc: func(_ engine.Data) (engine.GenerateResult, error) {
			return engine.GenerateResult{}, errors.New("disk full")
		},
	}

	_, err := runGenerate(mock, engine.Data{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error when generating template")
}

func TestUT_RunGenerate_Success(t *testing.T) {
	t.Parallel()

	mock := &mockGenerator{
		GenerateFunc: func(_ engine.Data) (engine.GenerateResult, error) {
			return engine.GenerateResult{Created: 3}, nil
		},
	}

	result, err := runGenerate(mock, engine.Data{})
	require.NoError(t, err)
	assert.Equal(t, 3, result.Created)
}

func TestUT_PrintGenerateSummary_AllFieldsNonZero(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	result := engine.GenerateResult{
		Created:     2,
		Skipped:     1,
		Overwritten: 3,
		Modified:    4,
		Details: []engine.FileOpDetail{
			{Op: "created", Path: "a.go"},
			{Op: "skipped", Path: "b.go"},
		},
	}

	printGenerateSummary(&buf, result, true)
	out := buf.String()

	assert.Contains(t, out, "created")
	assert.Contains(t, out, "a.go")
	assert.Contains(t, out, "skipped")
	assert.Contains(t, out, "b.go")
	assert.Contains(t, out, "Generated: 2 created, 1 skipped, 3 overwritten, 4 modified")
}

// ===========================================================================
// completion.go — coverage for bash/zsh/fish subcommand actions
// ===========================================================================

func TestUT_CompletionBash_PrintsScript(t *testing.T) {
	// No t.Parallel() — captures os.Stdout via pipe.
	cliApp := &cli.App{Name: "tag"}
	cmd := CompletionCommand(cliApp)

	// Find bash subcommand
	var bashCmd *cli.Command
	for _, sc := range cmd.Subcommands {
		if sc.Name == "bash" {
			bashCmd = sc
			break
		}
	}
	require.NotNil(t, bashCmd)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	set := flag.NewFlagSet("test", flag.ContinueOnError)
	ctx := cli.NewContext(cliApp, set, nil)
	err := bashCmd.Action(ctx)
	require.NoError(t, err)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	assert.Contains(t, buf.String(), "complete")
}

func TestUT_CompletionZsh_PrintsScript(t *testing.T) {
	// No t.Parallel() — captures os.Stdout via pipe.
	cliApp := &cli.App{Name: "tag"}
	cmd := CompletionCommand(cliApp)

	var zshCmd *cli.Command
	for _, sc := range cmd.Subcommands {
		if sc.Name == "zsh" {
			zshCmd = sc
			break
		}
	}
	require.NotNil(t, zshCmd)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	set := flag.NewFlagSet("test", flag.ContinueOnError)
	ctx := cli.NewContext(cliApp, set, nil)
	err := zshCmd.Action(ctx)
	require.NoError(t, err)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	assert.Contains(t, buf.String(), "#compdef")
}

func TestUT_CompletionFish_PrintsScript(t *testing.T) {
	// No t.Parallel() — captures os.Stdout via pipe.
	cliApp := &cli.App{
		Name: "tag",
		Commands: []*cli.Command{
			{Name: "generate"},
			{Name: "scaffold"},
		},
	}
	cmd := CompletionCommand(cliApp)

	var fishCmd *cli.Command
	for _, sc := range cmd.Subcommands {
		if sc.Name == "fish" {
			fishCmd = sc
			break
		}
	}
	require.NotNil(t, fishCmd)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	set := flag.NewFlagSet("test", flag.ContinueOnError)
	ctx := cli.NewContext(cliApp, set, nil)
	err := fishCmd.Action(ctx)
	require.NoError(t, err)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	assert.Contains(t, buf.String(), "complete")
}

// ===========================================================================
// templatetest.go — coverage for printTestReport and templateTestAction
// ===========================================================================

func TestUT_PrintTestReport_AllCasesPassed(t *testing.T) {
	t.Parallel()

	report := templatetest.Report{
		Cases: []templatetest.CaseResult{
			{Name: "test1", Status: templatetest.CasePassed},
			{Name: "test2", Status: templatetest.CasePassed},
		},
		Passed: 2,
	}

	var buf bytes.Buffer
	printTestReport(&buf, report)
	out := buf.String()

	assert.Contains(t, out, "test1")
	assert.Contains(t, out, "test2")
	assert.Contains(t, out, "2 passed, 0 failed, 0 errored")
	assert.Contains(t, out, "All tests passed.")
}

func TestUT_PrintTestReport_WithCaseFailures(t *testing.T) {
	t.Parallel()

	report := templatetest.Report{
		Cases: []templatetest.CaseResult{
			{
				Name:   "fail-test",
				Status: templatetest.CaseFailed,
				Assertions: []templatetest.AssertionResult{
					{Passed: false, Detail: "file not found: main.go"},
				},
			},
		},
		Failed: 1,
	}

	var buf bytes.Buffer
	printTestReport(&buf, report)
	out := buf.String()

	assert.Contains(t, out, "fail-test")
	assert.Contains(t, out, "FAIL: file not found: main.go")
	assert.Contains(t, out, "0 passed, 1 failed, 0 errored")
}

func TestUT_PrintTestReport_WithCaseErrors(t *testing.T) {
	t.Parallel()

	report := templatetest.Report{
		Cases: []templatetest.CaseResult{
			{
				Name:   "error-test",
				Status: templatetest.CaseErrored,
				Error:  "fixture parse error",
			},
		},
		Errored: 1,
	}

	var buf bytes.Buffer
	printTestReport(&buf, report)
	out := buf.String()

	assert.Contains(t, out, "error-test")
	assert.Contains(t, out, "fixture parse error")
	assert.Contains(t, out, "0 passed, 0 failed, 1 errored")
}

func TestUT_PrintTestReport_NoCases(t *testing.T) {
	t.Parallel()

	report := templatetest.Report{}

	var buf bytes.Buffer
	printTestReport(&buf, report)
	out := buf.String()

	assert.Contains(t, out, "No test fixtures found.")
}

func TestUT_TemplateTestAction_NoFixtures(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	var buf bytes.Buffer
	err := templateTestAction(context.Background(), &buf, dir)
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "0 passed")
}

// ===========================================================================
// generate_agent_file.go — additional coverage
// ===========================================================================

func TestUT_AgentFileFormatNames(t *testing.T) {
	t.Parallel()

	names := agentFileFormatNames()
	assert.Contains(t, names, "claude")
	assert.Contains(t, names, "cursor")
	assert.Contains(t, names, "windsurf")
	assert.Contains(t, names, "copilot")
	assert.Len(t, names, 4)
}

func TestUT_ReplaceMarkerSection_StartOnlyNoEnd(t *testing.T) {
	t.Parallel()

	existing := "before\n" + agentMarkerStart + "\nold content without end marker"
	newContent := "new section"

	result := replaceMarkerSection(existing, newContent)
	assert.Contains(t, result, "new section")
}

func TestUT_WriteAgentFile_ExistingNoNewline(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "file.md")
	// File without trailing newline
	require.NoError(t, os.WriteFile(path, []byte("no newline"), 0o644))

	content := "new content\n"
	err := writeAgentFile(path, content)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	s := string(data)
	assert.Contains(t, s, "no newline")
	assert.Contains(t, s, "new content")
}

func TestUT_GenerateAgentFileCommand_Structure(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cmd := generateAgentFileCommand(cfg)

	assert.Equal(t, "agent-file", cmd.Name)
	assert.NotNil(t, cmd.Action)
	assert.NotNil(t, cmd.BashComplete)
}

// ===========================================================================
// scaffold.go — coverage for handleCookiecutterDetection (non-interactive path)
// ===========================================================================

func TestUT_HandleCookiecutterDetection_NonInteractive(t *testing.T) {
	t.Parallel()

	ctx := newScaffoldActionCLIContext(t, nil, map[string]string{
		"no-input": "true",
	})

	err := handleCookiecutterDetection(ctx, nil, "gh:user/cc-template", t.TempDir(), scaffold.Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Cookiecutter template")
	assert.Contains(t, err.Error(), "non-interactive")
}
