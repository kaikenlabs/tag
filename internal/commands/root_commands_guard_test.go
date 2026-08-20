package commands

import (
	"io"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

type probedCommand struct {
	path []string
}

// probeCommands walks RootCommands looking for the commands that both accept
// positional arguments and declare their own flags. Those are exactly the
// commands where urfave/cli's first-positional-stops-parsing behaviour can
// silently drop a trailing flag, so those are the commands reparseTrailingFlags
// must guard.
//
// Args and ArgsUsage are independent fields on cli.Command: bundle.go and
// new.go set both, but a future command could set only Args (leaving
// ArgsUsage empty for its own reasons), so both are checked rather than just
// ArgsUsage.
//
// Commands with positionals but no flags of their own (lib rm, lib update,
// template test, generate info) are deliberately excluded: they read no
// context flags at all, so reparsing there would accept-and-then-ignore a
// trailing global flag, which is worse than urfave/cli's own "unknown flag"
// rejection of it.
func probeCommands(t *testing.T) []probedCommand {
	t.Helper()

	cfg := createTestConfig(t, t.TempDir())
	root := RootCommands(cfg, "test", SkillDocs{})

	var probes []probedCommand
	var walk func(cmds []*cli.Command, prefix []string)
	walk = func(cmds []*cli.Command, prefix []string) {
		for _, cmd := range cmds {
			path := append(append([]string{}, prefix...), cmd.Name)
			if cmd.Action != nil && len(cmd.Flags) > 0 && (cmd.Args || cmd.ArgsUsage != "") {
				probes = append(probes, probedCommand{path: path})
			}
			if len(cmd.Subcommands) > 0 {
				walk(cmd.Subcommands, path)
			}
		}
	}
	walk(root, nil)
	return probes
}

func newProbeApp(t *testing.T) *cli.App {
	t.Helper()

	cfg := createTestConfig(t, t.TempDir())
	return &cli.App{
		Writer:         io.Discard,
		ErrWriter:      io.Discard,
		Flags:          GlobalFlags(),
		Commands:       RootCommands(cfg, "test", SkillDocs{}),
		ExitErrHandler: func(*cli.Context, error) {},
	}
}

func TestUT_AllCommands_RejectUnknownTrailingFlag(t *testing.T) {
	probes := probeCommands(t)
	require.NotEmpty(t, probes)

	for _, p := range probes {
		t.Run(strings.Join(p.path, " "), func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)

			argv := append([]string{"tag"}, p.path...)
			argv = append(argv, "tag-guard-probe", "--tag-guard-not-a-flag")

			err := newProbeApp(t).Run(argv)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "unknown flag -tag-guard-not-a-flag")

			entries, readErr := os.ReadDir(dir)
			require.NoError(t, readErr)
			assert.Empty(t, entries, "command must not have written anything before rejecting the unknown flag")
		})
	}
}

func TestUT_RootCommands_ProbedSurfaceMatchesGolden(t *testing.T) {
	probes := probeCommands(t)

	var got []string
	for _, p := range probes {
		got = append(got, strings.Join(p.path, " "))
	}
	sort.Strings(got)

	want := []string{
		"convert cookiecutter",
		"dialect show",
		"extract",
		"generate",
		"generate agent-file",
		"lib add",
		"lib edit",
		"lib search",
		"scaffold",
		"template graph",
		"template info",
		"template lint",
		"template new bundle",
		"template new generator",
		"template rename-var",
		"template variables",
		"test",
	}

	assert.Equal(t, want, got, "the guarded command surface changed — this must be a conscious in/out decision, not a silent exemption")
}
