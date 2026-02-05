package commands

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"

	"gitlab.com/Vitrifi/tag/pkg/app"

	"gitlab.com/Vitrifi/tag/internal/engine"
	"gitlab.com/Vitrifi/tag/internal/types/flags"

	"github.com/urfave/cli/v2"
	"gitlab.com/Vitrifi/tag/internal/chalk"
	"gitlab.com/Vitrifi/tag/internal/config"
)

const BundleExtension = ".json"

func BundleCommand(cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name:      "new-bundle",
		Aliases:   []string{"nb"},
		Usage:     fmt.Sprintf("creates a new bunle with the specified %s", chalk.Yellow("bundle-name")),
		Args:      true,
		ArgsUsage: "<bundle-name>",
		Action: func(c *cli.Context) error {
			return bundleAction(c, cfg)
		},
	}
}

func bundleAction(c *cli.Context, cfg *config.Config) error {
	config.CheckConfig(cfg)

	if c.Args().Len() == 0 {
		app.Terminate("please provide the bundle name")
	}
	bundleName := c.Args().Get(0)

	slog.Info(chalk.Green("creating new bundle"), cfg.Env.Path)
	dirPath := path.Join(cfg.Env.Path, c.Path(flags.BundlePathFlag), bundleName, fmt.Sprintf("%s%s", bundleName, BundleExtension))
	if err := os.MkdirAll(filepath.Dir(dirPath), 0o750); err != nil {
		app.Terminate("error creating %s: %s", dirPath, err)
		return nil
	}

	file, err := os.Create(filepath.Clean(dirPath))
	if err != nil {
		app.Terminate("error creating base template ", err)
		return nil
	}
	defer func() {
		if err := file.Close(); err != nil {
			slog.Info("error closing file", err)
		}
	}()

	if _, err := file.Write(getBundleTemplate(bundleName)); err != nil {
		app.Terminate("error creating file %s: %s", file.Name(), err)
		return nil
	}

	return nil
}

func getBundleTemplate(name string) []byte {
	newBundle := engine.Bundle{
		Name: name,
		Generators: []engine.Generators{
			{
				Name: "myGenerator",
			},
		},
	}
	jsonBytes, err := json.Marshal(newBundle)
	if err != nil {
		app.Terminate("cannot create bundle template: %s", err)
	}

	return jsonBytes
}
