package commands

import (
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"

	"gitlab.com/Vitrifi/tag/internal/types/flags"
	"gitlab.com/Vitrifi/tag/pkg/app"

	"github.com/urfave/cli/v2"
	"gitlab.com/Vitrifi/tag/internal/chalk"
	"gitlab.com/Vitrifi/tag/internal/config"
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
	slog.Info(chalk.Green("creating initial setup"), c.String(flags.PathFlag))
	dirPath := path.Join(".", c.String(flags.PathFlag), c.String(flags.SharedPathFlag), ".gitkeep")
	if err := os.MkdirAll(filepath.Dir(dirPath), 0o750); err != nil {
		slog.Info("error initialising tag's shared path", err.Error())
		return err
	}

	dirPath = path.Join(".", c.String(flags.PathFlag), c.String(flags.BundlePathFlag), ".gitkeep")
	if err := os.MkdirAll(filepath.Dir(dirPath), 0o750); err != nil {
		slog.Info("error initialising tag's bundle path", err.Error())
		return err
	}

	if err := config.CreateConfigFile(c); err != nil {
		app.Terminate("cannot create the config file at %s: %s", dirPath, err.Error())
	}

	return nil
}
