package commands

import (
	"os"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/config"
)

// TemplateCommand returns the parent command for template authoring operations.
func TemplateCommand(cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name:  "template",
		Usage: "Template authoring commands (init, new, info, list, lint, test)",
		Subcommands: []*cli.Command{
			templateInitCommand(),
			templateNewCommand(cfg),
			templateInfoCommand(),
			templateListCommand(cfg),
			templateLintCommand(),
			templateTestCommand(),
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

func templateListCommand(cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name:    "list",
		Aliases: []string{"ls"},
		Usage:   "List available generators and bundles",
		Action: func(_ *cli.Context) error {
			return generateList(cfg, os.Stdout)
		},
	}
}
