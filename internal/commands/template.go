package commands

import (
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/types/flags"
)

// TemplateCommand returns the parent command for template authoring operations.
func TemplateCommand(cfg *config.Config, version string) *cli.Command {
	return &cli.Command{
		Name:  "template",
		Usage: "Template authoring commands (init, new, info, list, lint, test, variables, rename-var, graph)",
		Subcommands: []*cli.Command{
			templateInitCommand(),
			templateNewCommand(cfg),
			templateInfoCommand(version),
			templateListCommand(cfg),
			templateLintCommand(),
			templateTestCommand(),
			templateVariablesCommand(),
			templateRenameVarCommand(),
			templateGraphCommand(),
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

// templateListCommand and generateListCommand share one implementation
// (generateList) and the SAME flags slice (generateListFlags), so `template
// list` and `generate list` cannot drift in either their flags or their
// output shape.
func templateListCommand(cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name:    "list",
		Aliases: []string{"ls"},
		Usage:   "List available generators and bundles",
		Flags:   generateListFlags(),
		Action: func(c *cli.Context) error {
			format, err := resolveFormat(c, formatText, formatJSON)
			if err != nil {
				return err
			}
			return generateList(cfg, c.Bool(flags.AllFlag), cmdOut(c), format)
		},
	}
}
