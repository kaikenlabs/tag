package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/chalk"
	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/engine"
	"github.com/kaikenlabs/tag/internal/history"
	"github.com/kaikenlabs/tag/internal/hooks"
	"github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/internal/types/flags"
	"github.com/kaikenlabs/tag/pkg/app"
)

// newEngine is a function variable that creates a new generator with optional history recording.
// It can be replaced in tests to inject a mock generator.
var newEngine = func(dryRun bool, dirPath, sharedPath string, rec *history.Recorder) (engine.Generator, error) {
	tmplEngine, err := template.NewEngine()
	if err != nil {
		return nil, fmt.Errorf("cannot create template engine: %w", err)
	}
	return engine.NewGeneratorWithRecorder(tmplEngine, dryRun, dirPath, sharedPath, rec)
}

// newBundleEngine is a function variable that creates a generator with a shared template engine and history recording.
// It can be replaced in tests to inject a mock generator.
var newBundleEngine = engine.NewGeneratorWithRecorder

func GenerateCommand(cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name:    "generate",
		Aliases: []string{"g"},
		Usage:   "Run a generator or bundle",
		Description: `Run a generator or bundle to create or modify files in an existing project.

TAG auto-resolves generators from the library template first, then falls back
to the local .tag/ directory. Bundles run multiple generators in sequence.

ARGUMENTS
  <bundle-or-generator>  Name of the generator or bundle to run
  <name>                 Entity name passed as {{ name }} in templates

FLAGS
  --meta, -m key=value        Override template variables (repeatable)
  --no-hooks                  Skip pre/post hooks defined in .tagconfig.json
  --on-existing=fail|skip|overwrite
                              Control behaviour when a file already exists (default: fail)
  --verbose, -v               Show per-file operation details in the summary
  --dry-run, -d               Preview changes without writing files (global flag)

EXAMPLES
  tag generate model User
  tag generate api-endpoint users --meta package=handlers
  tag generate crud Product --no-hooks
  tag generate crud Product --dry-run
  tag generate crud Product --on-existing=skip
  tag generate crud Product --on-existing=overwrite --verbose
  tag generate list                    # List available generators and bundles`,
		Subcommands: []*cli.Command{
			generateListCommand(cfg),
		},
		Args:      true,
		ArgsUsage: "<bundle-or-generator> <name> [args]",
		Action: func(c *cli.Context) error {
			return generateAction(c, cfg)
		},
		BashComplete: func(c *cli.Context) {
			// Only complete the first argument (generator/bundle name)
			if c.NArg() > 0 {
				return
			}
			completeGeneratorNames(cfg)
		},
		Flags: []cli.Flag{
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
			&cli.StringFlag{
				Name:  flags.OnExistingFlag,
				Usage: "Behaviour when a file to be created already exists: fail (default), skip, overwrite",
				Value: "",
			},
			&cli.BoolFlag{
				Name:    flags.VerboseFlag,
				Aliases: []string{"v"},
				Usage:   "Show per-file operation details in the summary",
			},
		},
	}
}

func generateAction(c *cli.Context, cfg *config.Config) error {
	if err := config.CheckConfig(cfg); err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return app.Errorf("configuration error: %w", err)
	}

	if c.Args().Len() < 2 {
		return app.UsageErrorf("please provide the generator/bundle and the name")
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

	// Validate --on-existing flag value.
	onExisting := engine.OnExistingPolicy(c.String(flags.OnExistingFlag))
	if !onExisting.IsValid() {
		return app.UsageErrorf("invalid --on-existing value %q: must be one of fail, skip, overwrite", c.String(flags.OnExistingFlag))
	}

	target, err := resolveGenerateTarget(cfg, generatorOrBundleName, c.Path(flags.BundlePathFlag))
	if err != nil {
		return err
	}

	if target.IsBundle {
		return generateBundle(c, cfg, generatorOrBundleName, targetName, target.BundlePath, onExisting)
	}
	return generateTemplate(c, cfg, generatorOrBundleName, targetName, target.GenDir, target.SharedDir, onExisting)
}

func generateBundle(c *cli.Context, cfg *config.Config, generatorName, targetName, bundlePath string, onExisting engine.OnExistingPolicy) error {
	if !c.Bool("no-hooks") {
		if err := runHooks(cfg.Hooks.Pre, hooks.HookPhasePreGen, cfg.Variables); err != nil {
			return err
		}
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
	tagDir := types.TemplatesDir

	// Create a single recorder shared across all generators in the bundle so
	// that first-touch semantics for hash_before work correctly.
	rec := history.NewRecorder(tagDir)

	var totalResult engine.GenerateResult
	slog.Info(chalk.Green("running bundle"), "bundle", generatorName, "target", targetName)
	for _, generator := range bundle.Generators {
		var genDirPath, sharedPath string
		if bundle.SelfContained {
			if err := ValidateNameSafe(generator.Name); err != nil {
				return app.Errorf("invalid generator name in bundle: %w", err)
			}
			bundleDir := filepath.Dir(bundlePath)
			genDirPath = filepath.Join(bundleDir, generator.Name)
			sharedPath = filepath.Join(bundleDir, types.SharedDir)
			if _, statErr := os.Stat(genDirPath); statErr != nil {
				return app.Errorf("generator %q not found in self-contained bundle %q (expected at %s)",
					generator.Name, generatorName, genDirPath)
			}
		} else {
			var resolveErr error
			genDirPath, sharedPath, resolveErr = resolveGeneratorPaths(cfg, generator.Name)
			if resolveErr != nil {
				return resolveErr
			}
		}

		gen, genErr := newBundleEngine(tmplEngine, dryRun, genDirPath, sharedPath, rec)
		if genErr != nil {
			return app.Errorf("error creating engine: %w", genErr)
		}

		genData := engine.Data{
			Name:       targetName,
			RawMeta:    c.StringSlice(flags.MetaFlag),
			OnExisting: onExisting,
		}
		if cfg.Variables != nil {
			genData.ScaffoldVars = cfg.Variables
		}

		genResult, genRunErr := runGenerate(gen, genData)
		if genRunErr != nil {
			// Print partial progress before surfacing the error so the user
			// knows how many operations completed before the failure.
			if totalResult.Created+totalResult.Skipped+totalResult.Overwritten+totalResult.Modified > 0 {
				printGenerateSummary(os.Stdout, totalResult, c.Bool(flags.VerboseFlag))
			}
			return genRunErr
		}
		totalResult.Created += genResult.Created
		totalResult.Skipped += genResult.Skipped
		totalResult.Overwritten += genResult.Overwritten
		totalResult.Modified += genResult.Modified
		totalResult.Details = append(totalResult.Details, genResult.Details...)
	}

	if !c.Bool("no-hooks") {
		if err := runHooks(cfg.Hooks.Post, hooks.HookPhasePostGen, cfg.Variables); err != nil {
			return err
		}
	}

	if !dryRun {
		gen := rec.Build(generatorName, "generate")
		if appendErr := history.Append(tagDir, gen); appendErr != nil {
			slog.Warn("could not write history manifest", "error", appendErr)
		}
	}

	printGenerateSummary(os.Stdout, totalResult, c.Bool(flags.VerboseFlag))
	return nil
}

func generateTemplate(c *cli.Context, cfg *config.Config, generatorName, targetName, dirPath, sharedPath string, onExisting engine.OnExistingPolicy) error {
	if !c.Bool("no-hooks") {
		if err := runHooks(cfg.Hooks.Pre, hooks.HookPhasePreGen, cfg.Variables); err != nil {
			return err
		}
	}

	slog.Info(chalk.Green("running generator"), "generator", generatorName, "target", targetName)

	dryRun := c.Bool(flags.DryRunFlag)
	tagDir := types.TemplatesDir
	rec := history.NewRecorder(tagDir)

	gen, err := newEngine(dryRun, dirPath, sharedPath, rec)
	if err != nil {
		return app.Errorf("error creating engine: %w", err)
	}

	data := engine.Data{
		Name:       targetName,
		RawMeta:    c.StringSlice(flags.MetaFlag),
		OnExisting: onExisting,
	}
	if cfg.Variables != nil {
		data.ScaffoldVars = cfg.Variables
	}

	result, err := runGenerate(gen, data)
	if err != nil {
		return err
	}

	if !c.Bool("no-hooks") {
		if err := runHooks(cfg.Hooks.Post, hooks.HookPhasePostGen, cfg.Variables); err != nil {
			return err
		}
	}

	if !dryRun {
		histGen := rec.Build(generatorName, "generate")
		if appendErr := history.Append(tagDir, histGen); appendErr != nil {
			slog.Warn("could not write history manifest", "error", appendErr)
		}
	}

	printGenerateSummary(os.Stdout, result, c.Bool(flags.VerboseFlag))
	return nil
}

func runHooks(hookCmds [][]string, phase hooks.HookPhase, vars map[string]any) error {
	if len(hookCmds) == 0 {
		return nil
	}

	dir, err := os.Getwd()
	if err != nil {
		return app.Errorf("failed to get working directory: %w", err)
	}

	var env []string
	if len(vars) > 0 {
		env = hooks.BuildVarEnv(vars, os.Stderr)
	}

	results, err := hooks.RunArgvHooks(phase, hookCmds, dir, env)

	hooks.PrintHookResults(results, os.Stdout)

	if err != nil {
		return app.Errorf("hook failed: %w", err)
	}

	return nil
}

// printGenerateSummary prints a post-generation summary line and, when verbose
// is true, prints the per-file operation details.
// runGenerate executes a single generator and routes errors to appropriate
// user-facing messages, distinguishing conflict errors from unexpected failures.
func runGenerate(gen engine.Generator, data engine.Data) (engine.GenerateResult, error) {
	result, err := gen.Generate(data)
	if err != nil {
		var ce *engine.ConflictError
		if errors.As(err, &ce) {
			return result, app.Errorf("%w", err)
		}
		return result, app.Errorf("error when generating template: %w", err)
	}
	return result, nil
}

func printGenerateSummary(w io.Writer, result engine.GenerateResult, verbose bool) {
	if verbose {
		for _, d := range result.Details {
			fmt.Fprintf(w, "  %-12s %s\n", d.Op, d.Path)
		}
	}
	fmt.Fprintf(w, "Generated: %d created, %d skipped, %d overwritten, %d modified\n",
		result.Created, result.Skipped, result.Overwritten, result.Modified)
}
