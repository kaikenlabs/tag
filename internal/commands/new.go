package commands

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/kaikenlabs/tag/pkg/app"

	"github.com/kaikenlabs/tag/internal/chalk"
	"github.com/kaikenlabs/tag/internal/config"
	"github.com/urfave/cli/v2"
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
	if err := config.CheckConfig(cfg); err != nil {
		return err
	}

	generator := c.Args().Get(0)
	if generator == "" {
		return app.Errorf("please provide the generator name")
	}

	if err := ValidateNameSafe(generator); err != nil {
		return app.Errorf("invalid generator name: %w", err)
	}

	slog.Info(chalk.Green("creating new generator"), "path", cfg.Env.Path)
	dirPath := filepath.Join(cfg.Env.Path, generator, generator+".go")

	if err := ValidatePathContainment(cfg.Env.Path, dirPath); err != nil {
		return app.Errorf("path safety check failed: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dirPath), 0o750); err != nil {
		return app.Errorf("error creating %s: %w", dirPath, err)
	}

	file, err := os.Create(filepath.Clean(dirPath))
	if err != nil {
		return app.Errorf("error creating base template: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			slog.Warn("error closing file", "error", err.Error())
		}
	}()

	if _, err := file.Write([]byte(fmt.Sprintf(newGeneratorTemplate, c.String("package"), c.String("package")))); err != nil {
		return app.Errorf("error creating file %s: %w", file.Name(), err)
	}

	return nil
}

var newGeneratorTemplate = `---
to: %s/{{ name | snake }}.go
---
package %s

func myFunction() {}
`
