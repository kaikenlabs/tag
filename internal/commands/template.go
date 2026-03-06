package commands

import (
	"os"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/types/flags"
)

// TemplateCommand returns the parent command for template authoring operations.
func TemplateCommand(cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name:  "template",
		Usage: "Template authoring commands (init, new, info, list, lint, test, variables)",
		Subcommands: []*cli.Command{
			templateInitCommand(),
			templateNewCommand(cfg),
			templateInfoCommand(),
			templateListCommand(cfg),
			templateLintCommand(),
			templateTestCommand(),
			templateVariablesCommand(),
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
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  flags.AllFlag,
				Usage: "Show all generators and bundles, including those with unmet requirements",
			},
		},
		Action: func(c *cli.Context) error {
			return generateList(cfg, c.Bool(flags.AllFlag), os.Stdout)
		},
	}
}
