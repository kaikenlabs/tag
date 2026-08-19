package commands

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/jsonout"
	"github.com/kaikenlabs/tag/pkg/app"
)

func TestUT_HumanList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		items []string
		want  string
	}{
		{nil, ""},
		{[]string{"text"}, "text"},
		{[]string{"text", "json"}, "text or json"},
		{[]string{"text", "json", "dot"}, "text, json, or dot"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, humanList(tt.items))
	}
}

func TestUT_FormatFlag_UsageDerivedFromAllowed(t *testing.T) {
	t.Parallel()

	f := formatFlag(formatText, formatJSON, formatDOT)
	assert.Equal(t, "format", f.Name)
	assert.Equal(t, formatText, f.Value, "default must be text so existing invocations are unchanged")
	assert.Equal(t, "Output format: text, json, or dot", f.Usage)
}

// newFormatContext builds a context with only the --format flag set, for
// exercising resolveFormat in isolation.
func newFormatContext(t *testing.T, value string) *cli.Context {
	t.Helper()

	set := flag.NewFlagSet("format-test", flag.ContinueOnError)
	if value != "" {
		require.NoError(t, formatFlag(formatText, formatJSON).Apply(set))
		require.NoError(t, set.Set("format", value))
	}
	require.NoError(t, set.Parse(nil))

	return cli.NewContext(&cli.App{}, set, nil)
}

func TestUT_ResolveFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		allowed []string
		want    string
		wantErr string
	}{
		{"default text", formatText, []string{formatText, formatJSON}, formatText, ""},
		{"json accepted", formatJSON, []string{formatText, formatJSON}, formatJSON, ""},
		{"dot accepted by graph", formatDOT, []string{formatText, formatJSON, formatDOT}, formatDOT, ""},
		{"dot rejected elsewhere", formatDOT, []string{formatText, formatJSON}, "", `unsupported format "dot" (use text or json)`},
		{"unknown rejected", "xml", []string{formatText, formatJSON}, "", `unsupported format "xml" (use text or json)`},
		// Contexts built by hand in tests never register the flag; treating the
		// zero value as an error would break every direct-action test in the package.
		{"unset falls back to text", "", []string{formatText, formatJSON}, formatText, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolveFormat(newFormatContext(t, tt.value), tt.allowed...)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)

				var cmdErr *app.CommandError
				require.ErrorAs(t, err, &cmdErr)
				assert.Equal(t, app.ExitUsage, cmdErr.Code, "unknown format is a usage error, not a general one")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestUT_TrailingFormatFlag_IsParsed is the regression test for the
// positional-flag bug: urfave/cli v2 stops parsing at the first non-flag token,
// so `tag template lint ./path --format json` used to silently ignore --format.
//
// It must run through a real cli.App. A hand-built flag.FlagSet cannot show the
// bug, because it hands the action a context in which the flag is already set —
// which is exactly why every existing test in this package missed it.
//
// The assertion uses a deliberately invalid format: if the flag reaches the
// action, the run fails with a usage error naming it; if it was swallowed as a
// positional, it does not.
func TestUT_TrailingFormatFlag_IsParsed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), []byte(validTemplateConfig), 0o600))

	tests := []struct {
		name string
		cmd  func() *cli.Command
		argv []string
	}{
		{"template lint", templateLintCommand, []string{"lint", dir, "--format", "xml"}},
		{"template variables", templateVariablesCommand, []string{"variables", dir, "--format", "xml"}},
		{"template graph", templateGraphCommand, []string{"graph", dir, "--format", "xml"}},
		{"test", TestCommand, []string{"test", dir, "--format", "xml"}},
		{"dialect show", dialectShowCommand, []string{"show", "go", "--format", "xml"}},
		{"lib search", libSearchCommand, []string{"search", "foo", "--format", "xml"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			run := runCLI(t, tt.cmd(), tt.argv...)
			require.Error(t, run.Err, "trailing --format was ignored")
			assert.Contains(t, run.Err.Error(), `unsupported format "xml"`)
		})
	}
}

// TestUT_LeadingFormatFlag_StillWorks pins the ordering that already worked, so
// the reparse fix cannot regress it.
func TestUT_LeadingFormatFlag_StillWorks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), []byte(validTemplateConfig), 0o600))

	run := runCLI(t, templateLintCommand(), "lint", "--format", formatJSON, dir)
	require.NoError(t, run.Err)
	assert.Empty(t, run.Stdout, "JSON mode must not write to os.Stdout directly")

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(run.Writer), &parsed))
}

// TestUT_TrailingFormatFlag_JSONReachesWriter proves the fix produces real JSON,
// not merely a different error.
func TestUT_TrailingFormatFlag_JSONReachesWriter(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), []byte(validTemplateConfig), 0o600))

	run := runCLI(t, templateLintCommand(), "lint", dir, "--format", formatJSON)
	require.NoError(t, run.Err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(run.Writer), &parsed), "expected JSON, got: %s", run.Writer)
}

func TestUT_ReparseTrailingFlags_FlagKinds(t *testing.T) {
	t.Parallel()

	cliFlags := []cli.Flag{
		&cli.StringFlag{Name: "format", Value: formatText},
		&cli.IntFlag{Name: "limit", Value: 10},
		&cli.BoolFlag{Name: "strict"},
		&cli.StringSliceFlag{Name: "meta", Aliases: []string{"m"}},
	}

	tests := []struct {
		name           string
		args           []string
		wantPositional []string
		wantErr        string
		check          func(t *testing.T, c *cli.Context)
	}{
		{
			name:           "value flag after positional",
			args:           []string{"query", "--limit", "5"},
			wantPositional: []string{"query"},
			check: func(t *testing.T, c *cli.Context) {
				t.Helper()
				assert.Equal(t, 5, c.Int("limit"))
			},
		},
		{
			name:           "equals form",
			args:           []string{"query", "--format=json"},
			wantPositional: []string{"query"},
			check: func(t *testing.T, c *cli.Context) {
				t.Helper()
				assert.Equal(t, formatJSON, c.String("format"))
			},
		},
		{
			name:           "bool flag consumes no value",
			args:           []string{"query", "--strict", "second"},
			wantPositional: []string{"query", "second"},
			check: func(t *testing.T, c *cli.Context) {
				t.Helper()
				assert.True(t, c.Bool("strict"))
			},
		},
		{
			name:           "repeated slice flag via alias",
			args:           []string{"query", "-m", "a=1", "-m", "b=2"},
			wantPositional: []string{"query"},
			check: func(t *testing.T, c *cli.Context) {
				t.Helper()
				assert.Equal(t, []string{"a=1", "b=2"}, c.StringSlice("meta"))
			},
		},
		{
			// Only reachable when "--" follows a positional: for `cmd -- -x`
			// urfave/cli consumes the "--" before the action ever runs.
			name:           "double dash after a positional passes the rest through",
			args:           []string{"query", "--", "-language:go", "--limit"},
			wantPositional: []string{"query", "-language:go", "--limit"},
		},
		{
			name:    "value flag with no value",
			args:    []string{"query", "--limit"},
			wantErr: "requires a value",
		},
		{
			name:    "unknown flag is an error, not a positional",
			args:    []string{"query", "-nope"},
			wantErr: "unknown flag -nope",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			set := flag.NewFlagSet("reparse-test", flag.ContinueOnError)
			for _, f := range cliFlags {
				require.NoError(t, f.Apply(set))
			}
			require.NoError(t, set.Parse(tt.args))
			c := cli.NewContext(&cli.App{}, set, nil)

			positional, err := reparseTrailingFlags(c, cliFlags)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantPositional, positional)
			if tt.check != nil {
				tt.check(t, c)
			}
		})
	}
}

func TestUT_JSONOut_Write(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, jsonout.Write(&buf, map[string]any{"b": 1, "a": []string{}}))

	assert.Equal(t, "{\n  \"a\": [],\n  \"b\": 1\n}\n", buf.String(),
		"two-space indent and a single trailing newline are the wire contract")
}

func TestUT_CmdOut_FallsBackToStdout(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	withWriter := cli.NewContext(&cli.App{Writer: &buf}, flag.NewFlagSet("x", flag.ContinueOnError), nil)
	assert.Same(t, &buf, cmdOut(withWriter))

	noWriter := cli.NewContext(&cli.App{}, flag.NewFlagSet("x", flag.ContinueOnError), nil)
	assert.Same(t, os.Stdout, cmdOut(noWriter))
}
