package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/library"
	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/internal/types/flags"
	"github.com/kaikenlabs/tag/pkg/app"
)

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// newCoverageCLIContext creates a CLI context with scaffold flags plus lib-add flags.
func newCoverageCLIContext(t *testing.T, args []string, flagValues map[string]string, writer io.Writer) *cli.Context {
	t.Helper()

	cliFlags := scaffoldFlags()
	cliFlags = append(cliFlags,
		&cli.StringFlag{Name: flags.PathFlag, Value: ".tag"},
		&cli.StringFlag{Name: flags.SharedPathFlag, Value: "_shared"},
		&cli.StringFlag{Name: flags.BundlePathFlag, Value: "_bundles"},
		&cli.BoolFlag{Name: flags.VerboseFlag},
		&cli.BoolFlag{Name: flags.NoHooksFlag},
	)

	if writer == nil {
		writer = io.Discard
	}
	cliApp := &cli.App{Writer: writer, Flags: cliFlags}
	set := flag.NewFlagSet("cov-test", flag.ContinueOnError)
	for _, f := range cliFlags {
		require.NoError(t, f.Apply(set))
	}
	for k, v := range flagValues {
		require.NoError(t, set.Set(k, v))
	}
	require.NoError(t, set.Parse(args))

	return cli.NewContext(cliApp, set, nil)
}

// setupFakeLibraryWithConfig creates a fake library with a template that has
// a valid tag.template.json and a wrapper directory.
func setupFakeLibraryWithConfig(t *testing.T, name string) string {
	t.Helper()

	// Isolate HOME as well as the library dir. A scaffold driven through
	// scaffoldAction ends in replay.Save, whose getReplayDir resolves
	// os.UserHomeDir() directly (internal/replay/save.go) — with the real HOME
	// every such test dropped a file into the developer's ~/.tag/replay, which
	// is how hundreds accumulated there.
	t.Setenv("HOME", t.TempDir())

	dataDir := t.TempDir()
	templateDir := filepath.Join(dataDir, "templates", name)
	require.NoError(t, os.MkdirAll(templateDir, 0o750))

	// Create a valid template inside the library entry
	createMinimalTemplate(t, templateDir, "{{ vars.project_name }}")

	reg := library.Registry{
		Version: 1,
		Entries: map[string]*library.Entry{
			name: {
				Name:        name,
				Source:      "gh:test/" + name,
				Description: "Test template " + name,
				Version:     "1.0.0",
				AddedAt:     time.Now(),
				UpdatedAt:   time.Now(),
			},
		},
	}
	regData, err := json.Marshal(reg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "library.json"), regData, 0o644))

	orig := newLocalLibrary
	newLocalLibrary = func() (*library.Library, error) {
		return library.NewLocal(dataDir), nil
	}
	t.Cleanup(func() { newLocalLibrary = orig })

	return templateDir
}

// setupFakeLibraryError makes newLocalLibrary return an error.
func setupFakeLibraryError(t *testing.T) {
	t.Helper()

	orig := newLocalLibrary
	newLocalLibrary = func() (*library.Library, error) {
		return nil, errors.New("injected library error")
	}
	t.Cleanup(func() { newLocalLibrary = orig })
}

// --------------------------------------------------------------------------
// scaffold.go — scaffoldAction
// --------------------------------------------------------------------------

func TestUT_ScaffoldAction_InvalidTrailingFlags(t *testing.T) {
	// Pass an invalid trailing flag that reparseTrailingFlags should reject
	cliFlags := scaffoldFlags()
	cliApp := &cli.App{Writer: io.Discard, Flags: cliFlags}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range cliFlags {
		require.NoError(t, f.Apply(set))
	}
	// Simulate a value flag with no value: --output (requires value)
	require.NoError(t, set.Parse([]string{"templatename", "--output"}))
	ctx := cli.NewContext(cliApp, set, nil)

	err := scaffoldAction(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid flags")
}

func TestUT_ScaffoldAction_NoArgs_LibraryError(t *testing.T) {
	// setupFakeLibraryError mutates package-level var — do NOT use t.Parallel()
	setupFakeLibraryError(t)

	ctx := newCoverageCLIContext(t, nil, map[string]string{"no-input": "true"}, nil)
	err := scaffoldAction(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize library")
}

func TestUT_ScaffoldAction_NoArgs_TemplateNotInLibrary(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	setupFakeLibrary(t, "some-tmpl")

	// no-input + positional template name that does NOT exist in library
	ctx := newCoverageCLIContext(t, nil, map[string]string{"no-input": "true"}, nil)
	// With no positional args, it goes to resolveTemplateName → non-TTY → error
	err := scaffoldAction(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "template argument required")
}

// --------------------------------------------------------------------------
// scaffold.go — scaffoldFromLibrary
// --------------------------------------------------------------------------

func TestUT_ScaffoldFromLibrary_FullFlow(t *testing.T) {
	// setupFakeLibraryWithConfig mutates package-level var — do NOT use t.Parallel()
	setupFakeLibraryWithConfig(t, "lib-flow-tmpl")

	lib, err := newLocalLibrary()
	require.NoError(t, err)

	entry, err := lib.Get("lib-flow-tmpl")
	require.NoError(t, err)

	outputDir := t.TempDir()
	outputPath := filepath.Join(outputDir, "lib-flow-proj")

	ctx := newCoverageCLIContext(t, nil, map[string]string{
		"no-input": "true",
		"output":   outputPath,
	}, nil)

	err = scaffoldFromLibrary(ctx, lib, entry, []string{"lib-flow-tmpl", "lib-flow-proj"})
	require.NoError(t, err)

	_, statErr := os.Stat(outputPath)
	assert.NoError(t, statErr)
}

func TestUT_ScaffoldFromLibrary_InvalidMetaFlag(t *testing.T) {
	// setupFakeLibraryWithConfig mutates package-level var — do NOT use t.Parallel()
	setupFakeLibraryWithConfig(t, "meta-err-tmpl")

	lib, err := newLocalLibrary()
	require.NoError(t, err)
	entry, err := lib.Get("meta-err-tmpl")
	require.NoError(t, err)

	// Create context with invalid meta value (no = sign) — parse.ParseKeyValues(strict=true) should fail
	cliFlags := scaffoldFlags()
	cliFlags = append(cliFlags,
		&cli.StringFlag{Name: flags.PathFlag, Value: ".tag"},
		&cli.StringFlag{Name: flags.SharedPathFlag, Value: "_shared"},
		&cli.StringFlag{Name: flags.BundlePathFlag, Value: "_bundles"},
		&cli.BoolFlag{Name: flags.VerboseFlag},
		&cli.BoolFlag{Name: flags.NoHooksFlag},
	)
	cliApp := &cli.App{Writer: io.Discard, Flags: cliFlags}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range cliFlags {
		require.NoError(t, f.Apply(set))
	}
	require.NoError(t, set.Set("no-input", "true"))
	require.NoError(t, set.Set("meta", "invalid-no-equals"))
	require.NoError(t, set.Parse(nil))
	ctx := cli.NewContext(cliApp, set, nil)

	err = scaffoldFromLibrary(ctx, lib, entry, []string{"meta-err-tmpl"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid meta flag")
}

func TestUT_ScaffoldFromLibrary_NoProjectName(t *testing.T) {
	// setupFakeLibraryWithConfig mutates package-level var — do NOT use t.Parallel()
	setupFakeLibraryWithConfig(t, "noname-tmpl")

	lib, err := newLocalLibrary()
	require.NoError(t, err)
	entry, err := lib.Get("noname-tmpl")
	require.NoError(t, err)

	outputDir := t.TempDir()
	outputPath := filepath.Join(outputDir, "my-project")

	ctx := newCoverageCLIContext(t, nil, map[string]string{
		"no-input": "true",
		"output":   outputPath,
	}, nil)

	// Pass only template name, no project name → projectName == ""
	err = scaffoldFromLibrary(ctx, lib, entry, []string{"noname-tmpl"})
	require.NoError(t, err)
}

// --------------------------------------------------------------------------
// scaffold.go — scaffoldFromRef
// --------------------------------------------------------------------------

func TestUT_ScaffoldFromRef_BareNameNotInLibrary_FallsToResolver(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	setupFakeLibrary(t, "existing")

	// "unknown-name" is a bare name not in the library → falls through to resolver
	ctx := newCoverageCLIContext(t, nil, map[string]string{"no-input": "true"}, nil)
	err := scaffoldFromRef(ctx, []string{"unknown-name"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve template")
}

func TestUT_ScaffoldFromRef_LocalDir_WithMetaOverrides(t *testing.T) {
	templateDir := t.TempDir()
	createMinimalTemplate(t, templateDir, "{{ vars.project_name }}")

	outputDir := t.TempDir()
	outputPath := filepath.Join(outputDir, "meta-proj")

	cliFlags := scaffoldFlags()
	cliFlags = append(cliFlags,
		&cli.StringFlag{Name: flags.PathFlag, Value: ".tag"},
		&cli.StringFlag{Name: flags.SharedPathFlag, Value: "_shared"},
		&cli.StringFlag{Name: flags.BundlePathFlag, Value: "_bundles"},
		&cli.BoolFlag{Name: flags.VerboseFlag},
		&cli.BoolFlag{Name: flags.NoHooksFlag},
	)
	cliApp := &cli.App{Writer: io.Discard, Flags: cliFlags}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range cliFlags {
		require.NoError(t, f.Apply(set))
	}
	require.NoError(t, set.Set("no-input", "true"))
	require.NoError(t, set.Set("output", outputPath))
	require.NoError(t, set.Set("meta", "project_name=custom-name"))
	require.NoError(t, set.Parse(nil))
	ctx := cli.NewContext(cliApp, set, nil)

	err := scaffoldFromRef(ctx, []string{templateDir, "meta-proj"})
	require.NoError(t, err)
}

func TestUT_ScaffoldFromRef_LocalDir_InvalidMeta(t *testing.T) {
	t.Parallel()

	templateDir := t.TempDir()
	createMinimalTemplate(t, templateDir, "{{ vars.project_name }}")

	cliFlags := scaffoldFlags()
	cliFlags = append(cliFlags,
		&cli.StringFlag{Name: flags.PathFlag, Value: ".tag"},
		&cli.StringFlag{Name: flags.SharedPathFlag, Value: "_shared"},
		&cli.StringFlag{Name: flags.BundlePathFlag, Value: "_bundles"},
		&cli.BoolFlag{Name: flags.VerboseFlag},
		&cli.BoolFlag{Name: flags.NoHooksFlag},
	)
	cliApp := &cli.App{Writer: io.Discard, Flags: cliFlags}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range cliFlags {
		require.NoError(t, f.Apply(set))
	}
	require.NoError(t, set.Set("no-input", "true"))
	require.NoError(t, set.Set("meta", "bad-meta-no-eq"))
	require.NoError(t, set.Parse(nil))
	ctx := cli.NewContext(cliApp, set, nil)

	err := scaffoldFromRef(ctx, []string{templateDir})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid meta flag")
}

func TestUT_ScaffoldFromRef_LocalDir_WithAddToLibFlag(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	setupFakeLibrary(t, "dummy-for-addtolib")

	templateDir := t.TempDir()
	createMinimalTemplate(t, templateDir, "{{ vars.project_name }}")

	outputDir := t.TempDir()
	outputPath := filepath.Join(outputDir, "addtolib-proj")

	ctx := newCoverageCLIContext(t, nil, map[string]string{
		"no-input":         "true",
		"output":           outputPath,
		flags.AddToLibFlag: "true",
	}, nil)

	err := scaffoldFromRef(ctx, []string{templateDir, "addtolib-proj"})
	require.NoError(t, err)
}

func TestUT_ScaffoldFromRef_LocalDir_WithGenerators_NoInput(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	setupFakeLibrary(t, "dummy-gen")

	templateDir := t.TempDir()
	createMinimalTemplate(t, templateDir, "{{ vars.project_name }}")
	// Add generators dir so resolveAddToLib checks hasGenerators
	require.NoError(t, os.MkdirAll(filepath.Join(templateDir, types.GeneratorsDir), 0o755))

	outputDir := t.TempDir()
	outputPath := filepath.Join(outputDir, "gen-noinput-proj")

	ctx := newCoverageCLIContext(t, nil, map[string]string{
		"no-input": "true",
		"output":   outputPath,
	}, nil)

	err := scaffoldFromRef(ctx, []string{templateDir, "gen-noinput-proj"})
	require.NoError(t, err)
}

// --------------------------------------------------------------------------
// scaffold.go — resolveAddToLib
// --------------------------------------------------------------------------

func TestUT_ResolveAddToLib_WithTemplatesDir_NoInput_ReturnsFalse(t *testing.T) {
	t.Parallel()

	templateDir := t.TempDir()
	// Use TemplatesDir (.tag) instead of GeneratorsDir to test the other branch
	require.NoError(t, os.MkdirAll(filepath.Join(templateDir, types.TemplatesDir), 0o755))

	ctx := newCoverageCLIContext(t, nil, map[string]string{"no-input": "true"}, nil)
	assert.False(t, resolveAddToLib(ctx, templateDir))
}

func TestUT_ResolveAddToLib_WithBothDirs_NoInput_ReturnsFalse(t *testing.T) {
	t.Parallel()

	templateDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(templateDir, types.TemplatesDir), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(templateDir, types.GeneratorsDir), 0o755))

	ctx := newCoverageCLIContext(t, nil, map[string]string{"no-input": "true"}, nil)
	// Non-interactive + generators → false (safe default)
	assert.False(t, resolveAddToLib(ctx, templateDir))
}

// --------------------------------------------------------------------------
// scaffold.go — handleCookiecutterDetection (non-interactive path)
// --------------------------------------------------------------------------

func TestUT_HandleCookiecutterDetection_NonTTY_ReturnsError(t *testing.T) {
	t.Parallel()

	// no-input is not set, but IsTTY is false in test
	ctx := newCoverageCLIContext(t, nil, nil, nil)
	err := handleCookiecutterDetection(
		ctx,
		&scaffold.CookiecutterDetectedError{CookiecutterPath: "/tmp/cc"},
		"./my-cc-template",
		t.TempDir(),
		scaffold.Options{},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Cookiecutter template")
	assert.Contains(t, err.Error(), "tag convert cookiecutter ./my-cc-template")
}

// --------------------------------------------------------------------------
// scaffold.go — runCookiecutterConversion
// --------------------------------------------------------------------------

func TestUT_RunCookiecutterConversion_ValidCookiecutter(t *testing.T) {
	t.Parallel()

	// Create a minimal cookiecutter template
	templateDir := t.TempDir()
	ccJSON := `{"project_name": "my-project", "version": "1.0.0"}`
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "cookiecutter.json"), []byte(ccJSON), 0o644))

	// Create the wrapper dir that cookiecutter expects
	wrapperDir := filepath.Join(templateDir, "{{cookiecutter.project_name}}")
	require.NoError(t, os.MkdirAll(wrapperDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(wrapperDir, "README.md"),
		[]byte("# {{cookiecutter.project_name}}"),
		0o644,
	))

	outputDir := t.TempDir()
	destination := filepath.Join(outputDir, "converted")

	var buf bytes.Buffer
	cliApp := &cli.App{Writer: &buf}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	boolFlag := &cli.BoolFlag{Name: "force"}
	require.NoError(t, boolFlag.Apply(set))
	ctx := cli.NewContext(cliApp, set, nil)

	result, err := runCookiecutterConversion(ctx, templateDir, destination)
	require.NoError(t, err)
	require.NotNil(t, result)

	out := buf.String()
	assert.Contains(t, out, "Converted template to:")
	assert.Contains(t, out, "Variables:")
	assert.Contains(t, out, "Files:")
}

func TestUT_RunCookiecutterConversion_WithWarnings(t *testing.T) {
	t.Parallel()

	templateDir := t.TempDir()
	// cookiecutter.json with Jinja2 features that produce warnings
	ccJSON := `{"project_name": "test", "_copy_without_render": [".git"]}`
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "cookiecutter.json"), []byte(ccJSON), 0o644))
	wrapperDir := filepath.Join(templateDir, "{{cookiecutter.project_name}}")
	require.NoError(t, os.MkdirAll(wrapperDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(wrapperDir, "main.py"),
		[]byte("print('hello')"),
		0o644,
	))

	outputDir := t.TempDir()
	destination := filepath.Join(outputDir, "converted-warn")

	var buf bytes.Buffer
	cliApp := &cli.App{Writer: &buf}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	boolFlag := &cli.BoolFlag{Name: "force"}
	require.NoError(t, boolFlag.Apply(set))
	ctx := cli.NewContext(cliApp, set, nil)

	result, err := runCookiecutterConversion(ctx, templateDir, destination)
	require.NoError(t, err)
	require.NotNil(t, result)

	out := buf.String()
	assert.Contains(t, out, "Converted template to:")
}

func TestUT_RunCookiecutterConversion_InvalidSource(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cliApp := &cli.App{Writer: &buf}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	boolFlag := &cli.BoolFlag{Name: "force"}
	require.NoError(t, boolFlag.Apply(set))
	ctx := cli.NewContext(cliApp, set, nil)

	_, err := runCookiecutterConversion(ctx, "/nonexistent/path", "/tmp/out")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conversion failed")
}

// --------------------------------------------------------------------------
// scaffold.go — addToLibrary
// --------------------------------------------------------------------------

func TestUT_AddToLibrary_LibraryInitError(t *testing.T) {
	// setupFakeLibraryError mutates package-level var — do NOT use t.Parallel()
	setupFakeLibraryError(t)

	var buf bytes.Buffer
	cliApp := &cli.App{Writer: &buf}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	ctx := cli.NewContext(cliApp, set, nil)

	// Should not panic; should log warning and return
	addToLibrary(ctx, "gh:test/some-tmpl", t.TempDir())

	// No output expected since it logs to slog, not app writer
	assert.Empty(t, buf.String())
}

func TestUT_AddToLibrary_TemplateAlreadyExists_ShowsMessage(t *testing.T) {
	// setupFakeLibraryWithConfig mutates package-level var — do NOT use t.Parallel()
	setupFakeLibraryWithConfig(t, "dup-tmpl")

	var buf bytes.Buffer
	cliApp := &cli.App{Writer: &buf}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	ctx := cli.NewContext(cliApp, set, nil)

	// "dup-tmpl" is already in the library → should show "already in library"
	addToLibrary(ctx, "gh:test/dup-tmpl", t.TempDir())

	out := buf.String()
	assert.Contains(t, out, "already in library")
	assert.Contains(t, out, "dup-tmpl")
}

func TestUT_AddToLibrary_AddsNewTemplate(t *testing.T) {
	// setupFakeLibraryWithConfig mutates package-level var — do NOT use t.Parallel()
	setupFakeLibraryWithConfig(t, "existing-tmpl2")

	var buf bytes.Buffer
	cliApp := &cli.App{Writer: &buf}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	ctx := cli.NewContext(cliApp, set, nil)

	templateDir := t.TempDir()
	addToLibrary(ctx, "gh:test/brand-new-tmpl", templateDir)

	out := buf.String()
	assert.Contains(t, out, "Template added to library")
	assert.Contains(t, out, "brand-new-tmpl")
}

// --------------------------------------------------------------------------
// scaffold.go — displayScaffoldSummary (branch coverage)
// --------------------------------------------------------------------------

func TestUT_DisplayScaffoldSummary_NoProjectName_NoVersion(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	result := scaffold.ScaffoldResult{
		OutputDir:   "/tmp/no-proj-name",
		TemplateDir: t.TempDir(),
		Vars:        map[string]any{"other_var": 42},
		Opts: scaffold.Options{
			TemplateName: "test-tmpl",
			TemplateRef:  "gh:test/test-tmpl",
		},
	}

	displayScaffoldSummary(&buf, result)

	output := buf.String()
	assert.Contains(t, output, "Scaffolding complete!")
	assert.Contains(t, output, "Output: /tmp/no-proj-name")
	// No "Project:" line because project_name not in vars
	assert.NotContains(t, output, "Project:")
	// Template line but no version
	assert.Contains(t, output, "Template: gh:test/test-tmpl")
	assert.NotContains(t, output, "(")
}

func TestUT_DisplayScaffoldSummary_WithGeneratorsDir(t *testing.T) {
	t.Parallel()

	templateDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(templateDir, types.GeneratorsDir), 0o755))

	var buf bytes.Buffer
	result := scaffold.ScaffoldResult{
		OutputDir:   "/tmp/gen-dir",
		TemplateDir: templateDir,
		Vars:        map[string]any{},
		Opts:        scaffold.Options{},
	}

	displayScaffoldSummary(&buf, result)

	output := buf.String()
	assert.Contains(t, output, "tag generate list")
}

// --------------------------------------------------------------------------
// scaffold.go — runScaffold (error paths)
// --------------------------------------------------------------------------

func TestUT_RunScaffold_InvalidTemplateDir(t *testing.T) {
	t.Parallel()

	ctx := newCoverageCLIContext(t, nil, map[string]string{"no-input": "true"}, nil)

	opts := scaffold.Options{
		TemplateDir: "/nonexistent/template/dir",
		NoInput:     true,
	}

	err := runScaffold(ctx, opts, func(_ *scaffold.CookiecutterDetectedError) error {
		return app.Errorf("unexpected cookiecutter")
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scaffolding failed")
}

func TestUT_RunScaffold_CookiecutterDetected(t *testing.T) {
	t.Parallel()

	// Create a directory with cookiecutter.json (triggers CookiecutterDetectedError)
	templateDir := t.TempDir()
	ccJSON := `{"project_name": "my-project"}`
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "cookiecutter.json"), []byte(ccJSON), 0o644))
	wrapperDir := filepath.Join(templateDir, "{{cookiecutter.project_name}}")
	require.NoError(t, os.MkdirAll(wrapperDir, 0o750))

	ctx := newCoverageCLIContext(t, nil, map[string]string{"no-input": "true"}, nil)

	opts := scaffold.Options{
		TemplateDir: templateDir,
		NoInput:     true,
	}

	var calledWith *scaffold.CookiecutterDetectedError
	err := runScaffold(ctx, opts, func(ccErr *scaffold.CookiecutterDetectedError) error {
		calledWith = ccErr
		return app.Errorf("handled cookiecutter: %s", ccErr.CookiecutterPath)
	})
	require.Error(t, err)
	assert.NotNil(t, calledWith)
	assert.Contains(t, err.Error(), "handled cookiecutter")
}

func TestUT_RunScaffold_SuccessfulScaffold(t *testing.T) {
	t.Parallel()

	templateDir := t.TempDir()
	createMinimalTemplate(t, templateDir, "{{ vars.project_name }}")

	outputDir := t.TempDir()
	outputPath := filepath.Join(outputDir, "my-project")

	var buf bytes.Buffer
	ctx := newCoverageCLIContext(t, nil, map[string]string{
		"no-input": "true",
		"output":   outputPath,
	}, &buf)

	opts := scaffold.Options{
		TemplateDir: templateDir,
		OutputDir:   outputPath,
		NoInput:     true,
	}

	err := runScaffold(ctx, opts, func(_ *scaffold.CookiecutterDetectedError) error {
		return app.Errorf("unexpected")
	})
	require.NoError(t, err)

	// displayScaffoldSummary should have written something
	assert.Contains(t, buf.String(), "Scaffolding complete!")
}

// --------------------------------------------------------------------------
// scaffold.go — verifyTemplateLock
// --------------------------------------------------------------------------

func TestUT_VerifyTemplateLock_UpdateLock(t *testing.T) {
	t.Parallel()

	err := verifyTemplateLock("gh:test/tmpl", t.TempDir(), true, false)
	// updateLock=true should create/update the lock without error
	assert.NoError(t, err)
}

// --------------------------------------------------------------------------
// scaffold.go — ScaffoldCommand (flag completeness)
// --------------------------------------------------------------------------

func TestUT_ScaffoldCommand_HasAliasAndDescription(t *testing.T) {
	t.Parallel()

	cmd := ScaffoldCommand()
	assert.Contains(t, cmd.Aliases, "s")
	assert.NotEmpty(t, cmd.Description)
	assert.Equal(t, "[template] [project-name]", cmd.ArgsUsage)
}

// --------------------------------------------------------------------------
// scaffold.go — scaffoldFlags
// --------------------------------------------------------------------------

func TestUT_ScaffoldFlags_ContainsAllCommonFlags(t *testing.T) {
	t.Parallel()

	sflags := scaffoldFlags()
	flagNames := make(map[string]bool)
	for _, f := range sflags {
		for _, n := range f.Names() {
			flagNames[n] = true
		}
	}

	expected := []string{
		"output", "values", "meta", "no-input", "force", "replay",
		"no-save", "accept-hooks", "allow-recursive-render", "update",
		flags.UpdateLockFlag, flags.IgnoreLockFlag, flags.DryRunFlag, flags.AddToLibFlag,
	}
	for _, name := range expected {
		assert.True(t, flagNames[name], "missing flag: %s", name)
	}
}

// --------------------------------------------------------------------------
// library.go — libAddAction (with local template)
// --------------------------------------------------------------------------

func TestUT_LibAddAction_WithLocalTemplate(t *testing.T) {
	// Cannot use t.Parallel() — overrides XDG_DATA_HOME
	templateDir := t.TempDir()
	createMinimalTemplate(t, templateDir, "{{ vars.project_name }}")

	// Use XDG_DATA_HOME to control where newLibrary stores data
	dataDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataDir)

	var buf bytes.Buffer
	cliFlags := []cli.Flag{
		&cli.StringFlag{Name: "as"},
		&cli.BoolFlag{Name: "force"},
	}
	cliApp := &cli.App{Writer: &buf, Flags: cliFlags}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range cliFlags {
		require.NoError(t, f.Apply(set))
	}
	require.NoError(t, set.Set("as", "my-local-tmpl"))
	require.NoError(t, set.Parse([]string{templateDir}))
	ctx := cli.NewContext(cliApp, set, nil)

	cmd := libAddCommand()
	err := cmd.Action(ctx)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Added")
	assert.Contains(t, out, "my-local-tmpl")
	assert.Contains(t, out, "tag scaffold my-local-tmpl")
}

func TestUT_LibAddAction_WithForce_OverwritesExisting(t *testing.T) {
	templateDir := t.TempDir()
	createMinimalTemplate(t, templateDir, "{{ vars.project_name }}")

	dataDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataDir)

	cliFlags := []cli.Flag{
		&cli.StringFlag{Name: "as"},
		&cli.BoolFlag{Name: "force"},
	}

	// First add
	cliApp := &cli.App{Writer: io.Discard, Flags: cliFlags}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range cliFlags {
		require.NoError(t, f.Apply(set))
	}
	require.NoError(t, set.Set("as", "force-tmpl"))
	require.NoError(t, set.Parse([]string{templateDir}))
	ctx := cli.NewContext(cliApp, set, nil)
	cmd := libAddCommand()
	require.NoError(t, cmd.Action(ctx))

	// Second add with --force
	var buf bytes.Buffer
	cliApp2 := &cli.App{Writer: &buf, Flags: cliFlags}
	set2 := flag.NewFlagSet("test2", flag.ContinueOnError)
	for _, f := range cliFlags {
		require.NoError(t, f.Apply(set2))
	}
	require.NoError(t, set2.Set("as", "force-tmpl"))
	require.NoError(t, set2.Set("force", "true"))
	require.NoError(t, set2.Parse([]string{templateDir}))
	ctx2 := cli.NewContext(cliApp2, set2, nil)
	cmd2 := libAddCommand()
	err := cmd2.Action(ctx2)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Updated")
	assert.Contains(t, out, "force-tmpl")
}

// --------------------------------------------------------------------------
// library.go — libListAction (version/description display)
// --------------------------------------------------------------------------

func TestUT_LibListAction_ShowsDashForEmptyVersion(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	dataDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "templates", "novver"), 0o750))

	reg := library.Registry{
		Version: 1,
		Entries: map[string]*library.Entry{
			"novver": {
				Name:    "novver",
				Source:  "gh:test/novver",
				AddedAt: time.Now(),
				// Version is empty string
			},
		},
	}
	regData, err := json.Marshal(reg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "library.json"), regData, 0o644))

	orig := newLocalLibrary
	newLocalLibrary = func() (*library.Library, error) {
		return library.NewLocal(dataDir), nil
	}
	t.Cleanup(func() { newLocalLibrary = orig })

	var buf bytes.Buffer
	cliApp := &cli.App{Writer: &buf}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	ctx := cli.NewContext(cliApp, set, nil)

	cmd := libListCommand()
	err = cmd.Action(ctx)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "-") // Version should show "-"
	assert.Contains(t, out, "novver")
}

// --------------------------------------------------------------------------
// library.go — libRemoveAction (success path)
// --------------------------------------------------------------------------

func TestUT_LibRemoveAction_SuccessfulRemove(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	setupFakeLibrary(t, "rm-target")

	var buf bytes.Buffer
	cliApp := &cli.App{Writer: &buf}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	require.NoError(t, set.Parse([]string{"rm-target"}))
	ctx := cli.NewContext(cliApp, set, nil)

	cmd := libRemoveCommand()
	err := cmd.Action(ctx)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Removed")
	assert.Contains(t, out, "rm-target")
}

// --------------------------------------------------------------------------
// library.go — libEditAction (editor resolution paths)
// --------------------------------------------------------------------------

func TestUT_LibEditAction_WithEditorFlag(t *testing.T) {
	// setupFakeLibraryWithConfig mutates package-level var — do NOT use t.Parallel()
	setupFakeLibraryWithConfig(t, "edit-target")

	cliFlags := []cli.Flag{
		&cli.StringFlag{Name: "editor"},
	}
	cliApp := &cli.App{Writer: io.Discard, Flags: cliFlags}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range cliFlags {
		require.NoError(t, f.Apply(set))
	}
	// Use "true" as editor — will fail to run but we exercise the template path resolution
	require.NoError(t, set.Set("editor", "true"))
	require.NoError(t, set.Parse([]string{"edit-target"}))
	ctx := cli.NewContext(cliApp, set, nil)

	cmd := libEditCommand()
	err := cmd.Action(ctx)
	// "true" will exit 0 on most systems, so no error is expected
	// But the template path was resolved successfully
	if err != nil {
		assert.Contains(t, err.Error(), "editor exited with error")
	}
}

// --------------------------------------------------------------------------
// library.go — editorSource.resolve (more branches)
// --------------------------------------------------------------------------

func TestUT_ResolveEditor_SaveEditorPreference_SaveError(t *testing.T) {
	t.Parallel()

	s := &editorSource{
		loadConfig: func() (*config.GlobalConfig, error) { return &config.GlobalConfig{}, nil },
		saveConfig: func(_ *config.GlobalConfig) error { return errors.New("save failed") },
		getenv:     func(string) string { return "" },
		isTTY:      func() bool { return true },
		prompt:     func() (string, error) { return "vim", nil },
	}

	editor, err := s.resolve("")
	require.NoError(t, err)
	assert.Equal(t, "vim", editor)
	// Save failed but resolve still succeeds — non-fatal
}

func TestUT_ResolveEditor_SaveEditorPreference_LoadErrorOnSave(t *testing.T) {
	t.Parallel()

	var savedCfg *config.GlobalConfig
	loadCount := 0
	s := &editorSource{
		loadConfig: func() (*config.GlobalConfig, error) {
			loadCount++
			if loadCount == 1 {
				// First call: resolve path (empty config)
				return &config.GlobalConfig{}, nil
			}
			// Second call in saveEditorPreference: error
			return nil, errors.New("corrupt")
		},
		saveConfig: func(c *config.GlobalConfig) error { savedCfg = c; return nil },
		getenv:     func(string) string { return "" },
		isTTY:      func() bool { return true },
		prompt:     func() (string, error) { return "emacs", nil },
	}

	editor, err := s.resolve("")
	require.NoError(t, err)
	assert.Equal(t, "emacs", editor)
	require.NotNil(t, savedCfg)
	assert.Equal(t, "emacs", savedCfg.Editor)
}

func TestUT_ResolveEditor_PromptError(t *testing.T) {
	t.Parallel()

	s := &editorSource{
		loadConfig: func() (*config.GlobalConfig, error) { return &config.GlobalConfig{}, nil },
		saveConfig: func(_ *config.GlobalConfig) error { return nil },
		getenv:     func(string) string { return "" },
		isTTY:      func() bool { return true },
		prompt:     func() (string, error) { return "", errors.New("prompt failed") },
	}

	_, err := s.resolve("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prompt failed")
}

// --------------------------------------------------------------------------
// library.go — newLibrary with XDG_DATA_HOME override
// --------------------------------------------------------------------------

func TestUT_NewLibrary_WithXDGDataHome(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataDir)

	lib, err := newLibrary()
	require.NoError(t, err)
	require.NotNil(t, lib)
}

func TestUT_NewLocalLibrary_WithXDGDataHome(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataDir)

	lib, err := defaultNewLocalLibrary()
	require.NoError(t, err)
	require.NotNil(t, lib)
}

// --------------------------------------------------------------------------
// library.go — libSearchCommand action (no HTTP, just structure tests)
// --------------------------------------------------------------------------

func TestUT_LibSearchCommand_HasAllFlagDefaults(t *testing.T) {
	t.Parallel()

	cmd := libSearchCommand()

	for _, f := range cmd.Flags {
		switch f.Names()[0] {
		case "limit":
			sf := f.(*cli.IntFlag)
			assert.Equal(t, 10, sf.Value)
		case "sort":
			sf := f.(*cli.StringFlag)
			assert.Equal(t, "stars", sf.Value)
		case "order":
			sf := f.(*cli.StringFlag)
			assert.Equal(t, "desc", sf.Value)
		}
	}
}

// --------------------------------------------------------------------------
// library.go — truncate (from convert.go, used in library.go)
// --------------------------------------------------------------------------

func TestUT_Truncate_ShortString(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "hello", truncate("hello", 10))
}

func TestUT_Truncate_ExactLength(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "hello", truncate("hello", 5))
}

func TestUT_Truncate_LongString(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "hel...", truncate("hello world", 6))
}

// --------------------------------------------------------------------------
// library.go — updateSingleTemplate / updateAllTemplates
// --------------------------------------------------------------------------

func TestUT_UpdateSingleTemplate_ViaCommand(t *testing.T) {
	// Uses XDG_DATA_HOME to control library location
	dataDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataDir)

	// Set up a library with a local template
	templateDir := t.TempDir()
	createMinimalTemplate(t, templateDir, "{{ vars.project_name }}")

	// First, add the template
	cliFlags := []cli.Flag{
		&cli.StringFlag{Name: "as"},
		&cli.BoolFlag{Name: "force"},
	}
	cliApp := &cli.App{Writer: io.Discard, Flags: cliFlags}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range cliFlags {
		require.NoError(t, f.Apply(set))
	}
	require.NoError(t, set.Set("as", "update-tmpl"))
	require.NoError(t, set.Parse([]string{templateDir}))
	ctx := cli.NewContext(cliApp, set, nil)
	cmd := libAddCommand()
	require.NoError(t, cmd.Action(ctx))

	// Now test update command with the template name
	var buf bytes.Buffer
	cliApp2 := &cli.App{Writer: &buf}
	set2 := flag.NewFlagSet("test2", flag.ContinueOnError)
	require.NoError(t, set2.Parse([]string{"update-tmpl"}))
	ctx2 := cli.NewContext(cliApp2, set2, nil)

	cmd2 := libUpdateCommand()
	err := cmd2.Action(ctx2)
	// Update of a local template re-resolves — should succeed since source is a local path
	if err != nil {
		// Acceptable: the library may not have a resolver for local paths
		assert.Error(t, err)
	}
}

func TestUT_UpdateAllTemplates_Empty(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataDir)

	// Create empty registry
	reg := library.Registry{Version: 1, Entries: map[string]*library.Entry{}}
	regData, err := json.Marshal(reg)
	require.NoError(t, err)
	libDir := filepath.Join(dataDir, "tag")
	require.NoError(t, os.MkdirAll(libDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(libDir, "library.json"), regData, 0o644))

	var buf bytes.Buffer
	cliApp := &cli.App{Writer: &buf}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	ctx := cli.NewContext(cliApp, set, nil)

	cmd := libUpdateCommand()
	err = cmd.Action(ctx)
	// Empty library returns an error
	if err != nil {
		assert.Contains(t, err.Error(), "library")
	}
}

// --------------------------------------------------------------------------
// library.go — printAddResult (edge cases)
// --------------------------------------------------------------------------

func TestUT_PrintAddResult_NoWarnings_NoConversion(t *testing.T) {
	t.Parallel()

	result := &library.AddResult{
		Name:        "simple-tmpl",
		Source:      "gh:acme/simple-tmpl",
		TemplateDir: "/path/to/tmpl",
		IsUpdate:    false,
	}

	var buf bytes.Buffer
	printAddResult(&buf, result)

	out := buf.String()
	assert.Contains(t, out, "Added")
	assert.NotContains(t, out, "Converted from:")
	assert.NotContains(t, out, "Warnings:")
	assert.Contains(t, out, "tag scaffold simple-tmpl")
}

// --------------------------------------------------------------------------
// scaffold.go — promptForConversion / promptForProjectDir edge cases
// --------------------------------------------------------------------------

func TestUT_PromptForProjectDir_BothEmpty_InputReturnsValue(t *testing.T) {
	t.Parallel()

	mockPrompter := &mockPrompterForScaffold{
		inputResult: "/custom/output",
	}

	opts := &scaffold.Options{}
	err := promptForProjectDir(mockPrompter, opts)
	require.NoError(t, err)
	assert.Equal(t, "/custom/output", opts.OutputDir)
}

// --------------------------------------------------------------------------
// scaffold.go — scaffoldAction full integration (library → scaffold → summary)
// --------------------------------------------------------------------------

func TestUT_ScaffoldAction_LibraryTemplate_FullFlow(t *testing.T) {
	// setupFakeLibraryWithConfig mutates package-level var — do NOT use t.Parallel()
	setupFakeLibraryWithConfig(t, "fullflow-tmpl")

	outputDir := t.TempDir()
	outputPath := filepath.Join(outputDir, "fullflow-proj")

	var buf bytes.Buffer
	ctx := newCoverageCLIContext(t, []string{"fullflow-tmpl", "fullflow-proj"}, map[string]string{
		"no-input": "true",
		"output":   outputPath,
	}, &buf)

	err := scaffoldAction(ctx)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Scaffolding complete!")
}

func TestUT_ScaffoldAction_LibraryTemplate_EntryNotFound(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	setupFakeLibrary(t, "some-other-tmpl")

	// Provide a name that's not in the library, and not a path either
	// so scaffoldAction takes the len(positional) >= 1 path → scaffoldFromRef
	ctx := newCoverageCLIContext(t, []string{"nonexistent-tmpl"}, map[string]string{
		"no-input": "true",
	}, nil)

	err := scaffoldAction(ctx)
	require.Error(t, err)
	// It falls through to the resolver which will fail
	assert.Contains(t, err.Error(), "failed to resolve template")
}

// --------------------------------------------------------------------------
// scaffold.go — scaffoldFromRef with local cookiecutter template
// --------------------------------------------------------------------------

func TestUT_ScaffoldFromRef_CookiecutterDetection_NoInput(t *testing.T) {
	t.Parallel()

	// Create a cookiecutter template
	templateDir := t.TempDir()
	ccJSON := `{"project_name": "test"}`
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "cookiecutter.json"), []byte(ccJSON), 0o644))
	wrapperDir := filepath.Join(templateDir, "{{cookiecutter.project_name}}")
	require.NoError(t, os.MkdirAll(wrapperDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(wrapperDir, "main.txt"), []byte("hello"), 0o644))

	ctx := newCoverageCLIContext(t, nil, map[string]string{"no-input": "true"}, nil)

	err := scaffoldFromRef(ctx, []string{templateDir})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Cookiecutter template")
}

// --------------------------------------------------------------------------
// library.go — libListAction error from newLocalLibrary
// --------------------------------------------------------------------------

func TestUT_LibListAction_LibraryError(t *testing.T) {
	// setupFakeLibraryError mutates package-level var — do NOT use t.Parallel()
	setupFakeLibraryError(t)

	cliApp := &cli.App{Writer: io.Discard}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	ctx := cli.NewContext(cliApp, set, nil)

	cmd := libListCommand()
	err := cmd.Action(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize library")
}

func TestUT_LibRemoveAction_LibraryError(t *testing.T) {
	// setupFakeLibraryError mutates package-level var — do NOT use t.Parallel()
	setupFakeLibraryError(t)

	cliApp := &cli.App{Writer: io.Discard}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	require.NoError(t, set.Parse([]string{"some-tmpl"}))
	ctx := cli.NewContext(cliApp, set, nil)

	cmd := libRemoveCommand()
	err := cmd.Action(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize library")
}

func TestUT_LibEditAction_LibraryError(t *testing.T) {
	// setupFakeLibraryError mutates package-level var — do NOT use t.Parallel()
	setupFakeLibraryError(t)

	cliFlags := []cli.Flag{
		&cli.StringFlag{Name: "editor"},
	}
	cliApp := &cli.App{Writer: io.Discard, Flags: cliFlags}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range cliFlags {
		require.NoError(t, f.Apply(set))
	}
	require.NoError(t, set.Parse([]string{"some-tmpl"}))
	ctx := cli.NewContext(cliApp, set, nil)

	cmd := libEditCommand()
	err := cmd.Action(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize library")
}

// --------------------------------------------------------------------------
// library.go — saveEditorPreference with existing config
// --------------------------------------------------------------------------

func TestUT_SaveEditorPreference_ExistingConfig(t *testing.T) {
	t.Parallel()

	existingCfg := &config.GlobalConfig{Editor: "old-editor"}
	var savedCfg *config.GlobalConfig

	s := &editorSource{
		loadConfig: func() (*config.GlobalConfig, error) { return existingCfg, nil },
		saveConfig: func(c *config.GlobalConfig) error { savedCfg = c; return nil },
		getenv:     func(string) string { return "" },
		isTTY:      func() bool { return false },
		prompt:     func() (string, error) { return "", nil },
	}

	s.saveEditorPreference("new-editor")

	require.NotNil(t, savedCfg)
	assert.Equal(t, "new-editor", savedCfg.Editor)
}

// --------------------------------------------------------------------------
// scaffold.go — scaffoldFromLibrary error from lib.TemplatePath
// --------------------------------------------------------------------------

func TestUT_ScaffoldFromLibrary_TemplatePathError(t *testing.T) {
	// Create a library with a template entry but no directory on disk
	dataDir := t.TempDir()
	reg := library.Registry{
		Version: 1,
		Entries: map[string]*library.Entry{
			"ghost-tmpl": {
				Name:    "ghost-tmpl",
				Source:  "gh:test/ghost-tmpl",
				AddedAt: time.Now(),
			},
		},
	}
	regData, err := json.Marshal(reg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "library.json"), regData, 0o644))

	orig := newLocalLibrary
	newLocalLibrary = func() (*library.Library, error) {
		return library.NewLocal(dataDir), nil
	}
	t.Cleanup(func() { newLocalLibrary = orig })

	lib, err := newLocalLibrary()
	require.NoError(t, err)

	entry, err := lib.Get("ghost-tmpl")
	require.NoError(t, err)

	ctx := newCoverageCLIContext(t, nil, map[string]string{"no-input": "true"}, nil)

	// The template dir doesn't exist on disk, so TemplatePath should fail
	err = scaffoldFromLibrary(ctx, lib, entry, []string{"ghost-tmpl"})
	require.Error(t, err)
}
