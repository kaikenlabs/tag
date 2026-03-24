package commands

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/internal/types/flags"
)

// newScaffoldActionCLIContext creates a CLI context with all scaffold-related flags registered.
func newScaffoldActionCLIContext(t *testing.T, args []string, flagValues map[string]string) *cli.Context {
	t.Helper()

	cliFlags := scaffoldFlags()
	// Add flags not already in scaffoldFlags() but needed by tests
	cliFlags = append(cliFlags,
		&cli.StringFlag{Name: flags.PathFlag, Value: ".tag"},
		&cli.StringFlag{Name: flags.SharedPathFlag, Value: "_shared"},
		&cli.StringFlag{Name: flags.BundlePathFlag, Value: "_bundles"},
		&cli.BoolFlag{Name: flags.VerboseFlag},
		&cli.BoolFlag{Name: flags.NoHooksFlag},
	)

	cliApp := &cli.App{Writer: io.Discard, Flags: cliFlags}
	set := flag.NewFlagSet("scaffold-action-test", flag.ContinueOnError)
	for _, f := range cliFlags {
		require.NoError(t, f.Apply(set))
	}
	for k, v := range flagValues {
		require.NoError(t, set.Set(k, v))
	}
	require.NoError(t, set.Parse(args))

	return cli.NewContext(cliApp, set, nil)
}

// createMinimalTemplate creates a minimal valid TAG template directory.
func createMinimalTemplate(t *testing.T, dir, projectDirName string) {
	t.Helper()

	wrapperDir := filepath.Join(dir, projectDirName)
	require.NoError(t, os.MkdirAll(wrapperDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(wrapperDir, "README.md"),
		[]byte("# {{ vars.project_name }}"),
		0o644,
	))

	tmplConfig := `{
  "name": "test-template",
  "vars": {
    "project_name": {
      "type": "string",
      "default": "my-project",
      "prompt": "Project name"
    }
  }
}`
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, types.TemplateConfigFile),
		[]byte(tmplConfig),
		0o644,
	))
}

func TestUT_ScaffoldFromRef_LocalTemplateDir(t *testing.T) {
	templateDir := t.TempDir()
	createMinimalTemplate(t, templateDir, "{{ vars.project_name }}")

	outputDir := t.TempDir()
	outputPath := filepath.Join(outputDir, "test-project")

	ctx := newScaffoldActionCLIContext(t, nil, map[string]string{
		"no-input": "true",
		"output":   outputPath,
	})

	err := scaffoldFromRef(ctx, []string{templateDir, "test-project"})
	require.NoError(t, err)

	_, statErr := os.Stat(outputPath)
	assert.NoError(t, statErr, "expected output directory to be created")
}

func TestUT_ScaffoldFromRef_LocalTemplateDir_NoInput(t *testing.T) {
	templateDir := t.TempDir()
	createMinimalTemplate(t, templateDir, "{{ vars.project_name }}")

	outputDir := t.TempDir()
	outputPath := filepath.Join(outputDir, "my-project")

	ctx := newScaffoldActionCLIContext(t, nil, map[string]string{
		"no-input": "true",
		"output":   outputPath,
	})

	err := scaffoldFromRef(ctx, []string{templateDir})
	require.NoError(t, err)

	_, statErr := os.Stat(outputPath)
	assert.NoError(t, statErr, "expected output directory to be created with default name")
}

func TestUT_ScaffoldFromRef_LibraryTemplate_WithOutput(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	templateDir := setupFakeLibrary(t, "my-go-api")
	createMinimalTemplate(t, templateDir, "{{ vars.project_name }}")

	outputDir := t.TempDir()
	outputPath := filepath.Join(outputDir, "test-api")

	ctx := newScaffoldActionCLIContext(t, nil, map[string]string{
		"no-input": "true",
		"output":   outputPath,
	})

	err := scaffoldFromRef(ctx, []string{"my-go-api", "test-api"})
	require.NoError(t, err)

	_, statErr := os.Stat(outputPath)
	assert.NoError(t, statErr, "expected output directory to be created")
}

func TestUT_ScaffoldFromRef_InvalidLocalPath(t *testing.T) {
	t.Parallel()

	ctx := newScaffoldActionCLIContext(t, nil, map[string]string{
		"no-input": "true",
	})

	err := scaffoldFromRef(ctx, []string{"./nonexistent-template-dir"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve template")
}

func TestUT_DisplayScaffoldSummary_WithReadme(t *testing.T) {
	t.Parallel()

	templateDir := t.TempDir()
	readmeContent := "# My Template\n\nThis is a great template.\n"
	require.NoError(t, os.WriteFile(
		filepath.Join(templateDir, types.TemplateReadme),
		[]byte(readmeContent),
		0o644,
	))

	result := scaffold.ScaffoldResult{
		OutputDir:   "/tmp/my-output",
		TemplateDir: templateDir,
		Vars:        map[string]any{"project_name": "my-output"},
		Opts:        scaffold.Options{},
	}

	var buf bytes.Buffer
	displayScaffoldSummary(&buf, result)

	output := buf.String()
	assert.Contains(t, output, "Scaffolding complete!")
	assert.Contains(t, output, "My Template")
}

func TestUT_ScaffoldAction_NoInput_NoArgs(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	setupFakeLibrary(t, "some-template")

	ctx := newScaffoldActionCLIContext(t, nil, map[string]string{
		"no-input": "true",
	})

	err := scaffoldAction(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "template argument required")
}

func TestUT_PromptForConversion_Declined(t *testing.T) {
	t.Parallel()

	mockPrompter := &mockPrompterForScaffold{
		confirmResult: false,
	}

	_, err := promptForConversion(mockPrompter, "gh:user/cookiecutter-api")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Conversion declined")
}

func TestUT_PromptForProjectDir_AlreadySet(t *testing.T) {
	t.Parallel()

	opts := &scaffold.Options{ProjectName: "existing"}
	err := promptForProjectDir(nil, opts)
	require.NoError(t, err)
}

func TestUT_PromptForProjectDir_OutputDirSet(t *testing.T) {
	t.Parallel()

	opts := &scaffold.Options{OutputDir: "/tmp/output"}
	err := promptForProjectDir(nil, opts)
	require.NoError(t, err)
}

// mockPrompterForScaffold implements scaffold.Prompter for testing.
type mockPrompterForScaffold struct {
	confirmResult bool
	confirmErr    error
	inputResult   string
	inputErr      error
}

func (m *mockPrompterForScaffold) Confirm(_ string, _ bool) (bool, error) {
	if m.confirmErr != nil {
		return false, m.confirmErr
	}
	return m.confirmResult, nil
}

func (m *mockPrompterForScaffold) Input(_, defaultVal string, _ bool) (string, error) {
	if m.inputErr != nil {
		return "", m.inputErr
	}
	if m.inputResult != "" {
		return m.inputResult, nil
	}
	return defaultVal, nil
}

func (m *mockPrompterForScaffold) Select(_ string, _ []string, _ int) (string, error) {
	return "", nil
}

func (m *mockPrompterForScaffold) Number(_ string, defaultVal float64) (float64, error) {
	return defaultVal, nil
}
