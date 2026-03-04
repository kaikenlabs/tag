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
	"github.com/kaikenlabs/tag/internal/tmplconfig"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/internal/types/flags"
	"github.com/kaikenlabs/tag/internal/writer"
	"github.com/kaikenlabs/tag/pkg/app"
)

// GeneratorFactory creates a generator for single-template execution.
type GeneratorFactory func(dryRun bool, dirPath, sharedPath string, rec *history.Recorder, out io.Writer) (engine.Generator, error)

// BundleGeneratorFactory creates a generator for bundle execution, sharing a template engine across generators.
type BundleGeneratorFactory func(tmplEngine *template.Engine, dryRun bool, dirPath, sharedPath string, rec *history.Recorder, out io.Writer) (engine.Generator, error)

// generatorFactories holds the factory functions used to create engine generators.
// Tests can substitute these to inject mock generators without mutating global state.
type generatorFactories struct {
	newEngine       GeneratorFactory
	newBundleEngine BundleGeneratorFactory
}

func defaultGeneratorFactories() generatorFactories {
	return generatorFactories{
		newEngine: func(dryRun bool, dirPath, sharedPath string, rec *history.Recorder, out io.Writer) (engine.Generator, error) {
			tmplEngine, err := template.NewEngine()
			if err != nil {
				return nil, fmt.Errorf("cannot create template engine: %w", err)
			}
			return engine.NewGeneratorWithRecorder(tmplEngine, dryRun, dirPath, sharedPath, rec, out)
		},
		newBundleEngine: engine.NewGeneratorWithRecorder,
	}
}

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
  --update-lock               Refresh the lockfile entry for the template
  --ignore-lock               Skip lockfile verification for this run

EXAMPLES
  tag generate model User
  tag generate api-endpoint users --meta package=handlers
  tag generate crud Product --no-hooks
  tag generate crud Product --dry-run
  tag generate crud Product --on-existing=skip
  tag generate crud Product --on-existing=overwrite --verbose
  tag generate list                    # List available generators and bundles
  tag generate info model              # Show JSON metadata for a generator or bundle
  tag generate agent-file claude       # Generate AI agent reference file`,
		Subcommands: []*cli.Command{
			generateListCommand(cfg),
			generateInfoCommand(cfg),
			generateAgentFileCommand(cfg),
		},
		Args:      true,
		ArgsUsage: "<bundle-or-generator> <name> [args]",
		Action: func(c *cli.Context) error {
			return generateAction(c, cfg, defaultGeneratorFactories())
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
				Name:  flags.NoHooksFlag,
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
			&cli.BoolFlag{
				Name:  flags.UpdateLockFlag,
				Usage: "Refresh the lockfile entry for the template (accepts new version/checksum)",
			},
			&cli.BoolFlag{
				Name:  flags.IgnoreLockFlag,
				Usage: "Skip lockfile verification for this run (a warning will be printed)",
			},
		},
	}
}

func generateAction(c *cli.Context, cfg *config.Config, fac generatorFactories) error {
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
		return generateBundle(c, cfg, fac, generatorOrBundleName, targetName, target.BundlePath, onExisting)
	}
	return generateTemplate(c, cfg, fac, generatorOrBundleName, targetName, target.GenDir, target.SharedDir, onExisting)
}

// generateFunc is the core generation work executed between pre/post hooks.
// rec is the shared history recorder to pass to the engine.
type generateFunc func(rec *history.Recorder) (engine.GenerateResult, error)

// generateWithHooks runs pre-hooks, calls fn for the core generation logic, runs
// post-hooks, records history, and prints the final summary.
// When fn returns an error it may have already printed a partial-progress summary;
// generateWithHooks returns the error immediately without printing again.
func generateWithHooks(c *cli.Context, cfg *config.Config, generatorName string, fn generateFunc) error {
	if !c.Bool(flags.NoHooksFlag) {
		if err := runHooks(cfg.Hooks.Pre, hooks.HookPhasePreGen, cfg.Variables, c.App.Writer); err != nil {
			return err
		}
	}

	dryRun := c.Bool(flags.DryRunFlag)
	rec := history.NewRecorder(types.TemplatesDir)

	result, err := fn(rec)
	if err != nil {
		if errors.Is(err, writer.ErrUserQuit) {
			fmt.Fprintln(c.App.Writer, "\nDry-run review stopped.")
			return nil
		}
		return err
	}

	if !c.Bool(flags.NoHooksFlag) {
		if err := runHooks(cfg.Hooks.Post, hooks.HookPhasePostGen, cfg.Variables, c.App.Writer); err != nil {
			return err
		}
	}

	if !dryRun {
		histGen := rec.Build(generatorName, "generate")
		if appendErr := history.Append(types.TemplatesDir, histGen); appendErr != nil {
			slog.Warn("could not write history manifest", "error", appendErr)
		}
	}

	printGenerateSummary(c.App.Writer, result, c.Bool(flags.VerboseFlag))
	return nil
}

func generateBundle(c *cli.Context, cfg *config.Config, fac generatorFactories, generatorName, targetName, bundlePath string, onExisting engine.OnExistingPolicy) error {
	data, err := os.ReadFile(bundlePath)
	if err != nil {
		return app.Errorf("cannot open bundle file: %w", err)
	}

	var bundle engine.Bundle
	if err = json.Unmarshal(data, &bundle); err != nil {
		return app.Errorf("cannot decode bundle file: %w", err)
	}

	// Check bundle prerequisites before running any generators.
	vars := make(map[string]any)
	if cfg.Variables != nil {
		vars = cfg.Variables
	}
	if reqErr := checkRequirements(generatorName, "bundle", bundle.Requires, vars); reqErr != nil {
		return reqErr
	}

	tmplEngine, err := template.NewEngine()
	if err != nil {
		return app.Errorf("cannot create template engine: %w", err)
	}

	slog.Info(chalk.Green("running bundle"), "bundle", generatorName, "target", targetName)
	return generateWithHooks(c, cfg, generatorName, func(rec *history.Recorder) (engine.GenerateResult, error) {
		var total engine.GenerateResult
		for _, generator := range bundle.Generators {
			var genDirPath, sharedPath string
			if bundle.SelfContained {
				if err := ValidateNameSafe(generator.Name); err != nil {
					return total, app.Errorf("invalid generator name in bundle: %w", err)
				}
				bundleDir := filepath.Dir(bundlePath)
				genDirPath = filepath.Join(bundleDir, generator.Name)
				sharedPath = filepath.Join(bundleDir, types.SharedDir)
				if _, statErr := os.Stat(genDirPath); statErr != nil {
					return total, app.Errorf("generator %q not found in self-contained bundle %q (expected at %s)",
						generator.Name, generatorName, genDirPath)
				}
			} else {
				var resolveErr error
				genDirPath, sharedPath, resolveErr = resolveGeneratorPaths(cfg, generator.Name)
				if resolveErr != nil {
					return total, resolveErr
				}
			}

			gen, genErr := fac.newBundleEngine(tmplEngine, c.Bool(flags.DryRunFlag), genDirPath, sharedPath, rec, c.App.Writer)
			if genErr != nil {
				return total, app.Errorf("error creating engine: %w", genErr)
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
				// Print partial progress so the user knows how many operations
				// completed before the failure.
				if total.Created+total.Skipped+total.Overwritten+total.Modified > 0 {
					printGenerateSummary(c.App.Writer, total, c.Bool(flags.VerboseFlag))
				}
				return total, genRunErr
			}
			total.Add(genResult)
		}
		return total, nil
	})
}

func generateTemplate(c *cli.Context, cfg *config.Config, fac generatorFactories, generatorName, targetName, dirPath, sharedPath string, onExisting engine.OnExistingPolicy) error {
	// Check generator prerequisites from tag.template.json if present.
	vars := make(map[string]any)
	if cfg.Variables != nil {
		vars = cfg.Variables
	}
	configPath := filepath.Join(dirPath, types.TemplateConfigFile)
	if data, readErr := os.ReadFile(configPath); readErr == nil {
		if tc, parseErr := tmplconfig.ParseTemplateConfig(data); parseErr == nil {
			if reqErr := checkRequirements(generatorName, "generator", tc.Requires, vars); reqErr != nil {
				return reqErr
			}
		}
	}

	slog.Info(chalk.Green("running generator"), "generator", generatorName, "target", targetName)
	return generateWithHooks(c, cfg, generatorName, func(rec *history.Recorder) (engine.GenerateResult, error) {
		gen, err := fac.newEngine(c.Bool(flags.DryRunFlag), dirPath, sharedPath, rec, c.App.Writer)
		if err != nil {
			return engine.GenerateResult{}, app.Errorf("error creating engine: %w", err)
		}
		data := engine.Data{
			Name:       targetName,
			RawMeta:    c.StringSlice(flags.MetaFlag),
			OnExisting: onExisting,
		}
		if cfg.Variables != nil {
			data.ScaffoldVars = cfg.Variables
		}
		return runGenerate(gen, data)
	})
}

func runHooks(hookCmds [][]string, phase hooks.HookPhase, vars map[string]any, w io.Writer) error {
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

	hooks.PrintHookResults(results, w)

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
