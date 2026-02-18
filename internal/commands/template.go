package commands

import (
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/config"
)

// TemplateCommand returns the parent command for template authoring operations.
func TemplateCommand(cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name:  "template",
		Usage: "Template authoring commands (init, new, info)",
		Subcommands: []*cli.Command{
			templateInitCommand(),
			templateNewCommand(cfg),
			templateInfoCommand(),
		},
	}
}

func templateNewCommand(cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name:  "new",
		Usage: "Create a new generator or bundle template",
		Subcommands: []*cli.Command{
			templateNewGeneratorCommand(cfg),
			templateNewBundleCommand(cfg),
		},
	}
}
