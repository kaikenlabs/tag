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
	// Uses t.Setenv — do NOT use t.Parallel.
	//
	// `lib update` with no argument routes to updateAllTemplates, which builds
	// a library via newLibrary() -> xdg.DataHome(). Without XDG_DATA_HOME
	// pointed somewhere disposable this test resolves the developer's REAL
	// library, re-fetches every installed template over the network, and
	// rewrites their registry — on every `go test ./...`. The original comment
	// here assumed the call would fail before doing anything; it does not.
	dataDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataDir)

	// An empty registry keeps UpdateAll offline: there is nothing to re-fetch.
	libDir := filepath.Join(dataDir, "tag")
	require.NoError(t, os.MkdirAll(libDir, 0o750))
	regData, err := json.Marshal(library.Registry{Version: 1, Entries: map[string]*library.Entry{}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(libDir, "library.json"), regData, 0o600))

	var buf bytes.Buffer
	cliApp := &cli.App{Writer: &buf}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	ctx := cli.NewContext(cliApp, set, nil)

	// An empty library is rejected by UpdateAll itself. That specific error is
	// the proof we want: the single-template path would have complained that a
	// template name is required, and never reached the library at all.
	err = libUpdateCommand().Action(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "library is empty")
	assert.NotContains(t, err.Error(), "template name is required")
	assert.Empty(t, buf.String())
}
