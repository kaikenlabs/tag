package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/pkg/app"
)

// The --format conformance suite. It derives its command list from the
// registered command tree, so a future command that gains --format is checked
// automatically and one that quietly drops it shows up as a census diff.
//
// The rejection half needs no fixtures at all: resolveFormat runs before a
// command validates its own inputs, so `--format bogus` is a usage error even
// against a nonexistent template in an empty directory. That property is what
// keeps this test from turning into a fixture farm, and it is asserted
// directly by TestUT_FormatConformance_RejectionNeedsNoFixture.

// formatCommand is one --format-capable command plus whether it accepts
// positional arguments. The arity matters: probing a command that takes none
// with a positional makes urfave/cli look for a subcommand of that name and
// fail with "No help topic" long before resolveFormat runs, which would
// assert nothing about --format.
type formatCommand struct {
	path            []string
	takesPositional bool
}

func (f formatCommand) name() string { return strings.Join(f.path, " ") }

// formatCommands walks RootCommands for every command advertising a --format
// flag, using the same traversal as the trailing-flag guard's census.
func formatCommands(t *testing.T) []formatCommand {
	t.Helper()

	cfg := createTestConfig(t, t.TempDir())
	root := RootCommands(cfg, "test", SkillDocs{})

	var found []formatCommand
	walkCommandTree(root, func(cmd *cli.Command, path []string) {
		if cmd.Action == nil {
			return
		}
		for _, f := range cmd.Flags {
			if slices.Contains(f.Names(), "format") {
				found = append(found, formatCommand{
					path:            path,
					takesPositional: cmd.Args || cmd.ArgsUsage != "",
				})
				return
			}
		}
	})
	return found
}

func newFormatApp(t *testing.T) (*cli.App, *bytes.Buffer) {
	t.Helper()

	var out bytes.Buffer
	cfg := createTestConfig(t, t.TempDir())
	return &cli.App{
		Writer:         &out,
		ErrWriter:      io.Discard,
		Flags:          GlobalFlags(),
		Commands:       RootCommands(cfg, "test", SkillDocs{}),
		ExitErrHandler: func(*cli.Context, error) {},
	}, &out
}

// assertUsageError accepts either a *app.CommandError carrying ExitUsage or a
// cli.ExitCoder reporting 2: commands reach the same outcome by both routes,
// and the conformance claim is about the exit code the user sees.
func assertUsageError(t *testing.T, err error, argv []string) {
	t.Helper()

	require.Error(t, err, "%v must reject an unknown --format value", argv)

	var cmdErr *app.CommandError
	if errors.As(err, &cmdErr) {
		assert.Equal(t, app.ExitUsage, cmdErr.Code,
			"%v: unknown format is a usage error, not a general one", argv)
		return
	}

	var coder cli.ExitCoder
	if errors.As(err, &coder) {
		assert.Equal(t, app.ExitUsage, coder.ExitCode(), "%v", argv)
		return
	}

	t.Fatalf("%v: error %v carries no exit code", argv, err)
}

func TestUT_AllFormatCommands_RejectUnknownValue(t *testing.T) {
	cmds := formatCommands(t)
	require.NotEmpty(t, cmds)

	for _, fc := range cmds {
		t.Run(fc.name(), func(t *testing.T) {
			base := append([]string{"tag"}, fc.path...)

			// A command taking positionals is probed in both orders: a
			// trailing --format is invisible to urfave/cli until
			// reparseTrailingFlags runs, so that row is the one that
			// regresses when a command forgets the reparse. A command taking
			// none is probed only in the order it can actually be invoked.
			argvs := [][]string{append(append([]string{}, base...), "--format", "bogus")}
			if fc.takesPositional {
				argvs[0] = append(append([]string{}, base...), "--format", "bogus", "conformance-probe")
				argvs = append(argvs, append(append([]string{}, base...), "conformance-probe", "--format", "bogus"))
			}

			for _, argv := range argvs {
				t.Chdir(t.TempDir())
				a, _ := newFormatApp(t)
				assertUsageError(t, a.Run(argv), argv)
			}
		})
	}
}

func TestUT_FormatConformance_RejectionNeedsNoFixture(t *testing.T) {
	// The load-bearing property behind the zero-fixture design above: format
	// validation happens before a command validates its own inputs. If a
	// future refactor moves resolveFormat after input validation, the census
	// test keeps passing while the rejection rows start reporting whatever
	// error the missing fixture produced instead.
	t.Chdir(t.TempDir())

	a, _ := newFormatApp(t)
	argv := []string{"tag", "template", "lint", "./does-not-exist", "--format", "bogus"}
	err := a.Run(argv)

	assertUsageError(t, err, argv)
	assert.Contains(t, err.Error(), "unsupported format",
		"the format error must win over the missing-template error")
}

func TestUT_FormatCommands_SurfaceMatchesGolden(t *testing.T) {
	var got []string
	for _, fc := range formatCommands(t) {
		got = append(got, fc.name())
	}
	sort.Strings(got)

	want := []string{
		"cache ls",
		"check",
		"convert cookiecutter",
		"diff",
		"dialect list",
		"dialect show",
		"doctor",
		"extract",
		"generate",
		"generate list",
		"lib ls",
		"lib search",
		"scaffold",
		"template graph",
		"template info",
		"template lint",
		"template list",
		"template variables",
		"test",
		"undo",
		"update",
		"version",
	}
	sort.Strings(want)

	assert.Equal(t, want, got,
		"the --format-capable surface changed — this must be a conscious in/out decision, not a silent one, and the new command needs its JSON shape documented")
}

// zeroFixtureJSONCommands are the commands that emit a complete JSON document
// in an empty directory, measured against a real build rather than assumed.
// The action commands (generate, scaffold, convert, extract, update) and the
// three needing a project fixture (check, template list, generate list) are
// deliberately absent: they are covered by their own targeted tests, and
// dragging their fixtures in here would make one broken fixture read as a
// conformance failure under CI's -failfast.
var zeroFixtureJSONCommands = [][]string{
	{"version"},
	{"cache", "ls"},
	{"lib", "ls"},
	{"lib", "search", "conformance-probe"},
	{"dialect", "list"},
	{"undo", "--list"},
}

func TestUT_ZeroFixtureCommands_EmitOneJSONDocument(t *testing.T) {
	for _, path := range zeroFixtureJSONCommands {
		t.Run(strings.Join(path, " "), func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("XDG_DATA_HOME", t.TempDir())
			t.Setenv("XDG_CACHE_HOME", t.TempDir())

			a, out := newFormatApp(t)
			argv := append(append([]string{"tag"}, path...), "--format", "json")
			require.NoError(t, a.Run(argv))

			// json.Unmarshal alone would accept some shapes with trailing
			// content; decoding once and then demanding EOF is what "exactly
			// one document" actually means.
			dec := json.NewDecoder(bytes.NewReader(out.Bytes()))
			var doc any
			require.NoError(t, dec.Decode(&doc), "stdout must parse as JSON: %q", out.String())

			_, err := dec.Token()
			require.ErrorIs(t, err, io.EOF, "exactly one JSON document must be on the wire, got %q", out.String())
		})
	}
}
