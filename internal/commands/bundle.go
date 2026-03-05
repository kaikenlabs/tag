package commands

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/chalk"
	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/engine"
	"github.com/kaikenlabs/tag/internal/fileutil"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/internal/types/flags"
	"github.com/kaikenlabs/tag/internal/validate"
	"github.com/kaikenlabs/tag/pkg/app"
)

func templateNewBundleCommand(cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name:      "bundle",
		Usage:     "creates a new bundle with the specified " + chalk.Yellow("bundle-name"),
		Args:      true,
		ArgsUsage: "<bundle-name>",
		Description: `Create a new bundle definition file in the .tag directory.

A bundle groups multiple generators to run together in a single command.

ARGUMENTS:
  <bundle-name>       Name of the bundle to create

FLAGS:
  --lib, -l           Create in the library template referenced by .tagconfig.json
  --self-contained, -s  Create a self-contained bundle (generators resolved from bundle directory)

EXAMPLES:
  # Create a bundle in the local .tag directory
  tag template new bundle my-feature

  # Create in a library template
  tag template new bundle my-feature --lib

  # Create a self-contained bundle
  tag template new bundle my-feature --self-contained`,
		Action: func(c *cli.Context) error {
			return bundleAction(c, cfg)
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    flags.LibFlag,
				Usage:   "Create bundle in the library template referenced by .tagconfig.json",
				Aliases: []string{"l"},
			},
			&cli.BoolFlag{
				Name:    flags.SelfContainedFlag,
				Usage:   "Create a self-contained bundle (generators resolved from bundle directory)",
				Aliases: []string{"s"},
			},
		},
	}
}

func bundleAction(c *cli.Context, cfg *config.Config) error {
	if c.Args().Len() == 0 {
		return app.UsageErrorf("please provide the bundle name")
	}
	bundleName := c.Args().Get(0)

	if err := ValidateNameSafe(bundleName); err != nil {
		return app.Errorf("invalid bundle name: %w", err)
	}

	if err := validate.GeneratorName(bundleName); err != nil {
		return app.Errorf("invalid bundle name: %w", err)
	}

	if err := config.CheckConfig(cfg); err != nil {
		return err
	}

	basePath, err := resolveBasePath(c, cfg)
	if err != nil {
		return err
	}
	var bundleSubPath string
	if c.Bool(flags.LibFlag) {
		bundleSubPath = types.BundlesDir
		slog.Info(chalk.Green("creating new bundle in library template"), "template", cfg.Template.Name)
	} else {
		if err = cfg.Validate(); err != nil {
			return app.Errorf("configuration error: %w", err)
		}
		bundleSubPath = c.Path(flags.BundlePathFlag)
		slog.Info(chalk.Green("creating new bundle"), "path", basePath)
	}

	dirPath := filepath.Join(basePath, bundleSubPath, bundleName, fmt.Sprintf("%s%s", bundleName, types.BundleExtension))

	if err = fileutil.ValidatePathContainment(basePath, dirPath); err != nil {
		return app.Errorf("path safety check failed: %w", err)
	}
	if err = os.MkdirAll(filepath.Dir(dirPath), types.DirModeRestricted); err != nil {
		return app.Errorf("error creating %s: %w", dirPath, err)
	}

	file, err := os.Create(filepath.Clean(dirPath))
	if err != nil {
		return app.Errorf("error creating base template: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			slog.Warn("error closing file", "error", closeErr.Error()) //nolint:gosec // G706: slog structured logging; log injection not a concern in a CLI tool
		}
	}()

	bundleTemplate, err := getBundleTemplate(bundleName, c.Bool(flags.SelfContainedFlag))
	if err != nil {
		return err
	}

	if _, err := file.Write(bundleTemplate); err != nil {
		return app.Errorf("error creating file %s: %w", file.Name(), err)
	}

	return nil
}

func getBundleTemplate(name string, selfContained bool) ([]byte, error) {
	newBundle := engine.Bundle{
		Name:          name,
		SelfContained: selfContained,
		Generators: []engine.GeneratorRef{
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
