package commands

import (
	"fmt"
	"io"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/convert"
	"github.com/kaikenlabs/tag/internal/jsonout"
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
path placeholders from {{ cookiecutter.var }} to {{ vars.var }} syntax.

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
  tag convert cookiecutter ./cookiecutter-myproject --dry-run

  # Force overwrite existing output
  tag convert cookiecutter ./cookiecutter-myproject -o ./output --force`,
		Flags:  convertCookiecutterFlags(),
		Action: convertCookiecutterAction,
	}
}

// convertCookiecutterFlags is shared between the command definition and
// reparseTrailingFlags, so a trailing --format (or --dry-run, a global flag)
// is recognised rather than silently dropped.
func convertCookiecutterFlags() []cli.Flag {
	return []cli.Flag{
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
		formatFlag(formatText, formatJSON),
	}
}

func convertCookiecutterAction(c *cli.Context) error {
	args, err := reparseTrailingFlags(c, convertCookiecutterFlags())
	if err != nil {
		return err
	}

	if len(args) < 1 {
		return app.UsageErrorf("source template is required\n\nUsage: tag convert cookiecutter <source>")
	}

	format, err := resolveFormat(c, formatText, formatJSON)
	if err != nil {
		return err
	}

	source := args[0]
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

	if format == formatJSON {
		return jsonout.Write(cmdOut(c), result)
	}

	// Display results
	printConversionResult(c.App.Writer, result)

	return nil
}

// printConversionResult displays the conversion summary.
func printConversionResult(w io.Writer, result *convert.Result) {
	if result.DryRun {
		fmt.Fprintln(w, "=== Dry Run - No files written ===")
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "Converted template: %s\n", result.Destination)
	fmt.Fprintln(w)

	// Success indicators
	fmt.Fprintf(w, "✓ Variables: %d converted\n", result.VariablesConverted)
	fmt.Fprintf(w, "✓ Directories renamed: %d\n", result.DirsRenamed)
	fmt.Fprintf(w, "✓ Files renamed: %d\n", result.FilesRenamed)
	fmt.Fprintf(w, "✓ Files processed: %d\n", result.FilesProcessed)

	if result.HooksCopied > 0 {
		fmt.Fprintf(w, "⚠ Hooks: %d files copied (review required)\n", result.HooksCopied)
	}

	// Incompatibilities
	if len(result.Incompatibilities) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "⚠ Content incompatibilities found: %d\n", len(result.Incompatibilities))
		fmt.Fprintln(w, "  Minor adjustments may be needed:")

		// Group by file
		byFile := make(map[string][]convert.Incompatibility)
		for _, inc := range result.Incompatibilities {
			byFile[inc.Path] = append(byFile[inc.Path], inc)
		}

		for path, incs := range byFile {
			for _, inc := range incs {
				fmt.Fprintf(w, "  - %s:%d - %s\n", path, inc.Line, inc.Kind)
				if inc.Original != "" {
					fmt.Fprintf(w, "    Found: %s\n", truncate(inc.Original, 60))
				}
				if inc.Suggestion != "" {
					fmt.Fprintf(w, "    Gonja: %s\n", truncate(inc.Suggestion, 60))
				}
			}
		}
	}

	// Warnings
	if len(result.Warnings) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Warnings:")
		for _, warn := range result.Warnings {
			fmt.Fprintf(w, "  ⚠ %s\n", warn)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "See: https://tag.kaikenlabs.com/docs/migration")

	if result.DryRun {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Run without --dry-run to perform the conversion.")
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
