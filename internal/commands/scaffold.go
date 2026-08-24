package commands

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/huh"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/convert"
	"github.com/kaikenlabs/tag/internal/fileaction"
	"github.com/kaikenlabs/tag/internal/history"
	"github.com/kaikenlabs/tag/internal/jsonout"
	"github.com/kaikenlabs/tag/internal/library"
	"github.com/kaikenlabs/tag/internal/lockfile"
	"github.com/kaikenlabs/tag/internal/parse"
	"github.com/kaikenlabs/tag/internal/remote"
	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/internal/types/flags"
	"github.com/kaikenlabs/tag/pkg/app"
)

// isTTY is a package var so tests can exercise the interactive branch; under
// `go test` the real probe is always false, which would make any assertion
// about JSON mode pass on a broken tree.
var isTTY = scaffold.IsTTY

// newPrompter is a package var so tests can substitute a prompter that fails
// the test if it is ever consulted. Asserting on a return value cannot
// distinguish "skipped the prompt" from "prompted and the prompt errored" —
// both of these call sites collapse a prompt failure into the same answer they
// give in non-interactive mode, which is precisely how a JSON-mode regression
// would hide.
var newPrompter = func() scaffold.Prompter { return scaffold.NewInteractivePrompter() }

// nonInteractive reports whether scaffold must run without prompting: the
// caller asked for it explicitly, JSON output has no terminal to prompt on,
// or stdin isn't a terminal to prompt through in the first place.
func nonInteractive(c *cli.Context, jsonMode bool) bool {
	return c.Bool("no-input") || jsonMode || !isTTY()
}

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

	// resolveFormat must run before the no-positional branch below, since that
	// branch decides between an interactive picker and a usage error based on
	// whether JSON mode is active — earlier than every other command needs it.
	format, err := resolveFormat(c, formatText, formatJSON)
	if err != nil {
		return err
	}
	jsonMode := format == formatJSON

	// No args → try library picker
	if len(positional) < 1 {
		lib, err := newLocalLibrary()
		if err != nil {
			return app.Errorf("failed to initialize library: %w", err)
		}

		templateName, err := resolveTemplateName(c, lib, positional, jsonMode)
		if err != nil {
			return err
		}

		entry, err := lib.Get(templateName)
		if err != nil {
			return asAppError(err)
		}

		return scaffoldFromLibrary(c, lib, entry, positional, jsonMode)
	}

	return scaffoldFromRef(c, positional, jsonMode)
}

// scaffoldFromLibrary scaffolds a project from an installed library template.
// runScaffold initialises and runs a scaffold with the given options.
// onCookiecutter handles the CookiecutterDetectedError case, allowing callers to
// provide context-specific error messages or recovery logic.
func runScaffold(c *cli.Context, opts scaffold.Options, jsonMode bool, onCookiecutter func(*scaffold.CookiecutterDetectedError) error) error {
	// Load dialect registry (all 3 tiers: built-in + user-global + template-local).
	reg, err := loadDialectRegistry(opts.TemplateDir)
	if err != nil {
		slog.Debug("dialect loading failed, continuing without dialects", "error", err)
		reg = nil
	}

	var sopts []scaffold.ScaffoldOption
	if reg != nil {
		tmplEngine, engineErr := template.NewEngine(template.WithDialectRegistry(reg))
		if engineErr != nil {
			return app.Errorf("failed to create template engine: %w", engineErr)
		}
		sopts = append(sopts, scaffold.WithEngine(tmplEngine))
	}

	// Human-facing text goes to c.App.ErrWriter under JSON mode, keeping
	// c.App.Writer free for the single JSON document.
	out := cmdOut(c)
	if jsonMode {
		out = cmdErr(c)
	}
	sopts = append(sopts, scaffold.WithOutput(out))

	s, err := scaffold.NewScaffold(opts, sopts...)
	if err != nil {
		return app.Errorf("failed to initialize scaffold: %w", err)
	}
	s.SetRecorder(history.NewRecorder(""))
	result, err := s.Run(opts)
	if err != nil {
		var ccErr *scaffold.CookiecutterDetectedError
		if errors.As(err, &ccErr) {
			return onCookiecutter(ccErr)
		}
		return app.Errorf("scaffolding failed: %w", err)
	}

	if jsonMode {
		return jsonout.Write(cmdOut(c), newScaffoldDoc(result))
	}
	displayScaffoldSummary(c.App.Writer, result)
	return nil
}

// scaffoldFileJSON is one entry of a scaffoldDoc's "files" list.
type scaffoldFileJSON struct {
	Path   string            `json:"path"`
	Action fileaction.Action `json:"action"`
}

// scaffoldDoc is the JSON shape for `scaffold --format json`.
type scaffoldDoc struct {
	OutputDir   string             `json:"output_dir"`
	ProjectRoot string             `json:"project_root"`
	Template    string             `json:"template"`
	Files       []scaffoldFileJSON `json:"files"`
	Created     int                `json:"created"`
	DryRun      bool               `json:"dry_run"`
}

func newScaffoldDoc(result scaffold.ScaffoldResult) scaffoldDoc {
	files := make([]scaffoldFileJSON, 0, len(result.Files))
	for _, f := range result.Files {
		files = append(files, scaffoldFileJSON{Path: f.Path, Action: f.Action})
	}

	return scaffoldDoc{
		OutputDir:   result.OutputDir,
		ProjectRoot: result.ProjectRoot,
		Template:    result.Opts.TemplateRef,
		Files:       files,
		Created:     len(files),
		DryRun:      result.Opts.DryRun,
	}
}

func scaffoldFromLibrary(c *cli.Context, lib *library.Library, entry *library.Entry, positional []string, jsonMode bool) error {
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

	opts := buildScaffoldOpts(c, templateDir, projectName, meta, jsonMode)
	opts.TemplateRef = entry.Source
	opts.TemplateName = entry.Name
	opts.IsRemote = false
	opts.SkipGeneratorCopy = true // generators resolve from library

	return runScaffold(c, opts, jsonMode, func(*scaffold.CookiecutterDetectedError) error {
		return app.Errorf("template %q is a Cookiecutter template; run 'tag lib update %s' to convert it", entry.Name, entry.Name)
	})
}

// scaffoldFromRef handles the case where a template reference is provided.
func scaffoldFromRef(c *cli.Context, positional []string, jsonMode bool) error {
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
				return scaffoldFromLibrary(c, lib, entry, positional, jsonMode)
			}
		}
	}

	// Resolve template reference (handles both local and remote)
	resolver, err := remote.NewResolver()
	if err != nil {
		return app.Errorf("failed to create resolver: %w", err)
	}

	resolveResult, err := resolver.Resolve(c.Context, templateRef, remote.ResolveOptions{
		ForceUpdate: c.Bool("update"),
	})
	if err != nil {
		return app.Errorf("failed to resolve template: %w", err)
	}
	templateDir := resolveResult.Path

	// Parse meta flags
	meta, err := parse.ParseKeyValues(c.StringSlice("meta"), true)
	if err != nil {
		return app.Errorf("invalid meta flag: %w", err)
	}

	// Determine if the template is remote
	isRemote := !remote.IsLocal(templateRef)

	// Build options
	opts := buildScaffoldOpts(c, templateDir, projectName, meta, jsonMode)
	opts.TemplateRef = templateRef
	opts.IsRemote = isRemote
	if isRemote {
		opts.TemplateName = remote.DeriveName(templateRef)
	}
	if jsonMode {
		opts.NoInput = true
	}

	// Decide whether to add the template to the library.
	// Remote templates are always added; local templates are added when
	// --add-to-lib is set or the user confirms interactively.
	addToLib := isRemote
	if !isRemote {
		addToLib = resolveAddToLib(c, templateDir, jsonMode)
	}
	if addToLib {
		opts.SkipGeneratorCopy = true
	}

	// Verify template lockfile for remote templates.
	if isRemote {
		if lockErr := verifyTemplateLock(templateRef, templateDir, opts.UpdateLock, opts.IgnoreLock); lockErr != nil {
			return app.Errorf("lockfile check failed: %w", lockErr)
		}
	}

	// Create and run scaffold
	if err := runScaffold(c, opts, jsonMode, func(ccErr *scaffold.CookiecutterDetectedError) error {
		return handleCookiecutterDetection(c, ccErr, templateRef, templateDir, opts, jsonMode)
	}); err != nil {
		return err
	}

	// Add the template to the library for generator resolution.
	if addToLib {
		addToLibrary(c, templateRef, templateDir, jsonMode)
	}

	return nil
}

// resolveAddToLib determines whether a local template should be added to the
// library after scaffolding. Returns true if --add-to-lib is set, or if the
// template has generators and the user confirms interactively.
func resolveAddToLib(c *cli.Context, templateDir string, jsonMode bool) bool {
	if c.Bool(flags.AddToLibFlag) {
		return true
	}

	// Only prompt if the template has generators/bundles worth installing.
	hasGenerators := hasSubdirScaffold(templateDir, types.TemplatesDir) || hasSubdirScaffold(templateDir, types.GeneratorsDir)
	if !hasGenerators {
		return false
	}

	// Non-interactive mode: don't add (safe default, generators copied to .tag/).
	if nonInteractive(c, jsonMode) {
		return false
	}

	prompter := newPrompter()
	add, err := prompter.Confirm("Add template to library? (enables generator resolution without copying .tag/)", false)
	if err != nil {
		return false
	}
	return add
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
func resolveTemplateName(c *cli.Context, lib *library.Library, positional []string, jsonMode bool) (string, error) {
	switch {
	case len(positional) >= 1:
		return positional[0], nil
	case !nonInteractive(c, jsonMode):
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
			return "", &app.CommandError{Message: "cancelled", Code: app.ExitInterrupted}
		}
		return "", err
	}

	return result, nil
}

// scaffoldFlags returns the full set of flags for the scaffold command.
func scaffoldFlags() []cli.Flag {
	return append(commonScaffoldFlags(),
		&cli.BoolFlag{
			Name:    "update",
			Aliases: []string{"u"},
			Usage:   "Force refresh of cached remote templates",
		},
		formatFlag(formatText, formatJSON),
	)
}

// handleCookiecutterDetection handles the case when a Cookiecutter template is detected.
// Output convention: fmt for user-facing messages, slog for diagnostic messages.
func handleCookiecutterDetection(c *cli.Context, _ *scaffold.CookiecutterDetectedError, templateRef, templateDir string, opts scaffold.Options, jsonMode bool) error {
	// In non-interactive mode, fail with helpful error
	if nonInteractive(c, jsonMode) {
		return app.Errorf("This appears to be a Cookiecutter template.\n"+
			"Cannot convert in non-interactive mode.\n"+
			"Run without --no-input to convert interactively, or use:\n"+
			"  tag convert cookiecutter %s",
			templateRef)
	}

	prompter := newPrompter()

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

	// Retry scaffolding with converted template — reuse runScaffold to
	// ensure dialect loading and all other initialization happens correctly.
	return runScaffold(c, opts, jsonMode, func(*scaffold.CookiecutterDetectedError) error {
		return app.Errorf("unexpected Cookiecutter detection after conversion")
	})
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
func addToLibrary(c *cli.Context, templateRef, templateDir string, jsonMode bool) {
	w := c.App.Writer
	if jsonMode {
		w = io.Discard
	}

	lib, err := newLocalLibrary()
	if err != nil {
		slog.Warn("could not add template to library", "error", err)
		return
	}

	name := remote.DeriveName(templateRef)

	// Skip if the template is already in the library.
	if _, getErr := lib.Get(name); getErr == nil {
		fmt.Fprintf(w, "\nTemplate %q already in library. Run with: tag scaffold %s\n", name, name)
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

	fmt.Fprintf(w, "\nTemplate added to library as %q. Run with: tag scaffold %s\n", result.Name, result.Name)
}

// suggestConvertedTemplateName generates a default name for converted template output.
func suggestConvertedTemplateName(templateRef string) string {
	return remote.DeriveName(templateRef) + "-tag"
}

// displayScaffoldSummary prints a post-scaffold summary to w.
func displayScaffoldSummary(w io.Writer, result scaffold.ScaffoldResult) {
	projectRoot := result.ProjectRoot
	templateDir := result.TemplateDir
	vars := result.Vars
	opts := result.Opts

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Scaffolding complete!")
	fmt.Fprintf(w, "  Output: %s\n", projectRoot)

	// Show key variables
	if projectName, ok := vars["project_name"].(string); ok {
		fmt.Fprintf(w, "  Project: %s\n", projectName)
	}

	// Show template origin
	if opts.TemplateName != "" {
		version := ""
		if opts.TemplateVersion != "" {
			version = " (" + opts.TemplateVersion + ")"
		}
		fmt.Fprintf(w, "  Template: %s%s\n", opts.TemplateRef, version)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Next steps:")
	fmt.Fprintf(w, "  cd %s\n", projectRoot)

	// Check if the template has generators
	hasGenerators := hasSubdirScaffold(templateDir, types.TemplatesDir) || hasSubdirScaffold(templateDir, types.GeneratorsDir)

	if hasGenerators {
		fmt.Fprintln(w, "  tag generate list    # see available generators")
	}
	fmt.Fprintln(w)

	// Display template README if present
	readmePath := filepath.Join(templateDir, types.TemplateReadme)
	if content, err := os.ReadFile(readmePath); err == nil && len(content) > 0 {
		rendered, err := glamour.Render(string(content), "auto")
		if err != nil {
			// Fallback: print raw markdown
			fmt.Fprintln(w, string(content))
		} else {
			fmt.Fprint(w, rendered)
		}
	}
}

// hasSubdirScaffold checks if a directory contains a named subdirectory.
func hasSubdirScaffold(dir, subdir string) bool {
	info, err := os.Stat(filepath.Join(dir, subdir))
	return err == nil && info.IsDir()
}

// verifyTemplateLock checks or creates a lockfile entry for a remote template.
// projectRoot is the current working directory where .tag/lock.json lives.
func verifyTemplateLock(templateRef, templateDir string, updateLock, ignoreLock bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}
	return lockfile.VerifyAndMaybeUpdate(cwd, templateRef, templateDir, lockfile.VerifyOptions{
		UpdateLock: updateLock,
		IgnoreLock: ignoreLock,
	})
}
