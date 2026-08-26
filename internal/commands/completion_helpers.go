package commands

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/config"
)

// completeGeneratorNames prints available generator and bundle names for shell completion.
func completeGeneratorNames(cfg *config.Config, w io.Writer) {
	if cfg == nil {
		return
	}

	seen := make(map[string]bool)
	printUnique := func(infos []GeneratorInfo) {
		for _, g := range infos {
			if !seen[g.Name] {
				fmt.Fprintln(w, g.Name)
				seen[g.Name] = true
			}
		}
	}

	// Collect from library template
	if dir, ok := libraryTemplateDir(cfg); ok {
		gens, bundles := libraryGeneratorsAndBundles(dir)
		printUnique(gens)
		printUnique(bundles)
	}

	// Collect from local .tag/
	if cfg.Env.Path != "" {
		printUnique(scanGenerators(cfg.Env.Path))
		printUnique(scanBundles(filepath.Join(cfg.Env.Path, cfg.Env.BundlePath)))
	}
}

// libraryTemplateDir returns the library template directory path for the project's
// configured template, if available. Returns ("", false) on any error.
func libraryTemplateDir(cfg *config.Config) (string, bool) {
	if !cfg.HasTemplateOrigin() {
		return "", false
	}
	lib, err := newLocalLibrary()
	if err != nil {
		return "", false
	}
	dir, err := lib.TemplatePath(cfg.Template.Name)
	if err != nil {
		return "", false
	}
	return dir, true
}

// completeLibraryTemplateNames prints installed library template names for shell completion.
func completeLibraryTemplateNames(c *cli.Context) {
	// Only complete the first argument
	if c.NArg() > 0 {
		return
	}

	lib, err := newLocalLibrary()
	if err != nil {
		return
	}

	entries, err := lib.List()
	if err != nil {
		return
	}

	for _, entry := range entries {
		fmt.Println(entry.Name)
	}
}
