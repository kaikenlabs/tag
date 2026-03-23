package commands

import (
	"bytes"
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestUT_DialectCommand_Structure(t *testing.T) {
	t.Parallel()

	cmd := DialectCommand()
	require.NotNil(t, cmd)

	assert.Equal(t, "dialect", cmd.Name)
	assert.Contains(t, cmd.Aliases, "dialects")
	require.Len(t, cmd.Subcommands, 2)

	subNames := []string{cmd.Subcommands[0].Name, cmd.Subcommands[1].Name}
	assert.Contains(t, subNames, "list")
	assert.Contains(t, subNames, "show")
}

func TestUT_DialectShowCommand_MissingArg(t *testing.T) {
	t.Parallel()

	cmd := dialectShowCommand()
	app := &cli.App{Writer: &bytes.Buffer{}}
	set := flag.NewFlagSet("dialect-show", flag.ContinueOnError)
	require.NoError(t, set.Parse(nil))

	ctx := cli.NewContext(app, set, nil)
	err := cmd.Action(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing dialect name")
}

func TestUT_DialectShowCommand_UnknownDialect(t *testing.T) {
	t.Parallel()

	cmd := dialectShowCommand()
	app := &cli.App{Writer: &bytes.Buffer{}}
	set := flag.NewFlagSet("dialect-show-unknown", flag.ContinueOnError)
	require.NoError(t, set.Parse([]string{"does-not-exist"}))

	ctx := cli.NewContext(app, set, nil)
	err := cmd.Action(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown dialect")
}

func TestUT_DialectListCommand_ActionWritesList(t *testing.T) {
	t.Parallel()

	cmd := dialectListCommand()
	var out bytes.Buffer
	app := &cli.App{Writer: &out}
	set := flag.NewFlagSet("dialect-list", flag.ContinueOnError)
	require.NoError(t, set.Parse(nil))

	ctx := cli.NewContext(app, set, nil)
	err := cmd.Action(ctx)
	require.NoError(t, err)

	text := out.String()
	assert.Contains(t, text, "NAME")
	assert.Contains(t, text, "go")
}
