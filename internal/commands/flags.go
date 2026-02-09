package commands

import (
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/scaffold"
)

// commonScaffoldFlags returns flags shared between the scaffold and run commands.
func commonScaffoldFlags() []cli.Flag {
	return []cli.Flag{
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
		&cli.BoolFlag{
			Name:  "replay",
			Usage: "Reuse saved variable values from a previous scaffold of this template",
		},
		&cli.BoolFlag{
			Name:  "no-save",
			Usage: "Don't save variable values for future replay",
		},
		&cli.BoolFlag{
			Name:  "accept-hooks",
			Usage: "Accept and run pre/post scaffold hooks without prompting for confirmation",
		},
		&cli.BoolFlag{
			Name:  "allow-recursive-render",
			Usage: "Allow template syntax in variable values to be rendered (SECURITY: enables recursive template rendering)",
		},
	}
}

// buildScaffoldOpts reads common flags from the CLI context and returns a scaffold.Options
// with all shared fields populated. Callers set only the differing fields (TemplateRef, IsRemote).
func buildScaffoldOpts(c *cli.Context, templateDir, projectName string, meta map[string]string) scaffold.Options {
	return scaffold.Options{
		TemplateDir:          templateDir,
		OutputDir:            c.String("output"),
		ProjectName:          projectName,
		ValuesFile:           c.String("values"),
		Meta:                 meta,
		NoInput:              c.Bool("no-input"),
		Force:                c.Bool("force"),
		Replay:               c.Bool("replay"),
		NoSave:               c.Bool("no-save"),
		AcceptHooks:          c.Bool("accept-hooks"),
		AllowRecursiveRender: c.Bool("allow-recursive-render"),
		IsTTY:                scaffold.IsTTY(),
	}
}
