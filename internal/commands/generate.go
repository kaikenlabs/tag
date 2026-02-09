package commands

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/internal/types/flags"
	"github.com/kaikenlabs/tag/pkg/app"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/chalk"
	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/engine"
	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/template"
)

// newEngine is a function variable that creates a new engine.
// It can be replaced in tests to inject a mock generator.
var newEngine = engine.NewGenerator

// newBundleEngine is a function variable that creates an engine with a shared template engine.
// It can be replaced in tests to inject a mock generator.
var newBundleEngine = engine.NewGeneratorWithEngine

func GenerateCommand(cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name: "generate",
		Usage: fmt.Sprintf("runs a %s with the specified %s passing the arguments %s. Use %s if you want to generate a bundle",
			chalk.Green("bundle or generator"),
			chalk.Green("name"),
			chalk.Green("args"),
			chalk.Yellow("'-b or --bundle'")),
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

func generateBundle(c *cli.Context, cfg *config.Config, generatorName, targetName, args string) error {
	if !c.Bool("no-hooks") {
		if err := runHooks(cfg.Hooks.Pre, scaffold.HookPhasePreGen); err != nil {
			return err
		}
	}

	dirPath := filepath.Join(cfg.Env.Path, c.Path(flags.BundlePathFlag), generatorName, generatorName+types.BundleExtension)

	if err := ValidatePathContainment(cfg.Env.Path, dirPath); err != nil {
		return app.Errorf("path safety check failed: %w", err)
	}

	data, err := os.ReadFile(dirPath)
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
	sharedPath := filepath.Join(cfg.Env.Path, c.Path(flags.SharedPathFlag))

	slog.Info(chalk.Green("running bundle"), "bundle", generatorName, "target", targetName)
	for _, generator := range bundle.Generators {
		genDirPath := filepath.Join(cfg.Env.Path, generator.Name)

		if err := ValidatePathContainment(cfg.Env.Path, genDirPath); err != nil {
			return app.Errorf("path safety check failed: %w", err)
		}
		if _, err := os.ReadDir(genDirPath); err != nil {
			return app.Errorf("generator %s not found in: %s", generator.Name, cfg.Env.Path)
		}

		gen, err := newBundleEngine(tmplEngine, dryRun, genDirPath, sharedPath)
		if err != nil {
			return app.Errorf("error creating engine: %w", err)
		}

		err = gen.Generate(engine.Data{Name: targetName, Args: args, MetaArgs: c.StringSlice(flags.MetaFlag)})
		if err != nil {
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

	dirPath := filepath.Join(cfg.Env.Path, generatorName)

	if err := ValidatePathContainment(cfg.Env.Path, dirPath); err != nil {
		return app.Errorf("path safety check failed: %w", err)
	}

	_, err := os.ReadDir(dirPath)
	if err != nil {
		return app.Errorf("generator %s not found in: %s", generatorName, cfg.Env.Path)
	}
	sharedPath := filepath.Join(cfg.Env.Path, c.Path(flags.SharedPathFlag))

	slog.Info(chalk.Green("running generator"), "generator", generatorName, "target", targetName)

	gen, err := newEngine(c.Bool(flags.DryRunFlag), dirPath, sharedPath)
	if err != nil {
		return app.Errorf("error creating engine: %w", err)
	}

	err = gen.Generate(engine.Data{Name: targetName, Args: args, MetaArgs: c.StringSlice(flags.MetaFlag)})
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
