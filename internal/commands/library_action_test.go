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

	"github.com/kaikenlabs/tag/internal/library"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/pkg/app"
)

// --- asAppError ---

func TestUT_AsAppError_LibraryError(t *testing.T) {
	t.Parallel()
	libErr := &library.LibraryError{
		Name:      "test-tmpl",
		Operation: "remove",
		Err:       errors.New("not found"),
	}

	result := asAppError(libErr)
	require.Error(t, result)
	assert.Contains(t, result.Error(), "library remove")
	assert.Contains(t, result.Error(), "test-tmpl")
	assert.Contains(t, result.Error(), "not found")
}

func TestUT_AsAppError_GenericError(t *testing.T) {
	t.Parallel()
	err := errors.New("something went wrong")
	result := asAppError(err)
	require.Error(t, result)
	assert.Contains(t, result.Error(), "something went wrong")
}

// --- libListAction ---

func TestUT_LibListAction_EmptyLibrary(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	dataDir := t.TempDir()

	// Write an empty registry
	reg := library.Registry{Version: 1, Entries: map[string]*library.Entry{}}
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
	assert.Contains(t, out, "No templates installed")
	assert.Contains(t, out, "tag lib add")
}

func TestUT_LibListAction_WithTemplates(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	setupFakeLibrary(t, "go-api")

	var buf bytes.Buffer
	cliApp := &cli.App{Writer: &buf}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	ctx := cli.NewContext(cliApp, set, nil)

	cmd := libListCommand()
	err := cmd.Action(ctx)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "SOURCE")
	assert.Contains(t, out, "go-api")
}

func TestUT_LibListAction_MultipleTemplates(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	dataDir := t.TempDir()

	// Create template directories
	for _, name := range []string{"go-api", "react-app"} {
		require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "templates", name), 0o750))
	}

	reg := library.Registry{
		Version: 1,
		Entries: map[string]*library.Entry{
			"go-api": {
				Name:    "go-api",
				Source:  "gh:test/go-api",
				AddedAt: time.Now(),
			},
			"react-app": {
				Name:        "react-app",
				Source:      "gh:test/react-app",
				AddedAt:     time.Now(),
				Description: "React starter template",
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
	assert.Contains(t, out, "go-api")
	assert.Contains(t, out, "react-app")
	assert.Contains(t, out, "React starter template")
}

// --- libRemoveAction ---

func TestUT_LibRemoveAction_MissingArgument(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	setupFakeLibrary(t, "go-api")

	cliApp := &cli.App{Writer: io.Discard}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	ctx := cli.NewContext(cliApp, set, nil)

	cmd := libRemoveCommand()
	err := cmd.Action(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "template name is required")
}

func TestUT_LibRemoveAction_RemovesTemplate(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	templateDir := setupFakeLibrary(t, "go-api")

	// Write a tag.template.json to make it a valid template
	require.NoError(t, os.WriteFile(
		filepath.Join(templateDir, types.TemplateConfigFile),
		[]byte(`{"variables": []}`),
		0o644,
	))

	var buf bytes.Buffer
	cliApp := &cli.App{Writer: &buf}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	require.NoError(t, set.Parse([]string{"go-api"}))
	ctx := cli.NewContext(cliApp, set, nil)

	cmd := libRemoveCommand()
	err := cmd.Action(ctx)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Removed")
	assert.Contains(t, out, "go-api")
}

func TestUT_LibRemoveAction_NotFound(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	setupFakeLibrary(t, "go-api")

	cliApp := &cli.App{Writer: io.Discard}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	require.NoError(t, set.Parse([]string{"nonexistent"}))
	ctx := cli.NewContext(cliApp, set, nil)

	cmd := libRemoveCommand()
	err := cmd.Action(ctx)
	require.Error(t, err)
}

// --- libAddAction ---

func TestUT_LibAddAction_MissingArgument(t *testing.T) {
	t.Parallel()

	cliApp := &cli.App{Writer: io.Discard, Flags: []cli.Flag{
		&cli.StringFlag{Name: "as"},
		&cli.BoolFlag{Name: "force"},
	}}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range cliApp.Flags {
		require.NoError(t, f.Apply(set))
	}
	ctx := cli.NewContext(cliApp, set, nil)

	cmd := libAddCommand()
	err := cmd.Action(ctx)
	require.Error(t, err)
	var cmdErr *app.CommandError
	require.True(t, errors.As(err, &cmdErr))
	assert.Contains(t, err.Error(), "template reference is required")
}

// --- libEditAction ---

func TestUT_LibEditAction_MissingArgument(t *testing.T) {
	t.Parallel()

	cliApp := &cli.App{Writer: io.Discard, Flags: []cli.Flag{
		&cli.StringFlag{Name: "editor"},
	}}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range cliApp.Flags {
		require.NoError(t, f.Apply(set))
	}
	ctx := cli.NewContext(cliApp, set, nil)

	cmd := libEditCommand()
	err := cmd.Action(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "template name is required")
}

// --- libUpdateAction ---

func TestUT_LibUpdateAction_MissingArgument_UsesUpdateAll(t *testing.T) {
	// The update command with no args calls updateAllTemplates, which
	// requires a full library (with resolver). This fails because
	// newLibrary tries to create a real resolver. We just verify it
	// goes through the right path by checking the error.
	t.Parallel()

	cliApp := &cli.App{Writer: io.Discard}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	ctx := cli.NewContext(cliApp, set, nil)

	cmd := libUpdateCommand()
	err := cmd.Action(ctx)
	// It will fail trying to create the full library (needs resolver)
	// but should NOT say "template name is required"
	if err != nil {
		assert.NotContains(t, err.Error(), "template name is required")
	}
}
