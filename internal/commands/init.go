package commands

import (
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
)

func templateInitCommand() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: fmt.Sprintf("initialises %s's configuration and directory", chalk.Yellow("tag")),
		Description: `Initialize the TAG directory structure in the current project.

Creates the .tag/ directory with _shared/ and _bundles/ subdirectories,
and a .tagconfig.json configuration file.

EXAMPLES:
  # Initialize with default structure
  tag template init

  # Initialize with custom paths
  tag template init --path .tag --shared-path _shared --bundle-path _bundles`,
		Action: initAction,
	}
}

func initAction(c *cli.Context) error {
	slog.Info(chalk.Green("creating initial setup"), "path", c.String(flags.PathFlag))
	dirPath := filepath.Join(".", c.String(flags.PathFlag), c.String(flags.SharedPathFlag), ".gitkeep")
	if err := os.MkdirAll(filepath.Dir(dirPath), types.DirModeRestricted); err != nil {
		return app.Errorf("cannot initialise tag's shared path: %w", err)
	}

	dirPath = filepath.Join(".", c.String(flags.PathFlag), c.String(flags.BundlePathFlag), ".gitkeep")
	if err := os.MkdirAll(filepath.Dir(dirPath), types.DirModeRestricted); err != nil {
		return app.Errorf("cannot initialise tag's bundle path: %w", err)
	}

	if err := config.CreateConfigFile(config.CreateConfigOptions{
		TagPath:    c.String(flags.PathFlag),
		SharedPath: c.String(flags.SharedPathFlag),
		BundlePath: c.String(flags.BundlePathFlag),
	}); err != nil {
		return app.Errorf("cannot create the config file at %s: %w", dirPath, err)
	}

	return nil
}
