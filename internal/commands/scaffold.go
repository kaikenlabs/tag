package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/kaikenlabs/tag/internal/convert"
	"github.com/kaikenlabs/tag/internal/remote"
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
		Description: `Scaffold a new project from a local or remote template.

The template must contain a tag.template.json file that defines
the template configuration and variables.

TEMPLATE FORMATS:
  Local:    ./my-template, /path/to/template
  GitHub:   gh:user/repo, gh:user/repo@v1.0.0, gh:user/repo/subdir
  GitLab:   gl:user/repo, gl:user/repo@v1.0.0
  Bitbucket: bb:user/repo
  Git URL:  https://github.com/user/repo.git
  Zip URL:  https://example.com/template.zip
  Local Zip: ./template.zip

Examples:
  # Scaffold from a local template
  tag scaffold ./my-template

  # Scaffold from a GitHub template
  tag scaffold gh:user/awesome-template

  # Scaffold a specific version
  tag scaffold gh:user/awesome-template@v1.0.0

  # Scaffold from a subdirectory
  tag scaffold gh:user/templates/go-api

  # Scaffold with a project name
  tag scaffold gh:user/template my-awesome-project

  # Scaffold with variable overrides
  tag scaffold gh:user/template -m author="John Doe" -m license=MIT

  # Force refresh of cached template
  tag scaffold gh:user/template --update

  # Scaffold non-interactively (use defaults)
  tag scaffold gh:user/template --no-input

  # Replay with saved values from previous scaffold
  tag scaffold gh:user/template another-api --replay

  # Scaffold without saving replay data
  tag scaffold gh:user/template test-project --no-save`,
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
			&cli.BoolFlag{
				Name:    "update",
				Aliases: []string{"u"},
				Usage:   "Force refresh of cached remote templates",
			},
			&cli.BoolFlag{
				Name:  "replay",
				Usage: "Reuse saved variable values from a previous scaffold of this template",
			},
			&cli.BoolFlag{
				Name:  "no-save",
				Usage: "Don't save variable values for future replay",
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

	templateRef := c.Args().Get(0)
	projectName := c.Args().Get(1) // May be empty

	// Resolve template reference (handles both local and remote)
	resolver, err := remote.NewResolver()
	if err != nil {
		return app.Errorf("failed to create resolver: %w", err)
	}

	ctx := context.Background()
	templateDir, err := resolver.Resolve(ctx, templateRef, remote.ResolveOptions{
		ForceUpdate: c.Bool("update"),
	})
	if err != nil {
		return app.Errorf("failed to resolve template: %w", err)
	}

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
		Replay:      c.Bool("replay"),
		NoSave:      c.Bool("no-save"),
		TemplateRef: templateRef, // Pass original reference for replay ID generation
	}

	// Create and run scaffold
	s, err := scaffold.NewScaffold(opts)
	if err != nil {
		return app.Errorf("failed to initialize scaffold: %w", err)
	}

	if err := s.Run(opts); err != nil {
		// Check if this is a Cookiecutter template detection
		var ccErr *scaffold.ErrCookiecutterDetected
		if errors.As(err, &ccErr) {
			return handleCookiecutterDetection(c, ccErr, templateRef, templateDir, opts)
		}
		return app.Errorf("scaffolding failed: %w", err)
	}

	return nil
}

// handleCookiecutterDetection handles the case when a Cookiecutter template is detected.
func handleCookiecutterDetection(c *cli.Context, ccErr *scaffold.ErrCookiecutterDetected, templateRef, templateDir string, opts scaffold.Options) error {
	// In non-interactive mode, fail with helpful error
	if c.Bool("no-input") || !scaffold.IsTTY() {
		return app.Errorf("This appears to be a Cookiecutter template (found %s).\n"+
			"Cannot convert in non-interactive mode.\n"+
			"Run without --no-input to convert interactively, or use:\n"+
			"  tag convert cookiecutter %s",
			ccErr.CookiecutterPath, templateRef)
	}

	// Prompt for conversion
	prompter := scaffold.NewInteractivePrompter()
	confirmed, err := prompter.Confirm(
		fmt.Sprintf("This appears to be a Cookiecutter template (found %s). Convert to TAG format?",
			ccErr.CookiecutterPath),
		true, // default yes
	)
	if err != nil {
		return app.Errorf("prompt failed: %w", err)
	}
	if !confirmed {
		return app.Errorf("Conversion declined. Use 'tag convert cookiecutter %s' to convert manually.", templateRef)
	}

	// Run in-place conversion
	converter, err := convert.NewConverter()
	if err != nil {
		return app.Errorf("failed to create converter: %w", err)
	}
	ctx := context.Background()
	if err := converter.ConvertInPlace(ctx, templateDir); err != nil {
		return app.Errorf("conversion failed: %w", err)
	}

	fmt.Println("Created tag.template.json")
	fmt.Println("Template ready")
	fmt.Println()

	// Retry scaffolding
	s, err := scaffold.NewScaffold(opts)
	if err != nil {
		return app.Errorf("failed to reinitialize scaffold: %w", err)
	}
	if err := s.Run(opts); err != nil {
		return app.Errorf("scaffolding failed: %w", err)
	}
	return nil
}
