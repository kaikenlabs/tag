package commands

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

// newTestContext creates a cli.Context with the given flags registered and args parsed.
// Flags are parsed first (Go's flag package stops at the first non-flag), so all args
// after the first positional arg become c.Args() — exactly the scenario we need to test.
func newTestContext(t *testing.T, flags []cli.Flag, args []string) *cli.Context {
	t.Helper()

	app := &cli.App{Flags: flags}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range flags {
		require.NoError(t, f.Apply(set))
	}
	// Parse: Go stops at the first non-flag token, so trailing flags end up in Args().
	require.NoError(t, set.Parse(args))
	return cli.NewContext(app, set, nil)
}

func TestUT_ReparseTrailingFlags(t *testing.T) {
	// Shared flag definitions used across tests.
	reparseFlags := func() []cli.Flag {
		return []cli.Flag{
			&cli.StringSliceFlag{Name: "meta", Aliases: []string{"m"}},
			&cli.BoolFlag{Name: "no-input"},
			&cli.BoolFlag{Name: "accept-hooks"},
			&cli.BoolFlag{Name: "force", Aliases: []string{"f"}},
			&cli.StringFlag{Name: "output", Aliases: []string{"o"}},
			&cli.BoolFlag{Name: "no-save"},
		}
	}

	tests := []struct {
		name            string
		args            []string // raw args (simulating what the user typed)
		wantPositional  []string
		wantMeta        []string
		wantNoInput     bool
		wantAcceptHooks bool
		wantForce       bool
		wantOutput      string
		wantErr         string
	}{
		{
			name:            "flags after positional args",
			args:            []string{"my-template", "my-project", "-m", "key=val", "--no-input", "--accept-hooks"},
			wantPositional:  []string{"my-template", "my-project"},
			wantMeta:        []string{"key=val"},
			wantNoInput:     true,
			wantAcceptHooks: true,
		},
		{
			name:           "flags before positional args (already parsed by Go)",
			args:           []string{"-m", "key=val", "--no-input", "my-template"},
			wantPositional: []string{"my-template"},
			wantMeta:       []string{"key=val"},
			wantNoInput:    true,
		},
		{
			name:            "mixed flags and positional args",
			args:            []string{"my-template", "-m", "a=1", "my-project", "--no-input", "--accept-hooks"},
			wantPositional:  []string{"my-template", "my-project"},
			wantMeta:        []string{"a=1"},
			wantNoInput:     true,
			wantAcceptHooks: true,
		},
		{
			name:           "equals syntax",
			args:           []string{"my-template", "--output=/tmp/out", "--force"},
			wantPositional: []string{"my-template"},
			wantOutput:     "/tmp/out",
			wantForce:      true,
		},
		{
			name:           "multiple meta flags",
			args:           []string{"tpl", "proj", "-m", "a=1", "-m", "b=2"},
			wantPositional: []string{"tpl", "proj"},
			wantMeta:       []string{"a=1", "b=2"},
		},
		{
			name:           "short alias for boolean flag",
			args:           []string{"tpl", "-f"},
			wantPositional: []string{"tpl"},
			wantForce:      true,
		},
		{
			name:           "short alias for value flag",
			args:           []string{"tpl", "-o", "/tmp/x"},
			wantPositional: []string{"tpl"},
			wantOutput:     "/tmp/x",
		},
		{
			name:           "double-dash stops flag parsing",
			args:           []string{"tpl", "--", "--no-input", "-m", "x=y"},
			wantPositional: []string{"tpl", "--no-input", "-m", "x=y"},
		},
		{
			name:           "no trailing flags (positional only)",
			args:           []string{"tpl", "proj"},
			wantPositional: []string{"tpl", "proj"},
		},
		{
			name:    "value flag missing argument",
			args:    []string{"tpl", "-o"},
			wantErr: "flag -o requires a value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := reparseFlags()
			c := newTestContext(t, flags, tt.args)

			positional, err := reparseTrailingFlags(c, flags)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantPositional, positional)

			if len(tt.wantMeta) > 0 {
				assert.Equal(t, tt.wantMeta, c.StringSlice("meta"))
			}
			if tt.wantNoInput {
				assert.True(t, c.Bool("no-input"))
			}
			if tt.wantAcceptHooks {
				assert.True(t, c.Bool("accept-hooks"))
			}
			if tt.wantForce {
				assert.True(t, c.Bool("force"))
			}
			if tt.wantOutput != "" {
				assert.Equal(t, tt.wantOutput, c.String("output"))
			}
		})
	}
}

// TestUT_ReparseTrailingFlags_AppliesAppLevelGlobalFlag pins the fix for a
// silent-ignore bug that adding --format to `generate` would otherwise have
// turned into a hard error.
//
// `--dry-run`, `--path`, `--shared-path` and `--bundle-path` are declared on
// the cli.App in main.go, not on any command. urfave/cli stops parsing at the
// first positional, so `tag generate model User --dry-run` silently DROPPED
// --dry-run before this change: nothing rescanned the tail, and the value was
// left sitting in c.Args(). Feeding the reparser only the command's own flags
// would have made the same invocation fail with "unknown flag -dry-run", which
// is louder but still wrong.
//
// Context.Set already resolves a flag through the whole lineage
// (context.go lookupFlagSet), so the only thing missing was the lookup table:
// the reparser now also registers c.App.Flags.
func TestUT_ReparseTrailingFlags_AppliesAppLevelGlobalFlag(t *testing.T) {
	globalFlags := []cli.Flag{
		&cli.BoolFlag{Name: "dry-run", Aliases: []string{"d"}},
		&cli.StringFlag{Name: "path", Aliases: []string{"p"}},
	}
	cmdFlags := []cli.Flag{
		&cli.StringFlag{Name: "format", Value: "text"},
	}

	app := &cli.App{Flags: globalFlags}
	globalSet := flag.NewFlagSet("global", flag.ContinueOnError)
	for _, f := range globalFlags {
		require.NoError(t, f.Apply(globalSet))
	}
	require.NoError(t, globalSet.Parse(nil))
	parent := cli.NewContext(app, globalSet, nil)

	cmdSet := flag.NewFlagSet("cmd", flag.ContinueOnError)
	for _, f := range cmdFlags {
		require.NoError(t, f.Apply(cmdSet))
	}
	require.NoError(t, cmdSet.Parse([]string{"model", "User", "--dry-run", "--path", ".tag", "--format", "json"}))
	c := cli.NewContext(app, cmdSet, parent)

	positional, err := reparseTrailingFlags(c, cmdFlags)
	require.NoError(t, err)

	assert.Equal(t, []string{"model", "User"}, positional)
	assert.True(t, c.Bool("dry-run"), "trailing global --dry-run must be applied, not dropped and not rejected")
	assert.Equal(t, ".tag", c.String("path"), "trailing global --path must consume its value")
	assert.Equal(t, "json", c.String("format"), "the command's own trailing flag must still work")
}

// A dash-prefixed token that matches neither the command's flags nor the App's
// globals must still be an error — the fix widens the lookup table, it does not
// make the reparser permissive.
func TestUT_ReparseTrailingFlags_UnknownFlagStillRejectedWithGlobals(t *testing.T) {
	app := &cli.App{Flags: []cli.Flag{&cli.BoolFlag{Name: "dry-run"}}}
	cmdFlags := []cli.Flag{&cli.StringFlag{Name: "format", Value: "text"}}

	cmdSet := flag.NewFlagSet("cmd", flag.ContinueOnError)
	for _, f := range cmdFlags {
		require.NoError(t, f.Apply(cmdSet))
	}
	require.NoError(t, cmdSet.Parse([]string{"model", "--nope"}))
	c := cli.NewContext(app, cmdSet, nil)

	_, err := reparseTrailingFlags(c, cmdFlags)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown flag -nope")
}
