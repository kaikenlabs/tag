package commands

import (
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/convert"
	"github.com/kaikenlabs/tag/internal/types/flags"
	"github.com/kaikenlabs/tag/pkg/app"
)

// ConvertCommand returns the convert command definition with subcommands.
func ConvertCommand() *cli.Command {
	return &cli.Command{
		Name:  "convert",
		Usage: "Convert templates from other formats to TAG format",
		Subcommands: []*cli.Command{
			convertCookiecutterCommand(),
		},
	}
}

// convertCookiecutterCommand returns the cookiecutter conversion subcommand.
func convertCookiecutterCommand() *cli.Command {
	return &cli.Command{
		Name:      "cookiecutter",
		Usage:     "Convert a Cookiecutter template to TAG format",
		ArgsUsage: "<source>",
		Description: `Convert a Cookiecutter template to TAG format.

This command converts cookiecutter.json to tag.template.json and renames
path placeholders from {{ cookiecutter.var }} to __var__ syntax.

Template content (Jinja2) is mostly compatible with TAG's Gonja engine,
but the tool will analyze and report any potential incompatibilities.

ARGUMENTS:
  source       Path or remote reference to Cookiecutter template

REMOTE FORMATS:
  GitHub:      gh:user/cookiecutter-project
  GitLab:      gl:user/cookiecutter-project
  Bitbucket:   bb:user/cookiecutter-project
  Git URL:     https://github.com/user/cookiecutter-project.git

EXAMPLES:
  # Convert a local Cookiecutter template
  tag convert cookiecutter ./cookiecutter-myproject -o ./myproject-tag

  # Convert a remote template
  tag convert cookiecutter gh:user/cookiecutter-django -o ./django-tag

  # Preview conversion without writing files
  tag convert cookiecutter ./cookiecutter-myproject -d

  # Force overwrite existing output
  tag convert cookiecutter ./cookiecutter-myproject -o ./output --force`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Output directory (default: <source-name>-tag)",
			},
			&cli.BoolFlag{
				Name:    "force",
				Aliases: []string{"f"},
				Usage:   "Overwrite output directory if it exists",
			},
		},
		Action: convertCookiecutterAction,
	}
}

func convertCookiecutterAction(c *cli.Context) error {
	// Validate arguments
	if c.NArg() < 1 {
		return app.Errorf("source template is required\n\nUsage: tag convert cookiecutter <source>")
	}

	source := c.Args().Get(0)
	destination := c.String("output")

	// Create converter
	converter, err := convert.NewConverter()
	if err != nil {
		return app.Errorf("failed to initialize converter: %w", err)
	}

	// Build options
	opts := convert.Options{
		Source:      source,
		Destination: destination,
		DryRun:      c.Bool(flags.DryRunFlag),
		Force:       c.Bool("force"),
	}

	// Run conversion
	result, err := converter.Convert(c.Context, opts)
	if err != nil {
		return app.Errorf("conversion failed: %w", err)
	}

	// Display results
	printConversionResult(result)

	return nil
}

// printConversionResult displays the conversion summary.
func printConversionResult(result *convert.Result) {
	if result.DryRun {
		fmt.Println("=== Dry Run - No files written ===")
		fmt.Println()
	}

	fmt.Printf("Converted template: %s\n", result.Destination)
	fmt.Println()

	// Success indicators
	fmt.Printf("✓ Variables: %d converted\n", result.VariablesConverted)
	fmt.Printf("✓ Directories renamed: %d\n", result.DirsRenamed)
	fmt.Printf("✓ Files renamed: %d\n", result.FilesRenamed)
	fmt.Printf("✓ Files processed: %d\n", result.FilesProcessed)

	if result.HooksCopied > 0 {
		fmt.Printf("⚠ Hooks: %d files copied (review required)\n", result.HooksCopied)
	}

	// Incompatibilities
	if len(result.Incompatibilities) > 0 {
		fmt.Println()
		fmt.Printf("⚠ Content incompatibilities found: %d\n", len(result.Incompatibilities))
		fmt.Println("  Minor adjustments may be needed:")

		// Group by file
		byFile := make(map[string][]convert.Incompatibility)
		for _, inc := range result.Incompatibilities {
			byFile[inc.Path] = append(byFile[inc.Path], inc)
		}

		for path, incs := range byFile {
			for _, inc := range incs {
				fmt.Printf("  - %s:%d - %s\n", path, inc.Line, inc.Kind)
				if inc.Original != "" {
					fmt.Printf("    Found: %s\n", truncate(inc.Original, 60))
				}
				if inc.Suggestion != "" {
					fmt.Printf("    Gonja: %s\n", truncate(inc.Suggestion, 60))
				}
			}
		}
	}

	// Warnings
	if len(result.Warnings) > 0 {
		fmt.Println()
		fmt.Println("Warnings:")
		for _, w := range result.Warnings {
			fmt.Printf("  ⚠ %s\n", w)
		}
	}

	fmt.Println()
	fmt.Println("See: https://tag.kaikenlabs.com/docs/migration")

	if result.DryRun {
		fmt.Println()
		fmt.Println("Run without --dry-run to perform the conversion.")
	}
}

// truncate shortens a string to maxLen characters.
func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
