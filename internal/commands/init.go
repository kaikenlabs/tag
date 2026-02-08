package commands

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/kaikenlabs/tag/internal/types/flags"
	"github.com/kaikenlabs/tag/pkg/app"

	"github.com/kaikenlabs/tag/internal/chalk"
	"github.com/kaikenlabs/tag/internal/config"
	"github.com/urfave/cli/v2"
)

func InitCommand() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: fmt.Sprintf("initialises %s's configuration and directory", chalk.Yellow("tag")),
		Action: func(c *cli.Context) error {
			return initAction(c)
		},
	}
}

func initAction(c *cli.Context) error {
	slog.Info(chalk.Green("creating initial setup"), "path", c.String(flags.PathFlag))
	dirPath := filepath.Join(".", c.String(flags.PathFlag), c.String(flags.SharedPathFlag), ".gitkeep")
	if err := os.MkdirAll(filepath.Dir(dirPath), 0o750); err != nil {
		slog.Error("error initialising tag's shared path", "error", err.Error())
		return err
	}

	dirPath = filepath.Join(".", c.String(flags.PathFlag), c.String(flags.BundlePathFlag), ".gitkeep")
	if err := os.MkdirAll(filepath.Dir(dirPath), 0o750); err != nil {
		slog.Error("error initialising tag's bundle path", "error", err.Error())
		return err
	}

	if err := config.CreateConfigFile(c); err != nil {
		return app.Errorf("cannot create the config file at %s: %w", dirPath, err)
	}

	return nil
}
