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

func newScaffoldCLIContextExtra(t *testing.T, values map[string]string) *cli.Context {
	t.Helper()

	app := &cli.App{
		Writer: io.Discard,
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: flags.AddToLibFlag},
			&cli.BoolFlag{Name: "no-input"},
		},
	}
	set := flag.NewFlagSet("scaffold-test", flag.ContinueOnError)
	for _, f := range app.Flags {
		require.NoError(t, f.Apply(set))
	}
	for k, v := range values {
		require.NoError(t, set.Set(k, v))
	}
	require.NoError(t, set.Parse(nil))

	return cli.NewContext(app, set, nil)
}

func TestUT_ResolveAddToLib_AddFlagEnabled_ReturnsTrue(t *testing.T) {
	t.Parallel()

	templateDir := t.TempDir()
	ctx := newScaffoldCLIContextExtra(t, map[string]string{flags.AddToLibFlag: "true"})

	assert.True(t, resolveAddToLib(ctx, templateDir, false))
}

func TestUT_ResolveAddToLib_NoGenerators_ReturnsFalse(t *testing.T) {
	t.Parallel()

	templateDir := t.TempDir()
	ctx := newScaffoldCLIContextExtra(t, nil)

	assert.False(t, resolveAddToLib(ctx, templateDir, false))
}

func TestUT_ResolveAddToLib_NonInteractiveWithGenerators_ReturnsFalse(t *testing.T) {
	t.Parallel()

	templateDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(templateDir, types.GeneratorsDir), 0o755))
	ctx := newScaffoldCLIContextExtra(t, map[string]string{"no-input": "true"})

	assert.False(t, resolveAddToLib(ctx, templateDir, false))
}

// TestUT_ScaffoldNonInteractive_TruthTable does not use t.Parallel(): isTTY is
// a package-level var, and mutating it under a parallel sibling risks the
// same race the setupFakeLibrary lesson already documents for package vars.
func TestUT_ScaffoldNonInteractive_TruthTable(t *testing.T) {
	tests := []struct {
		name     string
		noInput  bool
		jsonMode bool
		tty      bool
		want     bool
	}{
		{"tty, text, no-input false -> interactive", false, false, true, false},
		{"no-input forces non-interactive", true, false, true, true},
		{"json format forces non-interactive despite tty", false, true, true, true},
		{"non-tty forces non-interactive", false, false, false, true},
		{"json format and non-tty", false, true, false, true},
		{"no-input and json format", true, true, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origIsTTY := isTTY
			isTTY = func() bool { return tt.tty }
			t.Cleanup(func() { isTTY = origIsTTY })

			values := map[string]string{}
			if tt.noInput {
				values["no-input"] = "true"
			}
			ctx := newScaffoldCLIContextExtra(t, values)

			assert.Equal(t, tt.want, nonInteractive(ctx, tt.jsonMode))
		})
	}
}

func TestUT_HandleCookiecutterDetection_NoInput_ReturnsHelpfulError(t *testing.T) {
	t.Parallel()

	ctx := newScaffoldCLIContextExtra(t, map[string]string{"no-input": "true"})
	err := handleCookiecutterDetection(
		ctx,
		&scaffold.CookiecutterDetectedError{CookiecutterPath: "/tmp/template"},
		"gh:acme/cookiecutter-api",
		t.TempDir(),
		scaffold.Options{},
		false,
		testVersion,
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Cannot convert in non-interactive mode")
	assert.Contains(t, err.Error(), "tag convert cookiecutter gh:acme/cookiecutter-api")
}

func TestUT_DisplayScaffoldSummary_WithGenerators_ShowsGenerateHint(t *testing.T) {
	t.Parallel()

	templateDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(templateDir, types.TemplatesDir), 0o755))

	result := scaffold.ScaffoldResult{
		OutputDir:   "/tmp/sample-app",
		ProjectRoot: "/tmp/sample-app",
		TemplateDir: templateDir,
		Vars: map[string]any{
			"project_name": "sample-app",
		},
		Opts: scaffold.Options{
			TemplateName: "sample-template",
			TemplateRef:  "./sample-template",
		},
	}

	var buf bytes.Buffer
	displayScaffoldSummary(&buf, result)

	output := buf.String()
	assert.Contains(t, output, "Template: ./sample-template")
	assert.Contains(t, output, "tag generate list")
}
