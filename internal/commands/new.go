package commands

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/chalk"
	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/fileutil"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/internal/types/flags"
	"github.com/kaikenlabs/tag/pkg/app"
)

func NewCommand(cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name:      "new",
		Usage:     "creates a new generator with the specified " + chalk.Yellow("generator-name"),
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
			&cli.BoolFlag{
				Name:    flags.LibFlag,
				Usage:   "Create generator in the library template referenced by .tagconfig.json",
				Aliases: []string{"l"},
			},
		},
	}
}

func newAction(c *cli.Context, cfg *config.Config) error {
	generator := c.Args().Get(0)
	if generator == "" {
		return app.Errorf("please provide the generator name")
	}

	if err := ValidateNameSafe(generator); err != nil {
		return app.Errorf("invalid generator name: %w", err)
	}

	if err := config.CheckConfig(cfg); err != nil {
		return err
	}

	var basePath string
	if c.Bool(flags.LibFlag) {
		if !cfg.HasTemplateOrigin() {
			return app.Errorf("no library template configured in %s", config.File)
		}
		tagDir, err := resolveLibraryTagDir(cfg.Template.Name)
		if err != nil {
			return err
		}
		basePath = tagDir
		slog.Info(chalk.Green("creating new generator in library template"), "template", cfg.Template.Name)
	} else {
		basePath = cfg.Env.Path
		slog.Info(chalk.Green("creating new generator"), "path", basePath)
	}

	dirPath := filepath.Join(basePath, generator, generator+".go")

	if err := fileutil.ValidatePathContainment(basePath, dirPath); err != nil {
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

	if _, err := fmt.Fprintf(file, newGeneratorTemplate, c.String("package"), c.String("package")); err != nil {
		return app.Errorf("error creating file %s: %w", file.Name(), err)
	}

	return nil
}

// resolveLibraryTagDir resolves the .tag directory inside a library template.
// Creates the .tag directory if it doesn't exist.
func resolveLibraryTagDir(libName string) (string, error) {
	lib, err := newLocalLibrary()
	if err != nil {
		return "", app.Errorf("failed to initialize library: %w", err)
	}

	templatePath, err := lib.TemplatePath(libName)
	if err != nil {
		return "", asAppError(err)
	}

	tagDir := filepath.Join(templatePath, types.TemplatesDir)
	if err := os.MkdirAll(tagDir, 0o750); err != nil {
		return "", app.Errorf("error creating directory %s: %w", tagDir, err)
	}

	return tagDir, nil
}

var newGeneratorTemplate = `---
to: %s/{{ name | snake }}.go
---
package %s

func myFunction() {}
`
