package commands

import (
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"

	"gitlab.com/Vitrifi/tag/pkg/app"

	"github.com/urfave/cli/v2"
	"gitlab.com/Vitrifi/tag/internal/chalk"
	"gitlab.com/Vitrifi/tag/internal/config"
)

func NewCommand(cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name:      "new",
		Usage:     fmt.Sprintf("creates a new generator with the specified %s", chalk.Yellow("generator-name")),
		Args:      true,
		ArgsUsage: "<generator-name>",
		Action: func(c *cli.Context) error {
			return newAction(c, cfg)
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "package",
				Value:   "mypackage",
				Usage:   "Specifies the package for the generator",
				Aliases: []string{"k"},
			},
		},
	}
}

func newAction(c *cli.Context, cfg *config.Config) error {
	config.CheckConfig(cfg)

	generator := c.Args().Get(0)
	if generator == "" {
		app.Terminate("please provide the generator name")
	}

	slog.Info(chalk.Green("creating new generator"), cfg.Env.Path)
	dirPath := path.Join(cfg.Env.Path, generator, fmt.Sprintf("%s%s", generator, cfg.Env.Extension))
	if err := os.MkdirAll(filepath.Dir(dirPath), 0o750); err != nil {
		app.Terminate("error creating %s: %s", dirPath, err)
		return nil
	}

	file, err := os.Create(filepath.Clean(dirPath))
	if err != nil {
		app.Terminate("error creating base template: %s", err)
		return nil
	}
	defer func() {
		if err := file.Close(); err != nil {
			slog.Info("error closing file", err)
		}
	}()

	if _, err := file.Write([]byte(fmt.Sprintf(newGeneratorTemplate, c.String("package"), c.String("package")))); err != nil {
		app.Terminate("error creating file %s: %s", file.Name(), err)
		return nil
	}

	return nil
}

var newGeneratorTemplate = `---
to: %s/{{ .Name | caseSnake }}.go
---
package %s

func myFunction() {}
`
