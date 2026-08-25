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
	"github.com/kaikenlabs/tag/internal/validate"
	"github.com/kaikenlabs/tag/pkg/app"
)

func templateNewGeneratorFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "package",
			Value:   "mypackage",
			Usage:   "Specifies the package for the generator",
			Aliases: []string{"p"},
		},
		&cli.BoolFlag{
			Name:    flags.LibFlag,
			Usage:   "Create generator in the library template referenced by .tagconfig.json",
			Aliases: []string{"l"},
		},
		&cli.StringFlag{
			Name:    flags.InBundleFlag,
			Usage:   "Create generator inside a self-contained bundle directory",
			Aliases: []string{"B"},
		},
	}
}

func templateNewGeneratorCommand(cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name:      "generator",
		Aliases:   []string{"gen"},
		Usage:     "creates a new generator with the specified " + chalk.Yellow("generator-name"),
		Args:      true,
		ArgsUsage: "<generator-name>",
		Description: `Create a new generator template file in the .tag directory.

ARGUMENTS:
  <generator-name>    Name of the generator to create

FLAGS:
  --package, -p       Package name for the generator (default: "mypackage")
  --lib, -l           Create in the library template referenced by .tagconfig.json
  --in-bundle, -B     Create inside a self-contained bundle directory

EXAMPLES:
  # Create a generator in the local .tag directory
  tag template new generator my-model

  # Create with a custom package name
  tag template new generator my-model --package models

  # Create in a library template
  tag template new generator my-model --lib

  # Create inside a specific bundle
  tag template new generator my-model --in-bundle my-bundle`,
		Action: func(c *cli.Context) error {
			return newAction(c, cfg)
		},
		Flags: templateNewGeneratorFlags(),
	}
}

func newAction(c *cli.Context, cfg *config.Config) error {
	args, err := reparseTrailingFlags(c, templateNewGeneratorFlags())
	if err != nil {
		return app.UsageErrorf("%s", err)
	}
	if len(args) < 1 {
		return app.UsageErrorf("please provide the generator name")
	}
	generator := args[0]

	if err = ValidateNameSafe(generator); err != nil {
		return app.Errorf("invalid generator name: %w", err)
	}

	if err = validate.GeneratorName(generator); err != nil {
		return app.Errorf("invalid generator name: %w", err)
	}

	err = config.CheckConfig(cfg)
	if err != nil {
		return err
	}

	basePath, err := resolveBasePath(c, cfg)
	if err != nil {
		return err
	}
	if c.Bool(flags.LibFlag) {
		slog.Info(chalk.Green("creating new generator in library template"), "template", cfg.Template.Name)
	} else {
		slog.Info(chalk.Green("creating new generator"), "path", basePath)
	}

	// When --bundle is set, create generator inside the bundle directory
	bundleName := c.String(flags.InBundleFlag)
	if bundleName != "" {
		if err = ValidateNameSafe(bundleName); err != nil {
			return app.Errorf("invalid bundle name: %w", err)
		}
		bundleSubPath := c.Path(flags.BundlePathFlag)
		if bundleSubPath == "" {
			bundleSubPath = types.BundlesDir
		}
		bundleDir := filepath.Join(basePath, bundleSubPath, bundleName)
		if err = fileutil.ValidatePathContainment(basePath, bundleDir); err != nil {
			return app.Errorf("path safety check failed: %w", err)
		}
		if _, statErr := os.Stat(bundleDir); statErr != nil {
			return app.Errorf("bundle directory %q does not exist; create it first with 'tag template new bundle %s'",
				bundleDir, bundleName)
		}
		basePath = bundleDir
		slog.Info(chalk.Green("creating generator in bundle"), "bundle", bundleName)
	}

	dirPath := filepath.Join(basePath, generator, generator+".go")

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

	if _, err = fmt.Fprintf(file, newGeneratorTemplate, c.String("package"), c.String("package")); err != nil {
		return app.Errorf("error creating file %s: %w", file.Name(), err)
	}

	return nil
}

// resolveLibraryTagDir resolves the .tag directory inside a library template.
// Creates the .tag directory if it doesn't exist.
// resolveBasePath returns the base .tag directory for new template/bundle creation.
// With --lib, it resolves the library template's .tag directory;
// otherwise it returns cfg.Env.Path.
func resolveBasePath(c *cli.Context, cfg *config.Config) (string, error) {
	if c.Bool(flags.LibFlag) {
		if !cfg.HasTemplateOrigin() {
			return "", app.Errorf("no library template configured in %s", config.File)
		}
		return resolveLibraryTagDir(cfg.Template.Name)
	}
	return cfg.Env.Path, nil
}

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
	if err := os.MkdirAll(tagDir, types.DirModeRestricted); err != nil {
		return "", app.Errorf("error creating directory %s: %w", tagDir, err)
	}

	return tagDir, nil
}

var newGeneratorTemplate = `---
to: %s/{{ name | snake }}.go
# inject: true
# before: "// marker"
# after: "// marker"
# append: true
# notes: "generator notes"
---
package %s

func myFunction() {}
`
