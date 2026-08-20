package commands

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/extract"
	"github.com/kaikenlabs/tag/internal/jsonout"
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
  tag extract --name user --as handler internal/handler/user_handler.go

  # Preview without writing files
  tag extract --name user --as handler --dry-run internal/handler/user_handler.go

  # Interactively confirm each replacement
  tag extract --name user --as handler -i internal/handler/user_handler.go`,
		Flags:  extractFlags(),
		Action: extractAction,
	}
}

// extractFlags is shared between the command definition and
// reparseTrailingFlags, so a trailing --format is recognised rather than
// silently dropped.
func extractFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:  flags.NameFlag,
			Usage: "Entity name to parameterize (e.g. user, order, product)",
		},
		&cli.StringFlag{
			Name:  flags.AsFlag,
			Usage: "Generator name (output directory under .tag/)",
		},
		&cli.BoolFlag{
			Name:    flags.DryRunFlag,
			Aliases: []string{"d"},
			Usage:   "Preview what would be written without creating any files",
		},
		&cli.BoolFlag{
			Name:    flags.InteractiveFlag,
			Aliases: []string{"i"},
			Usage:   "Confirm each replacement interactively",
		},
		formatFlag(formatText, formatJSON),
	}
}

func extractAction(c *cli.Context) error {
	args, err := reparseTrailingFlags(c, extractFlags())
	if err != nil {
		return err
	}

	if len(args) < 1 {
		return app.UsageErrorf("source file is required\n\nUsage: tag extract --name <name> --as <generator> <source-file>")
	}

	format, err := resolveFormat(c, formatText, formatJSON)
	if err != nil {
		return err
	}
	jsonMode := format == formatJSON

	sourcePath := args[0]
	name := c.String(flags.NameFlag)
	as := c.String(flags.AsFlag)
	interactive := c.Bool(flags.InteractiveFlag)

	// D3: silently disabling an explicitly requested flag is worse than
	// refusing outright.
	if jsonMode && interactive {
		return app.UsageErrorf("--format json does not support -i/--interactive")
	}

	if name == "" {
		return app.UsageErrorf("--name flag is required\n\nUsage: tag extract --name <name> --as <generator> <source-file>")
	}
	if as == "" {
		return app.UsageErrorf("--as flag is required\n\nUsage: tag extract --name <name> --as <generator> <source-file>")
	}

	// Validate generator name for path safety.
	if nameErr := ValidateNameSafe(as); nameErr != nil {
		return app.Errorf("invalid generator name: %w", nameErr)
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

	opts := extract.Options{
		Name:        name,
		As:          as,
		DryRun:      c.Bool(flags.DryRunFlag),
		Interactive: interactive,
		TagDir:      tagDir,
		Writer:      c.App.Writer,
	}

	// The dry-run preview writes human text through opts.Writer; in JSON mode
	// it must not reach stdout, so it is rerouted to c.App.ErrWriter — visible
	// to a human, absent from the parseable document.
	if jsonMode {
		opts.Writer = cmdErr(c)
	}

	if interactive {
		opts.Prompter = extract.NewPromptConfirmer(os.Stdin, c.App.Writer)
	}

	result, err := extract.Run(opts, sourcePath)
	if err != nil {
		return err
	}

	if jsonMode {
		return jsonout.Write(cmdOut(c), newExtractDoc(result, opts.DryRun))
	}

	if !opts.DryRun {
		fmt.Fprintf(c.App.Writer, "Extracted template: %s\n", result.TemplatePath)
		fmt.Fprintf(c.App.Writer, "  to: %s\n", result.ToPath)
		fmt.Fprintf(c.App.Writer, "  replacements: %d\n", result.Replacements)
	}

	return nil
}

// extractDoc is the JSON shape for `extract --format json`. Content carries
// the generated template body only in dry-run (D8): in dry-run nothing is on
// disk, so omitting it would leave the consumer with no way to see the
// result; on a real run the content is already on disk and unbounded, so it
// is omitted.
type extractDoc struct {
	TemplatePath string `json:"template_path"`
	ToPath       string `json:"to_path"`
	Replacements int    `json:"replacements"`
	Content      string `json:"content,omitempty"`
}

func newExtractDoc(result *extract.Result, dryRun bool) extractDoc {
	doc := extractDoc{
		TemplatePath: result.TemplatePath,
		ToPath:       result.ToPath,
		Replacements: result.Replacements,
	}
	if dryRun {
		doc.Content = result.Content
	}
	return doc
}
