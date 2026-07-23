package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/config"
)

func TestUT_TemplateCommand_SubcommandCount(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	cmd := TemplateCommand(cfg)

	require.NotNil(t, cmd)
	assert.Equal(t, "template", cmd.Name)
	assert.Len(t, cmd.Subcommands, 9)
}

func TestUT_TemplateCommand_NewHasSubcommands(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	cmd := TemplateCommand(cfg)

	var newCmd *cli.Command
	for _, sc := range cmd.Subcommands {
		if sc.Name == "new" {
			newCmd = sc
			break
		}
	}
	require.NotNil(t, newCmd)

	names := make([]string, len(newCmd.Subcommands))
	for i, sc := range newCmd.Subcommands {
		names[i] = sc.Name
	}
	assert.ElementsMatch(t, []string{"generator", "bundle"}, names)
}
