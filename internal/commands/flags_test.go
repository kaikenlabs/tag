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
	testFlags := func() []cli.Flag {
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
			flags := testFlags()
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
