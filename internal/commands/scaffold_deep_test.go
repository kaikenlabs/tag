package commands

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/library"
	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/internal/types/flags"
)

// --- addToLibrary ---

func TestUT_AddToLibrary_DuplicateSkipped(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	setupFakeLibrary(t, "existing-tmpl")

	var buf bytes.Buffer
	cliApp := &cli.App{Writer: &buf}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	ctx := cli.NewContext(cliApp, set, nil)

	addToLibrary(ctx, "gh:test/existing-tmpl", t.TempDir())

	out := buf.String()
	assert.Contains(t, out, "already in library")
	assert.Contains(t, out, "existing-tmpl")
}

func TestUT_AddToLibrary_NewTemplate(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	setupFakeLibrary(t, "other-tmpl")

	templateDir := t.TempDir()

	var buf bytes.Buffer
	cliApp := &cli.App{Writer: &buf}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	ctx := cli.NewContext(cliApp, set, nil)

	addToLibrary(ctx, "gh:test/new-template", templateDir)

	out := buf.String()
	assert.Contains(t, out, "Template added to library")
	assert.Contains(t, out, "new-template")
}

// --- verifyTemplateLock ---

func TestUT_VerifyTemplateLock_WorksDirNoError(t *testing.T) {
	t.Parallel()

	// Create a temp dir as project root, which will have no lock file.
	// verifyTemplateLock should not fail — it just verifies or creates.
	templateDir := t.TempDir()

	err := verifyTemplateLock("gh:test/tmpl", templateDir, false, true)
	assert.NoError(t, err) // ignoreLock=true skips verification
}

func TestUT_VerifyTemplateLock_IgnoreLock(t *testing.T) {
	t.Parallel()

	err := verifyTemplateLock("gh:test/tmpl", t.TempDir(), false, true)
	assert.NoError(t, err)
}

// --- promptForProjectDir ---

func TestUT_PromptForProjectDir_EmptyBothFields_UsesPrompt(t *testing.T) {
	t.Parallel()

	mockPrompter := &mockPrompterForScaffold{
		inputResult: "/tmp/custom-project",
	}

	opts := &scaffold.Options{}
	err := promptForProjectDir(mockPrompter, opts)
	require.NoError(t, err)
	assert.Equal(t, "/tmp/custom-project", opts.OutputDir)
}

func TestUT_PromptForProjectDir_EmptyInput_UsesDefault(t *testing.T) {
	t.Parallel()

	mockPrompter := &mockPrompterForScaffold{
		inputResult: "", // returns empty, should use default
	}

	opts := &scaffold.Options{}
	err := promptForProjectDir(mockPrompter, opts)
	require.NoError(t, err)
	assert.Equal(t, "./my-project", opts.OutputDir)
}

func TestUT_PromptForProjectDir_PromptError(t *testing.T) {
	t.Parallel()

	mockPrompter := &mockPrompterForScaffold{
		inputErr: assert.AnError,
	}

	opts := &scaffold.Options{}
	err := promptForProjectDir(mockPrompter, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prompt failed")
}

// --- promptForConversion ---

func TestUT_PromptForConversion_Accepted_WithCustomDestination(t *testing.T) {
	t.Parallel()

	mockPrompter := &mockPrompterForScaffold{
		confirmResult: true,
		inputResult:   "/tmp/converted",
	}

	dest, err := promptForConversion(mockPrompter, "gh:user/cookiecutter-api")
	require.NoError(t, err)
	assert.Equal(t, "/tmp/converted", dest)
}

func TestUT_PromptForConversion_Accepted_EmptyDestinationUsesDefault(t *testing.T) {
	t.Parallel()

	mockPrompter := &mockPrompterForScaffold{
		confirmResult: true,
		inputResult:   "", // empty → uses default
	}

	dest, err := promptForConversion(mockPrompter, "gh:user/cookiecutter-api")
	require.NoError(t, err)
	assert.Equal(t, "./api-tag", dest)
}

func TestUT_PromptForConversion_ConfirmError(t *testing.T) {
	t.Parallel()

	mockPrompter := &mockPrompterForScaffold{
		confirmErr: assert.AnError,
	}

	_, err := promptForConversion(mockPrompter, "gh:user/cookiecutter-api")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prompt failed")
}

func TestUT_PromptForConversion_InputError(t *testing.T) {
	t.Parallel()

	mockPrompter := &mockPrompterForScaffold{
		confirmResult: true,
		inputErr:      assert.AnError,
	}

	_, err := promptForConversion(mockPrompter, "gh:user/cookiecutter-api")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prompt failed")
}

// --- ScaffoldCommand structure ---

func TestUT_ScaffoldCommand_Structure(t *testing.T) {
	t.Parallel()

	cmd := ScaffoldCommand()
	require.NotNil(t, cmd)
	assert.Equal(t, "scaffold", cmd.Name)
	assert.Contains(t, cmd.Aliases, "s")
	assert.NotNil(t, cmd.Action)
	assert.NotNil(t, cmd.BashComplete)
	assert.NotEmpty(t, cmd.Flags)
}

// --- DisplayScaffoldSummary with version but no name ---

func TestUT_DisplayScaffoldSummary_VersionOnly_NoTemplateName(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	result := scaffold.ScaffoldResult{
		OutputDir:   "/tmp/proj",
		TemplateDir: t.TempDir(),
		Vars:        map[string]any{},
		Opts: scaffold.Options{
			TemplateVersion: "1.0.0",
		},
	}

	displayScaffoldSummary(&buf, result)

	output := buf.String()
	assert.NotContains(t, output, "Template:")
	assert.Contains(t, output, "Scaffolding complete!")
}

// --- scaffoldFromRef with output dir specified ---

func TestUT_ScaffoldFromRef_WithOutputDir(t *testing.T) {
	templateDir := t.TempDir()
	createMinimalTemplate(t, templateDir, "{{ vars.project_name }}")

	outputDir := t.TempDir()
	outputPath := filepath.Join(outputDir, "out-proj")

	ctx := newScaffoldActionCLIContext(t, nil, map[string]string{
		"no-input": "true",
		"output":   outputPath,
	})

	err := scaffoldFromRef(ctx, []string{templateDir, "out-proj"})
	require.NoError(t, err)

	_, statErr := os.Stat(outputPath)
	assert.NoError(t, statErr)
}

// --- scaffoldAction with positional arg ---

func TestUT_ScaffoldAction_WithTemplateArg(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	templateDir := setupFakeLibrary(t, "action-tmpl")
	createMinimalTemplate(t, templateDir, "{{ vars.project_name }}")

	outputDir := t.TempDir()
	outputPath := filepath.Join(outputDir, "test-proj")

	ctx := newScaffoldActionCLIContext(t, []string{"action-tmpl", "test-proj"}, map[string]string{
		"no-input": "true",
		"output":   outputPath,
	})

	err := scaffoldAction(ctx)
	require.NoError(t, err)
}

// --- scaffoldFromRef with library template and generators ---

func TestUT_ScaffoldFromRef_LibraryWithGenerators(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	templateDir := setupFakeLibrary(t, "gen-tmpl")
	createMinimalTemplate(t, templateDir, "{{ vars.project_name }}")

	// Create generators directory in the template
	require.NoError(t, os.MkdirAll(filepath.Join(templateDir, types.GeneratorsDir), 0o755))

	outputDir := t.TempDir()
	outputPath := filepath.Join(outputDir, "gen-proj")

	ctx := newScaffoldActionCLIContext(t, nil, map[string]string{
		"no-input": "true",
		"output":   outputPath,
	})

	err := scaffoldFromRef(ctx, []string{"gen-tmpl", "gen-proj"})
	require.NoError(t, err)
}

// --- completeLibraryTemplateNames ---

func TestUT_CompleteLibraryTemplateNames_WithTemplates(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	setupFakeLibrary(t, "complete-tmpl")

	cliApp := &cli.App{Writer: io.Discard}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	ctx := cli.NewContext(cliApp, set, nil)

	// Should not panic; just prints to stdout (we don't capture output).
	completeLibraryTemplateNames(ctx)
}

func TestUT_CompleteLibraryTemplateNames_SkipsWhenHasArgs(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	setupFakeLibrary(t, "complete-tmpl2")

	cliApp := &cli.App{Writer: io.Discard}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	require.NoError(t, set.Parse([]string{"already-provided"}))
	ctx := cli.NewContext(cliApp, set, nil)

	// With an arg already provided, should not print anything.
	completeLibraryTemplateNames(ctx)
}

// --- completeGeneratorNames ---

func TestUT_CompleteGeneratorNames_NilConfig(t *testing.T) {
	t.Parallel()
	// Should not panic.
	completeGeneratorNames(nil)
}

func TestUT_CompleteGeneratorNames_WithLocalPath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	genDir := filepath.Join(tmpDir, "mygen")
	require.NoError(t, os.MkdirAll(genDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(genDir, "mygen.go"),
		[]byte("---\nto: {{ name }}.go\n---\npackage main\n"),
		0o644,
	))

	cfg := createTestConfig(t, tmpDir)
	// Should not panic; prints to stdout.
	completeGeneratorNames(cfg)
}

// Helper to create multiple fake library entries for testing.
func setupFakeLibraryMultiple(t *testing.T, names []string) string {
	t.Helper()

	dataDir := t.TempDir()

	entries := make(map[string]*library.Entry, len(names))
	for _, name := range names {
		templateDir := filepath.Join(dataDir, "templates", name)
		require.NoError(t, os.MkdirAll(templateDir, 0o750))
		entries[name] = &library.Entry{
			Name:      name,
			Source:    "gh:test/" + name,
			AddedAt:   time.Now(),
			UpdatedAt: time.Now(),
		}
	}

	reg := library.Registry{
		Version: 1,
		Entries: entries,
	}
	regData, err := json.Marshal(reg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "library.json"), regData, 0o644))

	orig := newLocalLibrary
	newLocalLibrary = func() (*library.Library, error) {
		return library.NewLocal(dataDir), nil
	}
	t.Cleanup(func() { newLocalLibrary = orig })

	return dataDir
}

// --- buildScaffoldOpts ---

func TestUT_BuildScaffoldOpts_PopulatesFields(t *testing.T) {
	t.Parallel()

	templateDir := t.TempDir()
	ctx := newScaffoldActionCLIContext(t, nil, map[string]string{
		"no-input": "true",
		"output":   "/tmp/out",
	})

	meta := map[string]string{"key": "val"}
	opts := buildScaffoldOpts(ctx, templateDir, "myproj", meta)

	assert.Equal(t, templateDir, opts.TemplateDir)
	assert.Equal(t, "myproj", opts.ProjectName)
	assert.Equal(t, "/tmp/out", opts.OutputDir)
	assert.True(t, opts.NoInput)
	assert.Equal(t, map[string]string{"key": "val"}, opts.Meta)
}

// --- scaffoldFlags ---

func TestUT_ScaffoldFlags_ContainsUpdateFlag(t *testing.T) {
	t.Parallel()

	sflags := scaffoldFlags()
	flagNames := make(map[string]bool)
	for _, f := range sflags {
		for _, n := range f.Names() {
			flagNames[n] = true
		}
	}

	assert.True(t, flagNames["update"], "scaffoldFlags should include --update")
	assert.True(t, flagNames["no-input"], "scaffoldFlags should include --no-input")
	assert.True(t, flagNames[flags.DryRunFlag], "scaffoldFlags should include --dry-run")
}
