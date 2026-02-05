package commands

import (
	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/pkg/app"
	"github.com/urfave/cli/v2"
)

// ScaffoldCommand returns the scaffold command definition.
func ScaffoldCommand() *cli.Command {
	return &cli.Command{
		Name:      "scaffold",
		Usage:     "Create a new project from a template",
		ArgsUsage: "<template> [project-name]",
		Description: `Scaffold a new project from a local template directory.

The template directory must contain a tag.template.json file that defines
the template configuration and variables.

Examples:
  # Scaffold from a local template
  tag scaffold ./my-template

  # Scaffold with a project name
  tag scaffold ./my-template my-awesome-project

  # Scaffold with variable overrides
  tag scaffold ./my-template -m author="John Doe" -m license=MIT

  # Scaffold with a values file
  tag scaffold ./my-template --values config.json

  # Scaffold non-interactively (use defaults)
  tag scaffold ./my-template --no-input`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Output directory (default: ./<project_name>)",
			},
			&cli.StringFlag{
				Name:  "values",
				Usage: "JSON file with variable values",
			},
			&cli.StringSliceFlag{
				Name:    "meta",
				Aliases: []string{"m"},
				Usage:   "Variable override in key=value format (can be repeated)",
			},
			&cli.BoolFlag{
				Name:  "no-input",
				Usage: "Skip interactive prompts, use defaults only",
			},
			&cli.BoolFlag{
				Name:    "force",
				Aliases: []string{"f"},
				Usage:   "Overwrite output directory if it exists",
			},
		},
		Action: scaffoldAction,
	}
}

func scaffoldAction(c *cli.Context) error {
	// Validate arguments
	if c.NArg() < 1 {
		return app.Errorf("template path is required\n\nUsage: tag scaffold <template> [project-name]")
	}

	templateDir := c.Args().Get(0)
	projectName := c.Args().Get(1) // May be empty

	// Parse meta flags
	metaSlice := c.StringSlice("meta")
	meta, err := scaffold.ParseMetaFlags(metaSlice)
	if err != nil {
		return app.Errorf("invalid meta flag: %w", err)
	}

	// Build options
	opts := scaffold.Options{
		TemplateDir: templateDir,
		OutputDir:   c.String("output"),
		ProjectName: projectName,
		ValuesFile:  c.String("values"),
		Meta:        meta,
		NoInput:     c.Bool("no-input"),
		Force:       c.Bool("force"),
	}

	// Create and run scaffold
	s, err := scaffold.NewScaffold(opts)
	if err != nil {
		return app.Errorf("failed to initialize scaffold: %w", err)
	}

	if err := s.Run(opts); err != nil {
		return app.Errorf("scaffolding failed: %w", err)
	}

	return nil
}
