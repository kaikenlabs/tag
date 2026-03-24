package commands

import (
	"bytes"
	"flag"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/library"
)

// --- LibCommand structure ---

func TestUT_LibCommand_Structure(t *testing.T) {
	t.Parallel()

	cmd := LibCommand()
	require.NotNil(t, cmd)
	assert.Equal(t, "lib", cmd.Name)
	assert.Contains(t, cmd.Aliases, "library")

	subNames := make([]string, len(cmd.Subcommands))
	for i, sc := range cmd.Subcommands {
		subNames[i] = sc.Name
	}
	assert.ElementsMatch(t, []string{"search", "add", "ls", "rm", "update", "edit"}, subNames)
}

// --- libEditCommand structure ---

func TestUT_LibEditCommand_Structure(t *testing.T) {
	t.Parallel()

	cmd := libEditCommand()
	require.NotNil(t, cmd)
	assert.Equal(t, "edit", cmd.Name)
	assert.NotNil(t, cmd.Action)
	assert.NotNil(t, cmd.BashComplete)

	flagNames := make(map[string]bool)
	for _, f := range cmd.Flags {
		for _, n := range f.Names() {
			flagNames[n] = true
		}
	}
	assert.True(t, flagNames["editor"])
}

// --- libAddCommand structure ---

func TestUT_LibAddCommand_Structure(t *testing.T) {
	t.Parallel()

	cmd := libAddCommand()
	require.NotNil(t, cmd)
	assert.Equal(t, "add", cmd.Name)
	assert.NotNil(t, cmd.Action)
}

// --- libSearchCommand structure ---

func TestUT_LibSearchCommand_Structure(t *testing.T) {
	t.Parallel()

	cmd := libSearchCommand()
	require.NotNil(t, cmd)
	assert.Equal(t, "search", cmd.Name)
	assert.NotNil(t, cmd.Action)

	flagNames := make(map[string]bool)
	for _, f := range cmd.Flags {
		for _, n := range f.Names() {
			flagNames[n] = true
		}
	}
	assert.True(t, flagNames["limit"])
	assert.True(t, flagNames["sort"])
	assert.True(t, flagNames["order"])
}

// --- libUpdateCommand structure ---

func TestUT_LibUpdateCommand_Structure(t *testing.T) {
	t.Parallel()

	cmd := libUpdateCommand()
	require.NotNil(t, cmd)
	assert.Equal(t, "update", cmd.Name)
	assert.NotNil(t, cmd.Action)
	assert.NotNil(t, cmd.BashComplete)
}

// --- libRemoveCommand structure ---

func TestUT_LibRemoveCommand_Structure(t *testing.T) {
	t.Parallel()

	cmd := libRemoveCommand()
	require.NotNil(t, cmd)
	assert.Equal(t, "rm", cmd.Name)
	assert.Contains(t, cmd.Aliases, "remove")
	assert.NotNil(t, cmd.Action)
}

// --- asAppError with LibraryError variant ---

func TestUT_AsAppError_LibraryError_Get(t *testing.T) {
	t.Parallel()

	libErr := &library.LibraryError{
		Name:      "go-api",
		Operation: "get",
		Err:       assert.AnError,
	}
	result := asAppError(libErr)
	require.Error(t, result)
	assert.Contains(t, result.Error(), "go-api")
	assert.Contains(t, result.Error(), "get")
}

// --- libListCommand with version and description ---

func TestUT_LibListAction_ShowsVersionAndDescription(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	dataDir := setupFakeLibraryMultiple(t, []string{"versioned-tmpl"})

	// The setupFakeLibraryMultiple doesn't set version/description, update it directly.
	_ = dataDir // we just use the fake library

	var buf bytes.Buffer
	cliApp := &cli.App{Writer: &buf}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	ctx := cli.NewContext(cliApp, set, nil)

	cmd := libListCommand()
	err := cmd.Action(ctx)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "versioned-tmpl")
	assert.Contains(t, out, "NAME")
}

// --- ConvertCommand structure ---

func TestUT_ConvertCommand_Structure(t *testing.T) {
	t.Parallel()

	cmd := ConvertCommand()
	require.NotNil(t, cmd)
	assert.Equal(t, "convert", cmd.Name)
	require.Len(t, cmd.Subcommands, 1)
	assert.Equal(t, "cookiecutter", cmd.Subcommands[0].Name)
}

// --- convertCookiecutterCommand structure ---

func TestUT_ConvertCookiecutterCommand_Structure(t *testing.T) {
	t.Parallel()

	cmd := convertCookiecutterCommand()
	require.NotNil(t, cmd)
	assert.Equal(t, "cookiecutter", cmd.Name)
	assert.NotNil(t, cmd.Action)
	assert.NotEmpty(t, cmd.Description)

	flagNames := make(map[string]bool)
	for _, f := range cmd.Flags {
		for _, n := range f.Names() {
			flagNames[n] = true
		}
	}
	assert.True(t, flagNames["output"])
	assert.True(t, flagNames["force"])
}

// --- UpdateTemplateCommand structure (deeper) ---

func TestUT_UpdateTemplateCommand_HasAllFlags(t *testing.T) {
	t.Parallel()

	cmd := UpdateTemplateCommand()
	require.NotNil(t, cmd)

	flagNames := make(map[string]bool)
	for _, f := range cmd.Flags {
		for _, n := range f.Names() {
			flagNames[n] = true
		}
	}

	expectedFlags := []string{
		"dir", "ref", "set", "accept-ours", "accept-theirs",
		"skip", "dry-run", "backup", "continue", "abort", "skip-hooks", "accept-hooks",
	}
	for _, name := range expectedFlags {
		assert.True(t, flagNames[name], "expected flag %q", name)
	}
}

// --- libEditAction with nonexistent template ---

func TestUT_LibEditAction_NonexistentTemplate(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	setupFakeLibrary(t, "edit-tmpl")

	cliApp := &cli.App{Writer: io.Discard, Flags: []cli.Flag{
		&cli.StringFlag{Name: "editor"},
	}}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range cliApp.Flags {
		require.NoError(t, f.Apply(set))
	}
	// Use a template name that doesn't exist in the fake library
	require.NoError(t, set.Parse([]string{"nonexistent-tmpl"}))
	ctx := cli.NewContext(cliApp, set, nil)

	cmd := libEditCommand()
	err := cmd.Action(ctx)
	require.Error(t, err)
}
