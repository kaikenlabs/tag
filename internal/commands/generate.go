package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/chalk"
	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/engine"
	"github.com/kaikenlabs/tag/internal/library"
	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/internal/types/flags"
	"github.com/kaikenlabs/tag/pkg/app"
)

// newEngine is a function variable that creates a new engine.
// It can be replaced in tests to inject a mock generator.
var newEngine = engine.NewGenerator

// newBundleEngine is a function variable that creates an engine with a shared template engine.
// It can be replaced in tests to inject a mock generator.
var newBundleEngine = engine.NewGeneratorWithEngine

// GeneratorNotFoundError is returned when a generator cannot be found in any source.
type GeneratorNotFoundError struct {
	Generator string
	Template  string // library template name (empty if no template)
	Source    string // template source ref (for helpful message)
	LocalPath string // local .tag.templates/ path
}

func (e *GeneratorNotFoundError) Error() string {
	if e.Template != "" {
		return fmt.Sprintf("generator %q not found in template %q or local path.\n"+
			"Ensure the template is in the library: tag lib add %s", e.Generator, e.Template, e.Source)
	}
	return fmt.Sprintf("generator %q not found in %s", e.Generator, e.LocalPath)
}

func GenerateCommand(cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name:  "generate",
		Usage: "Run a generator or bundle",
		Subcommands: []*cli.Command{
			generateListCommand(cfg),
		},
		Args:      true,
		ArgsUsage: "<bundle-or-generator> <name> <args>",
		Action: func(c *cli.Context) error {
			return generateAction(c, cfg)
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "bundle",
				Aliases: []string{"b"},
				Value:   false,
				Usage:   "Runs a bundle instead of a generator",
			},
			&cli.StringSliceFlag{
				Name:    "meta",
				Usage:   "Specifies metadata to include into the generators",
				Aliases: []string{"m"},
			},
			&cli.BoolFlag{
				Name:  "no-hooks",
				Value: false,
				Usage: "Skip execution of pre and post hooks",
			},
		},
	}
}

func generateListCommand(cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name:    "list",
		Aliases: []string{"ls"},
		Usage:   "List available generators and bundles",
		Action: func(c *cli.Context) error {
			return generateList(cfg, os.Stdout)
		},
	}
}

// generatorInfo holds display information about a generator or bundle.
type generatorInfo struct {
	Name        string
	Description string
	Source      string // "template" or "local"
}

func generateList(cfg *config.Config, w io.Writer) error {
	if err := config.CheckConfig(cfg); err != nil {
		return err
	}

	var templateGens, localGens []generatorInfo
	var templateBundles, localBundles []generatorInfo
	var templateName, templateSource, templateVersion string

	// 1. Collect generators from library template
	if cfg.HasTemplateOrigin() {
		templateName = cfg.Template.Name
		templateSource = cfg.Template.Source
		templateVersion = cfg.Template.Version

		lib, err := newLocalLibrary()
		if err == nil {
			templateDir, pathErr := lib.TemplatePath(templateName)
			if pathErr == nil {
				templateGens = scanGenerators(filepath.Join(templateDir, types.TemplatesDir))
				templateBundles = scanBundles(filepath.Join(templateDir, types.TemplatesDir, types.BundlesDir))
			}
		}
	}

	// 2. Collect generators from local .tag.templates/
	if cfg.Env.Path != "" {
		localGens = scanGenerators(cfg.Env.Path)
		localBundles = scanBundles(filepath.Join(cfg.Env.Path, cfg.Env.BundlePath))
	}

	// Check if there's anything to show
	if len(templateGens) == 0 && len(localGens) == 0 && len(templateBundles) == 0 && len(localBundles) == 0 {
		fmt.Fprintln(w, "No generators found.")
		if templateName == "" {
			fmt.Fprintln(w)
			fmt.Fprintln(w, "This project was not scaffolded from a library template.")
			fmt.Fprintln(w, "Create generators in .tag.templates/ or scaffold from a template with generators.")
		}
		return nil
	}

	// Print header
	if templateName != "" {
		version := ""
		if templateVersion != "" {
			version = "@" + templateVersion
		}
		fmt.Fprintf(w, "Generators for this project (template: %s%s)\n\n", templateSource, version)
	} else {
		fmt.Fprintln(w, "Available generators:")
		fmt.Fprintln(w)
	}

	// Print template generators
	if len(templateGens) > 0 {
		fmt.Fprintf(w, "  %s (%s)\n", chalk.Green("TEMPLATE GENERATORS"), templateName)
		for _, g := range templateGens {
			printGeneratorLine(w, g)
		}
		fmt.Fprintln(w)
	}

	// Print local generators
	if len(localGens) > 0 {
		fmt.Fprintf(w, "  %s\n", chalk.Green("PROJECT GENERATORS"))
		for _, g := range localGens {
			printGeneratorLine(w, g)
		}
		fmt.Fprintln(w)
	}

	// Print bundles
	if len(templateBundles) > 0 || len(localBundles) > 0 {
		fmt.Fprintf(w, "  %s\n", chalk.Green("BUNDLES"))
		for _, b := range templateBundles {
			printGeneratorLine(w, b)
		}
		for _, b := range localBundles {
			printGeneratorLine(w, b)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "Run: tag generate <name> <target> [args]")
	return nil
}

func printGeneratorLine(w io.Writer, g generatorInfo) {
	if g.Description != "" {
		fmt.Fprintf(w, "  %-20s %s\n", g.Name, g.Description)
	} else {
		fmt.Fprintf(w, "  %s\n", g.Name)
	}
}

// scanGenerators scans a directory for generator subdirectories.
// Directories starting with _ are skipped (reserved: _shared, _bundles).
func scanGenerators(dir string) []generatorInfo {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var result []generatorInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Skip reserved directories (prefixed with _ or .)
		if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}

		info := generatorInfo{Name: name}

		// Try to read description from tag.template.json
		configPath := filepath.Join(dir, name, types.TemplateConfigFile)
		data, err := os.ReadFile(configPath)
		if err == nil {
			if tc, parseErr := scaffold.ParseTemplateConfig(data); parseErr == nil {
				info.Description = tc.Description
			}
		}

		result = append(result, info)
	}
	return result
}

// scanBundles scans a bundles directory for bundle definitions.
func scanBundles(dir string) []generatorInfo {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var result []generatorInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}

		info := generatorInfo{Name: name}

		// Bundle JSON exists but we use directory name for display consistency

		result = append(result, info)
	}
	return result
}

func generateAction(c *cli.Context, cfg *config.Config) error {
	if err := config.CheckConfig(cfg); err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return app.Errorf("configuration error: %w", err)
	}

	if c.Args().Len() < 2 {
		return app.Errorf("please provide the generator/bundle and the name")
	}

	generatorOrBundleName := c.Args().Get(0)
	targetName := c.Args().Get(1)

	// Validate names for path safety
	if err := ValidateNameSafe(generatorOrBundleName); err != nil {
		return app.Errorf("invalid generator/bundle name: %w", err)
	}
	if err := ValidateNameSafe(targetName); err != nil {
		return app.Errorf("invalid target name: %w", err)
	}

	var args string
	if c.Args().Len() > 2 {
		args = c.Args().Get(2)
	}

	if c.Bool("bundle") {
		return generateBundle(c, cfg, generatorOrBundleName, targetName, args)
	}
	return generateTemplate(c, cfg, generatorOrBundleName, targetName, args)
}

// resolveGeneratorPaths resolves the generator directory and shared path using
// library-first, local-fallback resolution. When a .tagconfig.json references
// a library template, generators from that template are preferred.
func resolveGeneratorPaths(cfg *config.Config, name string) (genDir, sharedDir string, err error) {
	// 1. Try library template
	if cfg.HasTemplateOrigin() {
		genDir, sharedDir, found, libErr := resolveFromLibrary(cfg, name)
		if libErr != nil {
			return "", "", libErr
		}
		if found {
			return genDir, sharedDir, nil
		}
	}

	// 2. Fall back to local .tag.templates/
	if cfg.Env.Path != "" {
		candidate := filepath.Join(cfg.Env.Path, name)
		if _, statErr := os.Stat(candidate); statErr == nil {
			sharedName := cfg.Env.SharedPath
			if sharedName == "" {
				sharedName = types.SharedDir
			}
			shared := filepath.Join(cfg.Env.Path, sharedName)
			return candidate, shared, nil
		}
	}

	// 3. Not found
	if cfg.HasTemplateOrigin() {
		return "", "", app.Errorf("%w", &GeneratorNotFoundError{
			Generator: name,
			Template:  cfg.Template.Name,
			Source:    cfg.Template.Source,
		})
	}
	return "", "", app.Errorf("%w", &GeneratorNotFoundError{
		Generator: name,
		LocalPath: cfg.Env.Path,
	})
}

// resolveFromLibrary attempts to find a generator in the library template.
// Returns (genDir, sharedDir, found, error). When found is false and error is nil,
// the caller should fall through to local resolution.
func resolveFromLibrary(cfg *config.Config, name string) (string, string, bool, error) {
	lib, err := newLocalLibrary()
	if err != nil {
		return "", "", false, fmt.Errorf("failed to initialize library: %w", err)
	}

	templateDir, err := lib.TemplatePath(cfg.Template.Name)
	if err != nil {
		// Only fall through on ErrTemplateNotFound (cache miss).
		if !errors.Is(err, library.ErrTemplateNotFound) {
			return "", "", false, fmt.Errorf("error accessing library template %q: %w", cfg.Template.Name, err)
		}
		slog.Debug("template not found in library, falling back to local", "template", cfg.Template.Name)
		return "", "", false, nil
	}

	candidate := filepath.Join(templateDir, types.TemplatesDir, name)
	if _, statErr := os.Stat(candidate); statErr == nil {
		shared := filepath.Join(templateDir, types.TemplatesDir, types.SharedDir)
		warnVersionMismatch(cfg, templateDir)
		return candidate, shared, true, nil
	}

	// Generator not found in library template — fall through to local
	return "", "", false, nil
}

// warnVersionMismatch prints a warning if the library template version differs
// from the scaffold-time version recorded in .tagconfig.json.
func warnVersionMismatch(cfg *config.Config, templateDir string) {
	if cfg.Template.Version == "" {
		return
	}
	libVersion, _, _ := library.ReadTemplateMetadata(templateDir)
	if libVersion != "" && libVersion != cfg.Template.Version {
		fmt.Fprintf(os.Stderr, "Warning: template version mismatch (scaffolded: %s, library: %s). "+
			"Consider re-scaffolding or running 'tag lib update %s'.\n",
			cfg.Template.Version, libVersion, cfg.Template.Name)
	}
}

// resolveBundlePath resolves the bundle JSON file path using library-first, local-fallback resolution.
func resolveBundlePath(cfg *config.Config, bundleName, bundleSubDir string) (string, error) {
	bundleFile := filepath.Join(bundleName, bundleName+types.BundleExtension)

	// 1. Try library template
	if cfg.HasTemplateOrigin() {
		path, err := resolveBundleFromLibrary(cfg, bundleFile)
		if err != nil {
			return "", err
		}
		if path != "" {
			return path, nil
		}
	}

	// 2. Fall back to local
	if cfg.Env.Path != "" {
		candidate := filepath.Join(cfg.Env.Path, bundleSubDir, bundleFile)
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate, nil
		}
	}

	return "", app.Errorf("cannot open bundle file: bundle %q not found", bundleName)
}

// resolveBundleFromLibrary attempts to find a bundle file in the library template.
// Returns ("", nil) when the bundle is not found (caller should fall through to local).
func resolveBundleFromLibrary(cfg *config.Config, bundleFile string) (string, error) {
	lib, err := newLocalLibrary()
	if err != nil {
		return "", fmt.Errorf("failed to initialize library: %w", err)
	}

	templateDir, err := lib.TemplatePath(cfg.Template.Name)
	if err != nil {
		if !errors.Is(err, library.ErrTemplateNotFound) {
			return "", fmt.Errorf("error accessing library template %q: %w", cfg.Template.Name, err)
		}
		return "", nil
	}

	candidate := filepath.Join(templateDir, types.TemplatesDir, types.BundlesDir, bundleFile)
	if _, statErr := os.Stat(candidate); statErr == nil {
		return candidate, nil
	}
	return "", nil
}

func generateBundle(c *cli.Context, cfg *config.Config, generatorName, targetName, args string) error {
	if !c.Bool("no-hooks") {
		if err := runHooks(cfg.Hooks.Pre, scaffold.HookPhasePreGen); err != nil {
			return err
		}
	}

	bundlePath, err := resolveBundlePath(cfg, generatorName, c.Path(flags.BundlePathFlag))
	if err != nil {
		return err
	}

	data, err := os.ReadFile(bundlePath)
	if err != nil {
		return app.Errorf("cannot open bundle file: %w", err)
	}

	var bundle engine.Bundle
	err = json.Unmarshal(data, &bundle)
	if err != nil {
		return app.Errorf("cannot decode bundle file: %w", err)
	}

	// Create shared template engine for all generators in the bundle.
	// This avoids re-creating the engine (and its cache) for each generator.
	tmplEngine, err := template.NewEngine()
	if err != nil {
		return app.Errorf("cannot create template engine: %w", err)
	}

	dryRun := c.Bool(flags.DryRunFlag)

	slog.Info(chalk.Green("running bundle"), "bundle", generatorName, "target", targetName)
	for _, generator := range bundle.Generators {
		// Resolve each generator independently (supports mixed-source bundles)
		genDirPath, sharedPath, resolveErr := resolveGeneratorPaths(cfg, generator.Name)
		if resolveErr != nil {
			return resolveErr
		}

		gen, genErr := newBundleEngine(tmplEngine, dryRun, genDirPath, sharedPath)
		if genErr != nil {
			return app.Errorf("error creating engine: %w", genErr)
		}

		genData := engine.Data{Name: targetName, Args: args, MetaArgs: c.StringSlice(flags.MetaFlag)}
		if cfg.Variables != nil {
			genData.ScaffoldVars = cfg.Variables
		}

		if err := gen.Generate(genData); err != nil {
			return app.Errorf("error when generating template: %w", err)
		}
	}

	if !c.Bool("no-hooks") {
		return runHooks(cfg.Hooks.Post, scaffold.HookPhasePostGen)
	}
	return nil
}

func generateTemplate(c *cli.Context, cfg *config.Config, generatorName, targetName, args string) error {
	if !c.Bool("no-hooks") {
		if err := runHooks(cfg.Hooks.Pre, scaffold.HookPhasePreGen); err != nil {
			return err
		}
	}

	// Resolve generator paths using library-first, local-fallback resolution
	dirPath, sharedPath, err := resolveGeneratorPaths(cfg, generatorName)
	if err != nil {
		return err
	}

	slog.Info(chalk.Green("running generator"), "generator", generatorName, "target", targetName)

	gen, err := newEngine(c.Bool(flags.DryRunFlag), dirPath, sharedPath)
	if err != nil {
		return app.Errorf("error creating engine: %w", err)
	}

	data := engine.Data{Name: targetName, Args: args, MetaArgs: c.StringSlice(flags.MetaFlag)}
	if cfg.Variables != nil {
		data.ScaffoldVars = cfg.Variables
	}

	err = gen.Generate(data)
	if err != nil {
		return app.Errorf("error when generating template: %w", err)
	}

	if !c.Bool("no-hooks") {
		return runHooks(cfg.Hooks.Post, scaffold.HookPhasePostGen)
	}
	return nil
}

func runHooks(hooks [][]string, phase scaffold.HookPhase) error {
	if len(hooks) == 0 {
		return nil
	}

	dir, err := os.Getwd()
	if err != nil {
		return app.Errorf("failed to get working directory: %w", err)
	}

	results, err := scaffold.RunArgvHooks(phase, hooks, dir, nil)

	scaffold.PrintHookResults(results)

	if err != nil {
		return app.Errorf("hook failed: %w", err)
	}

	return nil
}
