package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/convert"
	"github.com/kaikenlabs/tag/internal/library"
	"github.com/kaikenlabs/tag/internal/parse"
	"github.com/kaikenlabs/tag/internal/remote"
	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/pkg/app"
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
		Flags:  scaffoldFlags(),
		Action: scaffoldAction,
	}
}

func scaffoldAction(c *cli.Context) error {
	positional, err := reparseTrailingFlags(c, scaffoldFlags())
	if err != nil {
		return app.Errorf("invalid flags: %w", err)
	}

	// Validate arguments
	if len(positional) < 1 {
		return app.Errorf("template path is required\n\nUsage: tag scaffold <template> [project-name]")
	}

	templateRef := positional[0]
	projectName := ""
	if len(positional) >= 2 {
		projectName = positional[1]
	}

	// Resolve template reference (handles both local and remote)
	resolver, err := remote.NewResolver()
	if err != nil {
		return app.Errorf("failed to create resolver: %w", err)
	}

	templateDir, err := resolver.Resolve(c.Context, templateRef, remote.ResolveOptions{
		ForceUpdate: c.Bool("update"),
	})
	if err != nil {
		return app.Errorf("failed to resolve template: %w", err)
	}

	// Parse meta flags
	metaSlice := c.StringSlice("meta")
	meta, err := parse.ParseKeyValues(metaSlice, true)
	if err != nil {
		return app.Errorf("invalid meta flag: %w", err)
	}

	// Determine if the template is remote
	isRemote := !remote.IsLocal(templateRef)

	// Build options
	opts := buildScaffoldOpts(c, templateDir, projectName, meta)
	opts.TemplateRef = templateRef // Original reference for replay ID generation
	opts.IsRemote = isRemote
	if isRemote {
		opts.TemplateName = deriveTemplateName(templateRef)
	}

	// Create and run scaffold
	s, err := scaffold.NewScaffold(opts)
	if err != nil {
		return app.Errorf("failed to initialize scaffold: %w", err)
	}

	if err := s.Run(opts); err != nil {
		// Check if this is a Cookiecutter template detection
		var ccErr *scaffold.CookiecutterDetectedError
		if errors.As(err, &ccErr) {
			return handleCookiecutterDetection(c, ccErr, templateRef, templateDir, opts)
		}
		return app.Errorf("scaffolding failed: %w", err)
	}

	// Auto-add remote templates to the library for future use with `tag run`
	if isRemote {
		addToLibrary(c, templateRef, templateDir)
	}

	return nil
}

// scaffoldFlags returns the full set of flags for the scaffold command.
func scaffoldFlags() []cli.Flag {
	return append(commonScaffoldFlags(), &cli.BoolFlag{
		Name:    "update",
		Aliases: []string{"u"},
		Usage:   "Force refresh of cached remote templates",
	})
}

// handleCookiecutterDetection handles the case when a Cookiecutter template is detected.
// Output convention: fmt for user-facing messages, slog for diagnostic messages.
func handleCookiecutterDetection(c *cli.Context, _ *scaffold.CookiecutterDetectedError, templateRef, templateDir string, opts scaffold.Options) error {
	// In non-interactive mode, fail with helpful error
	if c.Bool("no-input") || !scaffold.IsTTY() {
		return app.Errorf("This appears to be a Cookiecutter template.\n"+
			"Cannot convert in non-interactive mode.\n"+
			"Run without --no-input to convert interactively, or use:\n"+
			"  tag convert cookiecutter %s",
			templateRef)
	}

	prompter := scaffold.NewInteractivePrompter()

	destination, err := promptForConversion(prompter, templateRef)
	if err != nil {
		return err
	}

	result, err := runCookiecutterConversion(c, templateDir, destination)
	if err != nil {
		return err
	}

	if promptErr := promptForProjectDir(prompter, &opts); promptErr != nil {
		return promptErr
	}

	// Update opts to use the converted template directory
	opts.TemplateDir = result.Destination

	// Retry scaffolding with converted template
	s, err := scaffold.NewScaffold(opts)
	if err != nil {
		return app.Errorf("failed to reinitialize scaffold: %w", err)
	}
	if err := s.Run(opts); err != nil {
		return app.Errorf("scaffolding failed: %w", err)
	}
	return nil
}

// promptForConversion asks the user to confirm Cookiecutter conversion and select a destination.
func promptForConversion(prompter scaffold.Prompter, templateRef string) (string, error) {
	confirmed, err := prompter.Confirm(
		"This appears to be a Cookiecutter template. Convert to TAG format?",
		true, // default yes
	)
	if err != nil {
		return "", app.Errorf("prompt failed: %w", err)
	}
	if !confirmed {
		return "", app.Errorf("Conversion declined. Use 'tag convert cookiecutter %s' to convert manually.", templateRef)
	}

	defaultDestination := "./" + suggestConvertedTemplateName(templateRef)
	destination, err := prompter.Input("Output directory for converted template", defaultDestination, false)
	if err != nil {
		return "", app.Errorf("prompt failed: %w", err)
	}
	if destination == "" {
		destination = defaultDestination
	}
	return destination, nil
}

// runCookiecutterConversion performs the Cookiecutter-to-TAG conversion and prints the result.
func runCookiecutterConversion(c *cli.Context, templateDir, destination string) (*convert.Result, error) {
	converter, err := convert.NewConverter()
	if err != nil {
		return nil, app.Errorf("failed to create converter: %w", err)
	}
	result, err := converter.Convert(c.Context, convert.Options{
		Source:      templateDir,
		Destination: destination,
		Force:       c.Bool("force"),
	})
	if err != nil {
		return nil, app.Errorf("conversion failed: %w", err)
	}

	fmt.Printf("Converted template to: %s\n", result.Destination)
	fmt.Printf("  Variables: %d, Files: %d\n", result.VariablesConverted, result.FilesProcessed)
	if len(result.Warnings) > 0 {
		fmt.Printf("  Warnings: %d (review after scaffolding)\n", len(result.Warnings))
	}
	fmt.Println()

	return result, nil
}

// promptForProjectDir prompts the user for a project output directory if not already specified.
func promptForProjectDir(prompter scaffold.Prompter, opts *scaffold.Options) error {
	if opts.OutputDir != "" || opts.ProjectName != "" {
		return nil
	}

	defaultProject := "./my-project"
	projectDir, err := prompter.Input("Output directory for scaffolded project", defaultProject, false)
	if err != nil {
		return app.Errorf("prompt failed: %w", err)
	}
	if projectDir == "" {
		projectDir = defaultProject
	}
	opts.OutputDir = projectDir
	return nil
}

// addToLibrary adds a scaffolded remote template to the library (non-fatal on error).
// If an entry with the same name already exists, it is left unchanged.
func addToLibrary(c *cli.Context, templateRef, templateDir string) {
	lib, err := newLocalLibrary()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not add template to library: %v\n", err)
		return
	}

	name := deriveTemplateName(templateRef)

	// Skip if the template is already in the library.
	if _, getErr := lib.Get(name); getErr == nil {
		fmt.Printf("\nTemplate %q already in library. Run with: tag run %s\n", name, name)
		return
	}

	result, err := lib.Add(c.Context, library.AddOptions{
		Ref:         templateRef,
		Name:        name,
		Force:       true,
		ResolvedDir: templateDir,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not add template to library: %v\n", err)
		return
	}

	fmt.Printf("\nTemplate added to library as %q. Run with: tag run %s\n", result.Name, result.Name)
}

// deriveTemplateName extracts a library-compatible template name from a remote reference.
// For example, "bb:whalar/go-ms-service-template" becomes "go-ms-service-template".
func deriveTemplateName(ref string) string {
	return remote.DeriveName(ref)
}

// suggestConvertedTemplateName generates a default name for converted template output.
func suggestConvertedTemplateName(templateRef string) string {
	return remote.DeriveName(templateRef) + "-tag"
}
