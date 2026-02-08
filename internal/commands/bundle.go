package commands

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/kaikenlabs/tag/pkg/app"

	"github.com/kaikenlabs/tag/internal/engine"
	"github.com/kaikenlabs/tag/internal/types/flags"

	"github.com/kaikenlabs/tag/internal/chalk"
	"github.com/kaikenlabs/tag/internal/config"
	"github.com/urfave/cli/v2"
)

const BundleExtension = ".json"

func BundleCommand(cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name:      "new-bundle",
		Aliases:   []string{"nb"},
		Usage:     fmt.Sprintf("creates a new bundle with the specified %s", chalk.Yellow("bundle-name")),
		Args:      true,
		ArgsUsage: "<bundle-name>",
		Action: func(c *cli.Context) error {
			return bundleAction(c, cfg)
		},
	}
}

func bundleAction(c *cli.Context, cfg *config.Config) error {
	if err := config.CheckConfig(cfg); err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return app.Errorf("configuration error: %w", err)
	}

	if c.Args().Len() == 0 {
		return app.Errorf("please provide the bundle name")
	}
	bundleName := c.Args().Get(0)

	slog.Info(chalk.Green("creating new bundle"), "path", cfg.Env.Path)
	dirPath := filepath.Join(cfg.Env.Path, c.Path(flags.BundlePathFlag), bundleName, fmt.Sprintf("%s%s", bundleName, BundleExtension))
	if err := os.MkdirAll(filepath.Dir(dirPath), 0o750); err != nil {
		return app.Errorf("error creating %s: %w", dirPath, err)
	}

	file, err := os.Create(filepath.Clean(dirPath))
	if err != nil {
		return app.Errorf("error creating base template: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			slog.Info("error closing file", "error", err.Error())
		}
	}()

	bundleTemplate, err := getBundleTemplate(bundleName)
	if err != nil {
		return err
	}

	if _, err := file.Write(bundleTemplate); err != nil {
		return app.Errorf("error creating file %s: %w", file.Name(), err)
	}

	return nil
}

func getBundleTemplate(name string) ([]byte, error) {
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
		return nil, app.Errorf("cannot create bundle template: %w", err)
	}

	return jsonBytes, nil
}
