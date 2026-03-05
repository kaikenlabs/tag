package commands

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/extract"
	"github.com/kaikenlabs/tag/internal/types/flags"
	"github.com/kaikenlabs/tag/pkg/app"
)

// ExtractCommand returns the extract command definition.
func ExtractCommand() *cli.Command {
	return &cli.Command{
		Name:      "extract",
		Aliases:   []string{"x"},
		Usage:     "Extract a generator template from an existing source file",
		ArgsUsage: "<source-file>",
		Description: `Extract a generator template from an existing source file.

This command reads a source file, detects occurrences of the given entity name
in various case forms (snake, pascal, upper, plural), and replaces them with
template expressions like {{ name | pascal }}.

ARGUMENTS:
  source-file  Path to the source file to extract from

EXAMPLES:
  # Extract a handler template from an existing Go file
  tag extract internal/handler/user_handler.go --name user --as handler

  # Preview without writing files
  tag extract internal/handler/user_handler.go --name user --as handler --dry-run

  # Interactively confirm each replacement
  tag extract internal/handler/user_handler.go --name user --as handler -i`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     flags.NameFlag,
				Required: true,
				Usage:    "Entity name to parameterize (e.g. user, order, product)",
			},
			&cli.StringFlag{
				Name:     flags.AsFlag,
				Required: true,
				Usage:    "Generator name (output directory under .tag/)",
			},
			&cli.BoolFlag{
				Name:    flags.InteractiveFlag,
				Aliases: []string{"i"},
				Usage:   "Confirm each replacement interactively",
			},
		},
		Action: extractAction,
	}
}

func extractAction(c *cli.Context) error {
	if c.NArg() < 1 {
		return app.UsageErrorf("source file is required\n\nUsage: tag extract <source-file> --name <name> --as <generator>")
	}

	sourcePath := c.Args().Get(0)
	name := c.String(flags.NameFlag)
	as := c.String(flags.AsFlag)

	// Validate generator name for path safety.
	if err := ValidateNameSafe(as); err != nil {
		return app.Errorf("invalid generator name: %w", err)
	}

	// Validate source file exists.
	info, err := os.Stat(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return app.NotFoundErrorf("source file not found: %s", sourcePath)
		}
		return app.Errorf("cannot access source file: %w", err)
	}
	if info.IsDir() {
		return app.UsageErrorf("source must be a file, not a directory: %s", sourcePath)
	}

	tagDir := c.String(flags.PathFlag)
	interactive := c.Bool(flags.InteractiveFlag)

	opts := extract.Options{
		Name:        name,
		As:          as,
		DryRun:      c.Bool(flags.DryRunFlag),
		Interactive: interactive,
		TagDir:      tagDir,
		Writer:      c.App.Writer,
	}

	if interactive {
		opts.Prompter = extract.NewPromptConfirmer(os.Stdin, c.App.Writer)
	}

	result, err := extract.Run(opts, sourcePath)
	if err != nil {
		return err
	}

	if !opts.DryRun {
		fmt.Fprintf(c.App.Writer, "Extracted template: %s\n", result.TemplatePath)
		fmt.Fprintf(c.App.Writer, "  to: %s\n", result.ToPath)
		fmt.Fprintf(c.App.Writer, "  replacements: %d\n", result.Replacements)
	}

	return nil
}
