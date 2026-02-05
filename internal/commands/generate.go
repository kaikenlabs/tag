package commands

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gitlab.com/Vitrifi/tag/internal/types/flags"
	"gitlab.com/Vitrifi/tag/pkg/app"

	"github.com/urfave/cli/v2"
	"gitlab.com/Vitrifi/tag/internal/chalk"
	"gitlab.com/Vitrifi/tag/internal/config"
	"gitlab.com/Vitrifi/tag/internal/engine"
)

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
		},
	}
}

func generateAction(c *cli.Context, cfg *config.Config) error {
	config.CheckConfig(cfg)

	if c.Args().Len() < 2 {
		app.Terminate("please provide the generator/bundle and the name")
	}

	generatorOrBundleName := c.Args().Get(0)
	targetName := c.Args().Get(1)
	var args string
	if c.Args().Len() > 2 {
		args = c.Args().Get(2)
	}

	if c.Bool("bundle") {
		generateBundle(c, cfg, generatorOrBundleName, targetName, args)
	} else {
		generateTemplate(c, cfg, generatorOrBundleName, targetName, args, false)
	}
	return nil
}

func generateBundle(c *cli.Context, cfg *config.Config, generatorName, targetName, args string) {
	runHooks(cfg.Hooks.Pre)

	dirPath := filepath.Join(cfg.Env.Path, c.Path(flags.BundlePathFlag), generatorName, generatorName+BundleExtension)

	data, err := os.ReadFile(dirPath)
	if err != nil {
		app.Terminate("cannot open bundle file: %s", err.Error())
	}

	var bundle engine.Bundle
	err = json.Unmarshal(data, &bundle)
	if err != nil {
		app.Terminate("cannot decode bundle file: %s", err.Error())
	}

	slog.Info(chalk.Green("running bundle"), "bundle", generatorName, "target", targetName)
	for _, generator := range bundle.Generators {
		generateTemplate(c, cfg, generator.Name, targetName, args, true)
	}

	runHooks(cfg.Hooks.Post)
}

func generateTemplate(c *cli.Context, cfg *config.Config, generatorName, targetName, args string, inBundle bool) {
	if !inBundle {
		runHooks(cfg.Hooks.Pre)
	}

	dirPath := filepath.Join(cfg.Env.Path, generatorName)
	_, err := os.ReadDir(dirPath)
	if err != nil {
		app.Terminate("generator %s not found in: %s", generatorName, cfg.Env.Path)
	}
	sharedPath := filepath.Join(cfg.Env.Path, c.Path(flags.SharedPathFlag))

	if !inBundle {
		slog.Info(chalk.Green("running generator"), "generator", generatorName, "target", targetName)
	}

	e, err := engine.New(c.Bool(flags.DryRunFlag), dirPath, sharedPath, cfg.Env.Extension)
	if err != nil {
		app.Terminate("error creating engine: %s", err.Error())
	}

	err = e.Generate(engine.Data{Name: targetName, Args: args, MetaArgs: c.StringSlice(flags.MetaFlag)})
	if err != nil {
		app.Terminate("error when generating template: %s", err.Error())
	}

	if !inBundle {
		runHooks(cfg.Hooks.Post)
	}
}

func runHooks(hooks [][]string) {
	for _, hook := range hooks {
		hookString := strings.Join(hook, " ")
		slog.Info("running hook", "hook", hookString)
		if err := runCommand(hook); err != nil {
			app.Terminate("failed to run hook %s: %s", hookString, err.Error())
		}
	}
}

func runCommand(hook []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(dir, hook[0]), hook[1:]...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if output != nil {
		fmt.Println(string(output))
	}
	if err != nil {
		return err
	}
	return nil
}
