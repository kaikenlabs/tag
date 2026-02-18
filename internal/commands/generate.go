package commands

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/chalk"
	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/engine"
	"github.com/kaikenlabs/tag/internal/hooks"
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

func GenerateCommand(cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name:  "generate",
		Usage: "Run a generator or bundle",
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

	target, err := resolveGenerateTarget(cfg, generatorOrBundleName, c.Path(flags.BundlePathFlag))
	if err != nil {
		return err
	}

	if target.IsBundle {
		return generateBundle(c, cfg, generatorOrBundleName, targetName, args, target.BundlePath)
	}
	return generateTemplate(c, cfg, generatorOrBundleName, targetName, args, target.GenDir, target.SharedDir)
}

func generateBundle(c *cli.Context, cfg *config.Config, generatorName, targetName, args, bundlePath string) error {
	if !c.Bool("no-hooks") {
		if err := runHooks(cfg.Hooks.Pre, hooks.HookPhasePreGen); err != nil {
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

		gen, genErr := newBundleEngine(tmplEngine, dryRun, genDirPath, sharedPath)
		if genErr != nil {
			return app.Errorf("error creating engine: %w", genErr)
		}

		genData := engine.Data{Name: targetName, Args: args, RawMeta: c.StringSlice(flags.MetaFlag)}
		if cfg.Variables != nil {
			genData.ScaffoldVars = cfg.Variables
		}

		if err := gen.Generate(genData); err != nil {
			return app.Errorf("error when generating template: %w", err)
		}
	}

	if !c.Bool("no-hooks") {
		return runHooks(cfg.Hooks.Post, hooks.HookPhasePostGen)
	}
	return nil
}

func generateTemplate(c *cli.Context, cfg *config.Config, generatorName, targetName, args, dirPath, sharedPath string) error {
	if !c.Bool("no-hooks") {
		if err := runHooks(cfg.Hooks.Pre, hooks.HookPhasePreGen); err != nil {
			return err
		}
	}

	slog.Info(chalk.Green("running generator"), "generator", generatorName, "target", targetName)

	gen, err := newEngine(c.Bool(flags.DryRunFlag), dirPath, sharedPath)
	if err != nil {
		return app.Errorf("error creating engine: %w", err)
	}

	data := engine.Data{Name: targetName, Args: args, RawMeta: c.StringSlice(flags.MetaFlag)}
	if cfg.Variables != nil {
		data.ScaffoldVars = cfg.Variables
	}

	err = gen.Generate(data)
	if err != nil {
		return app.Errorf("error when generating template: %w", err)
	}

	if !c.Bool("no-hooks") {
		return runHooks(cfg.Hooks.Post, hooks.HookPhasePostGen)
	}
	return nil
}

func runHooks(hookCmds [][]string, phase hooks.HookPhase) error {
	if len(hookCmds) == 0 {
		return nil
	}

	dir, err := os.Getwd()
	if err != nil {
		return app.Errorf("failed to get working directory: %w", err)
	}

	results, err := hooks.RunArgvHooks(phase, hookCmds, dir, nil)

	hooks.PrintHookResults(results, os.Stdout)

	if err != nil {
		return app.Errorf("hook failed: %w", err)
	}

	return nil
}
