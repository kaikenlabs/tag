package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestUT_CompletionCommand_Structure(t *testing.T) {
	t.Parallel()
	app := &cli.App{Name: "tag"}
	cmd := CompletionCommand(app)

	assert.Equal(t, "completion", cmd.Name)
	assert.NotEmpty(t, cmd.Usage)
	assert.NotEmpty(t, cmd.Description)
	require.Len(t, cmd.Subcommands, 3)
}

func TestUT_CompletionCommand_FishAction(t *testing.T) {
	t.Parallel()
	app := &cli.App{Name: "tag"}
	cmd := CompletionCommand(app)

	var fishCmd *cli.Command
	for _, sc := range cmd.Subcommands {
		if sc.Name == "fish" {
			fishCmd = sc
			break
		}
	}
	require.NotNil(t, fishCmd)
	assert.NotNil(t, fishCmd.Action)
}

func TestUT_BashCompletionScript_Content(t *testing.T) {
	t.Parallel()
	assert.Contains(t, bashCompletionScript, "complete")
	assert.Contains(t, bashCompletionScript, "tag")
	assert.Contains(t, bashCompletionScript, "_cli_bash_autocomplete")
}

func TestUT_ZshCompletionScript_Content(t *testing.T) {
	t.Parallel()
	assert.Contains(t, zshCompletionScript, "#compdef tag")
	assert.Contains(t, zshCompletionScript, "_cli_zsh_autocomplete")
	assert.Contains(t, zshCompletionScript, "compdef")
}

func TestUT_CompletionCommand_SubcommandUsage(t *testing.T) {
	t.Parallel()
	app := &cli.App{Name: "tag"}
	cmd := CompletionCommand(app)

	for _, sc := range cmd.Subcommands {
		assert.NotEmpty(t, sc.Usage, "subcommand %s should have usage", sc.Name)
		assert.NotNil(t, sc.Action, "subcommand %s should have action", sc.Name)
	}
}

func TestUT_CompletionCommand_DescriptionContainsExamples(t *testing.T) {
	t.Parallel()
	app := &cli.App{Name: "tag"}
	cmd := CompletionCommand(app)

	assert.Contains(t, cmd.Description, "bash")
	assert.Contains(t, cmd.Description, "zsh")
	assert.Contains(t, cmd.Description, "fish")
}
