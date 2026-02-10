package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaikenlabs/tag/internal/chalk"
	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/types"

	"github.com/urfave/cli/v2"
)

func generateListCommand(cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name:    "list",
		Aliases: []string{"ls"},
		Usage:   "List available generators and bundles",
		Action: func(c *cli.Context) error {
			return generateList(cfg, os.Stdout)
		},
	}
}

// generatorInfo holds display information about a generator or bundle.
type generatorInfo struct {
	Name        string
	Description string
	Source      string // "template" or "local"
}

func generateList(cfg *config.Config, w io.Writer) error {
	if err := config.CheckConfig(cfg); err != nil {
		return err
	}

	var templateGens, localGens []generatorInfo
	var templateBundles, localBundles []generatorInfo
	var templateName, templateSource, templateVersion string

	// 1. Collect generators from library template
	if cfg.HasTemplateOrigin() {
		templateName = cfg.Template.Name
		templateSource = cfg.Template.Source
		templateVersion = cfg.Template.Version

		lib, err := newLocalLibrary()
		if err == nil {
			templateDir, pathErr := lib.TemplatePath(templateName)
			if pathErr == nil {
				templateGens = scanGenerators(filepath.Join(templateDir, types.TemplatesDir))
				templateBundles = scanBundles(filepath.Join(templateDir, types.TemplatesDir, types.BundlesDir))
			}
		}
	}

	// 2. Collect generators from local .tag.templates/
	if cfg.Env.Path != "" {
		localGens = scanGenerators(cfg.Env.Path)
		localBundles = scanBundles(filepath.Join(cfg.Env.Path, cfg.Env.BundlePath))
	}

	// Check if there's anything to show
	if len(templateGens) == 0 && len(localGens) == 0 && len(templateBundles) == 0 && len(localBundles) == 0 {
		fmt.Fprintln(w, "No generators found.")
		if templateName == "" {
			fmt.Fprintln(w)
			fmt.Fprintln(w, "This project was not scaffolded from a library template.")
			fmt.Fprintln(w, "Create generators in .tag.templates/ or scaffold from a template with generators.")
		}
		return nil
	}

	// Print header
	if templateName != "" {
		version := ""
		if templateVersion != "" {
			version = "@" + templateVersion
		}
		fmt.Fprintf(w, "Generators for this project (template: %s%s)\n\n", templateSource, version)
	} else {
		fmt.Fprintln(w, "Available generators:")
		fmt.Fprintln(w)
	}

	// Print template generators
	if len(templateGens) > 0 {
		fmt.Fprintf(w, "  %s (%s)\n", chalk.Green("TEMPLATE GENERATORS"), templateName)
		for _, g := range templateGens {
			printGeneratorLine(w, g)
		}
		fmt.Fprintln(w)
	}

	// Print local generators
	if len(localGens) > 0 {
		fmt.Fprintf(w, "  %s\n", chalk.Green("PROJECT GENERATORS"))
		for _, g := range localGens {
			printGeneratorLine(w, g)
		}
		fmt.Fprintln(w)
	}

	// Print bundles
	if len(templateBundles) > 0 || len(localBundles) > 0 {
		fmt.Fprintf(w, "  %s\n", chalk.Green("BUNDLES"))
		for _, b := range templateBundles {
			printGeneratorLine(w, b)
		}
		for _, b := range localBundles {
			printGeneratorLine(w, b)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "Run: tag generate <name> <target> [args]")
	return nil
}

func printGeneratorLine(w io.Writer, g generatorInfo) {
	if g.Description != "" {
		fmt.Fprintf(w, "  %-20s %s\n", g.Name, g.Description)
	} else {
		fmt.Fprintf(w, "  %s\n", g.Name)
	}
}

// scanGenerators scans a directory for generator subdirectories.
// Directories starting with _ are skipped (reserved: _shared, _bundles).
func scanGenerators(dir string) []generatorInfo {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var result []generatorInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Skip reserved directories (prefixed with _ or .)
		if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}

		info := generatorInfo{Name: name}

		// Try to read description from tag.template.json
		configPath := filepath.Join(dir, name, types.TemplateConfigFile)
		data, err := os.ReadFile(configPath)
		if err == nil {
			if tc, parseErr := scaffold.ParseTemplateConfig(data); parseErr == nil {
				info.Description = tc.Description
			}
		}

		result = append(result, info)
	}
	return result
}

// scanBundles scans a bundles directory for bundle definitions.
func scanBundles(dir string) []generatorInfo {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var result []generatorInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}

		info := generatorInfo{Name: name}

		// Bundle JSON exists but we use directory name for display consistency

		result = append(result, info)
	}
	return result
}
