package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/urfave/cli/v2"
)

func TestUT_CompletionCommand_SubcommandNames(t *testing.T) {
	t.Parallel()
	app := &cli.App{Name: "tag"}
	cmd := CompletionCommand(app)

	require.NotNil(t, cmd)
	assert.Equal(t, "completion", cmd.Name)

	names := make([]string, len(cmd.Subcommands))
	for i, sc := range cmd.Subcommands {
		names[i] = sc.Name
	}
	assert.ElementsMatch(t, []string{"bash", "zsh", "fish"}, names)
}

func TestUT_CompletionCommand_BashAction(t *testing.T) {
	t.Parallel()
	app := &cli.App{Name: "tag"}
	cmd := CompletionCommand(app)

	var bashCmd *cli.Command
	for _, sc := range cmd.Subcommands {
		if sc.Name == "bash" {
			bashCmd = sc
			break
		}
	}
	require.NotNil(t, bashCmd)
	assert.NotNil(t, bashCmd.Action)
}

func TestUT_CompletionCommand_ZshAction(t *testing.T) {
	t.Parallel()
	app := &cli.App{Name: "tag"}
	cmd := CompletionCommand(app)

	var zshCmd *cli.Command
	for _, sc := range cmd.Subcommands {
		if sc.Name == "zsh" {
			zshCmd = sc
			break
		}
	}
	require.NotNil(t, zshCmd)
	assert.NotNil(t, zshCmd.Action)
}
