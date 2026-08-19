package commands

import (
	"bytes"
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/dialect"
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

// TestUT_WriteDialectJSON_NilTypesSerializeAsEmptyObject pins the substitution
// in writeDialectJSON: a dialect with a nil Types map must serialise its
// "types" key as {} on the wire, not null. A struct-level assertion cannot
// prove this — both `null` and `{}` unmarshal to a nil Go map, so decoding the
// output back into a struct and comparing would pass whether or not the
// substitution exists. The assertion below is against the raw JSON bytes.
func TestUT_WriteDialectJSON_NilTypesSerializeAsEmptyObject(t *testing.T) {
	t.Parallel()

	d := &dialect.Dialect{Name: "empty", Types: nil}
	require.Nil(t, d.Types, "precondition: the dialect under test must have a nil Types map")

	var buf bytes.Buffer
	require.NoError(t, writeDialectJSON(&buf, d))

	raw := buf.String()
	assert.Contains(t, raw, `"types": {}`)
	assert.NotContains(t, raw, `"types": null`)
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
