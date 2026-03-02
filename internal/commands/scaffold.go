package commands

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/convert"
	"github.com/kaikenlabs/tag/internal/history"
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
		Aliases:   []string{"s"},
		Usage:     "Create a new project from a template",
		ArgsUsage: "[template] [project-name]",
		Description: `Scaffold a new project from a local, remote, or library template.

With no arguments and a TTY, shows an interactive picker for installed
library templates (equivalent to the former 'tag run').

TEMPLATE FORMATS:
  Local:    ./my-template, /path/to/template
  GitHub:   gh:user/repo, gh:user/repo@v1.0.0, gh:user/repo/subdir
  GitLab:   gl:user/repo, gl:user/repo@v1.0.0
  Bitbucket: bb:user/repo
  Git URL:  https://github.com/user/repo.git
  Zip URL:  https://example.com/template.zip
  Local Zip: ./template.zip
  Library:  <template-name> (installed via 'tag lib add')

Examples:
  # Pick a library template interactively
  tag scaffold

  # Scaffold from an installed library template
  tag scaffold go-api my-service

  # Scaffold from a local template
  tag scaffold ./my-template

  # Scaffold from a GitHub template
  tag scaffold gh:user/awesome-template

  # Scaffold a specific version
  tag scaffold gh:user/awesome-template@v1.0.0

  # Scaffold with variable overrides
  tag scaffold gh:user/template -m author="John Doe" -m license=MIT

  # Force refresh of cached template
  tag scaffold gh:user/template --update

  # Scaffold non-interactively (use defaults)
  tag scaffold gh:user/template --no-input

  # Replay with saved values from previous scaffold
  tag scaffold gh:user/template another-api --replay`,
		Flags:        scaffoldFlags(),
		Action:       scaffoldAction,
		BashComplete: completeLibraryTemplateNames,
	}
}

func scaffoldAction(c *cli.Context) error {
	positional, err := reparseTrailingFlags(c, scaffoldFlags())
	if err != nil {
		return app.Errorf("invalid flags: %w", err)
	}

	// No args → try library picker
	if len(positional) < 1 {
		lib, err := newLocalLibrary()
		if err != nil {
			return app.Errorf("failed to initialize library: %w", err)
		}

		templateName, err := resolveTemplateName(c, lib, positional)
		if err != nil {
			return err
		}

		entry, err := lib.Get(templateName)
		if err != nil {
			return asAppError(err)
		}

		return scaffoldFromLibrary(c, lib, entry, positional)
	}

	return scaffoldFromRef(c, positional)
}

// scaffoldFromLibrary scaffolds a project from an installed library template.
func scaffoldFromLibrary(c *cli.Context, lib *library.Library, entry *library.Entry, positional []string) error {
	projectName := ""
	if len(positional) >= 2 {
		projectName = positional[1]
	}

	templateDir, err := lib.TemplatePath(entry.Name)
	if err != nil {
		return asAppError(err)
	}

	meta, err := parse.ParseKeyValues(c.StringSlice("meta"), true)
	if err != nil {
		return app.Errorf("invalid meta flag: %w", err)
	}

	opts := buildScaffoldOpts(c, templateDir, projectName, meta)
	opts.TemplateRef = entry.Source
	opts.TemplateName = entry.Name
	opts.IsRemote = false

	s, err := scaffold.NewScaffold(opts)
	if err != nil {
		return app.Errorf("failed to initialize scaffold: %w", err)
	}

	s.SetRecorder(history.NewRecorder(""))

	if err := s.Run(opts); err != nil {
		var ccErr *scaffold.CookiecutterDetectedError
		if errors.As(err, &ccErr) {
			return app.Errorf("template %q is a Cookiecutter template; run 'tag lib update %s' to convert it", entry.Name, entry.Name)
		}
		return app.Errorf("scaffolding failed: %w", err)
	}

	return nil
}

// scaffoldFromRef handles the case where a template reference is provided.
func scaffoldFromRef(c *cli.Context, positional []string) error {
	templateRef := positional[0]
	projectName := ""
	if len(positional) >= 2 {
		projectName = positional[1]
	}

	// Check if the reference is a library template name first.
	// Only do this for bare names — not paths, URLs, or remote shorthands.
	if looksLikeBareName(templateRef) {
		lib, libErr := newLocalLibrary()
		if libErr == nil {
			if entry, getErr := lib.Get(templateRef); getErr == nil {
				return scaffoldFromLibrary(c, lib, entry, positional)
			}
		}
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
	opts.TemplateRef = templateRef
	opts.IsRemote = isRemote
	if isRemote {
		opts.TemplateName = remote.DeriveName(templateRef)
	}

	// Create and run scaffold
	s, err := scaffold.NewScaffold(opts)
	if err != nil {
		return app.Errorf("failed to initialize scaffold: %w", err)
	}

	// Scaffold only creates new files (no inject/append), so tagDir for backups is unused.
	s.SetRecorder(history.NewRecorder(""))

	if err := s.Run(opts); err != nil {
		var ccErr *scaffold.CookiecutterDetectedError
		if errors.As(err, &ccErr) {
			return handleCookiecutterDetection(c, ccErr, templateRef, templateDir, opts)
		}
		return app.Errorf("scaffolding failed: %w", err)
	}

	// Auto-add remote templates to the library for future use
	if isRemote {
		addToLibrary(c, templateRef, templateDir)
	}

	return nil
}

// looksLikeBareName returns true if ref is a simple name (no path separators,
// no URL scheme, no remote shorthand prefix like "gh:"). Library template names
// are validated to be bare identifiers, so this distinguishes them from paths
// and remote references.
func looksLikeBareName(ref string) bool {
	if strings.HasPrefix(ref, "./") || strings.HasPrefix(ref, "../") || strings.HasPrefix(ref, "/") {
		return false
	}
	if strings.Contains(ref, "://") || strings.Contains(ref, ":") {
		return false
	}
	return true
}

// resolveTemplateName determines the template name from positional args or interactive picker.
func resolveTemplateName(c *cli.Context, lib *library.Library, positional []string) (string, error) {
	switch {
	case len(positional) >= 1:
		return positional[0], nil
	case scaffold.IsTTY() && !c.Bool("no-input"):
		return pickTemplate(lib)
	default:
		return "", app.UsageErrorf("template argument required\n\nUsage: tag scaffold <template> [project-name]")
	}
}

// pickTemplate shows an interactive template picker.
func pickTemplate(lib *library.Library) (string, error) {
	entries, err := lib.List()
	if err != nil {
		return "", err
	}

	if len(entries) == 0 {
		return "", app.Errorf("no templates installed; add one with: tag lib add <ref>")
	}

	opts := make([]huh.Option[string], len(entries))
	for i, e := range entries {
		display := e.Name
		if e.Description != "" {
			display += " - " + e.Description
		}
		display += " (" + e.Source + ")"
		opts[i] = huh.NewOption(display, e.Name)
	}

	var result string

	if err := huh.NewSelect[string]().
		Title("Select a template").
		Options(opts...).
		Height(10).
		Value(&result).
		Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", scaffold.ErrPromptCancelled
		}
		return "", err
	}

	return result, nil
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
	s.SetRecorder(history.NewRecorder(""))
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

	fmt.Fprintf(c.App.Writer, "Converted template to: %s\n", result.Destination)
	fmt.Fprintf(c.App.Writer, "  Variables: %d, Files: %d\n", result.VariablesConverted, result.FilesProcessed)
	if len(result.Warnings) > 0 {
		fmt.Fprintf(c.App.Writer, "  Warnings: %d (review after scaffolding)\n", len(result.Warnings))
	}
	fmt.Fprintln(c.App.Writer)

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
		slog.Warn("could not add template to library", "error", err)
		return
	}

	name := remote.DeriveName(templateRef)

	// Skip if the template is already in the library.
	if _, getErr := lib.Get(name); getErr == nil {
		fmt.Fprintf(c.App.Writer, "\nTemplate %q already in library. Run with: tag scaffold %s\n", name, name)
		return
	}

	result, err := lib.Add(c.Context, library.AddOptions{
		Ref:         templateRef,
		Name:        name,
		Force:       true,
		ResolvedDir: templateDir,
	})
	if err != nil {
		slog.Warn("could not add template to library", "error", err)
		return
	}

	fmt.Fprintf(c.App.Writer, "\nTemplate added to library as %q. Run with: tag scaffold %s\n", result.Name, result.Name)
}

// suggestConvertedTemplateName generates a default name for converted template output.
func suggestConvertedTemplateName(templateRef string) string {
	return remote.DeriveName(templateRef) + "-tag"
}
