package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaikenlabs/tag/internal/chalk"
	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/engine"
	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/internal/types/flags"

	"github.com/urfave/cli/v2"
)

func generateListCommand(cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name:    "list",
		Aliases: []string{"ls"},
		Usage:   "List available generators and bundles",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  flags.AllFlag,
				Usage: "Show all generators and bundles, including those with unmet requirements",
			},
		},
		Action: func(c *cli.Context) error {
			return generateList(cfg, c.Bool(flags.AllFlag), os.Stdout)
		},
	}
}

// generatorInfo holds display information about a generator or bundle.
type generatorInfo struct {
	Name        string
	Description string
	Source      string // "template" or "local"
	Requires    []string
}

// generatorLists holds all collected generator/bundle data for a project.
type generatorLists struct {
	templateName    string
	templateSource  string
	templateVersion string
	templateGens    []generatorInfo
	templateBundles []generatorInfo
	localGens       []generatorInfo
	localBundles    []generatorInfo
}

// collectGeneratorLists gathers generators and bundles from the library template and
// local .tag/ directory. Library errors are soft-failures (logged at debug level).
func collectGeneratorLists(cfg *config.Config) generatorLists {
	var lists generatorLists
	if cfg.HasTemplateOrigin() {
		lists.templateName = cfg.Template.Name
		lists.templateSource = cfg.Template.Source
		lists.templateVersion = cfg.Template.Version
		if lib, err := newLocalLibrary(); err != nil {
			slog.Debug("library unavailable for listing", "error", err)
		} else if templateDir, pathErr := lib.TemplatePath(lists.templateName); pathErr == nil {
			lists.templateGens = scanGenerators(filepath.Join(templateDir, types.TemplatesDir))
			lists.templateBundles = scanBundles(filepath.Join(templateDir, types.TemplatesDir, types.BundlesDir))
		}
	}
	if cfg.Env.Path != "" {
		lists.localGens = scanGenerators(cfg.Env.Path)
		lists.localBundles = scanBundles(filepath.Join(cfg.Env.Path, cfg.Env.BundlePath))
	}
	return lists
}

func generateList(cfg *config.Config, showAll bool, w io.Writer) error {
	if err := config.CheckConfig(cfg); err != nil {
		return err
	}

	lists := collectGeneratorLists(cfg)

	// Filter out generators/bundles with unmet requirements unless --all is set.
	vars := make(map[string]any)
	if cfg.Variables != nil {
		vars = cfg.Variables
	}
	if !showAll {
		lists.templateGens = filterByRequirements(lists.templateGens, vars)
		lists.localGens = filterByRequirements(lists.localGens, vars)
		lists.templateBundles = filterByRequirements(lists.templateBundles, vars)
		lists.localBundles = filterByRequirements(lists.localBundles, vars)
	}

	// Check if there's anything to show
	if len(lists.templateGens) == 0 && len(lists.localGens) == 0 && len(lists.templateBundles) == 0 && len(lists.localBundles) == 0 {
		fmt.Fprintln(w, "No generators found.")
		if lists.templateName == "" {
			fmt.Fprintln(w)
			fmt.Fprintln(w, "This project was not scaffolded from a library template.")
			fmt.Fprintln(w, "Create generators in .tag/ or scaffold from a template with generators.")
		}
		return nil
	}

	// Print header
	if lists.templateName != "" {
		version := ""
		if lists.templateVersion != "" {
			version = "@" + lists.templateVersion
		}
		fmt.Fprintf(w, "Generators for this project (template: %s%s)\n\n", lists.templateSource, version)
	} else {
		fmt.Fprintln(w, "Available generators:")
		fmt.Fprintln(w)
	}

	// Print template generators
	if len(lists.templateGens) > 0 {
		fmt.Fprintf(w, "  %s (%s)\n", chalk.Green("TEMPLATE GENERATORS"), lists.templateName)
		for _, g := range lists.templateGens {
			printGeneratorLine(w, g)
		}
		fmt.Fprintln(w)
	}

	// Print local generators
	if len(lists.localGens) > 0 {
		fmt.Fprintf(w, "  %s\n", chalk.Green("PROJECT GENERATORS"))
		for _, g := range lists.localGens {
			printGeneratorLine(w, g)
		}
		fmt.Fprintln(w)
	}

	// Print bundles
	if len(lists.templateBundles) > 0 || len(lists.localBundles) > 0 {
		fmt.Fprintf(w, "  %s\n", chalk.Green("BUNDLES"))
		for _, b := range lists.templateBundles {
			printGeneratorLine(w, b)
		}
		for _, b := range lists.localBundles {
			printGeneratorLine(w, b)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "Run: tag generate <name> <target> [args]")
	return nil
}

func printGeneratorLine(w io.Writer, g generatorInfo) {
	reqSuffix := ""
	if len(g.Requires) > 0 {
		reqSuffix = " [requires: " + strings.Join(g.Requires, ", ") + "]"
	}
	if g.Description != "" {
		fmt.Fprintf(w, "  %-20s %s%s\n", g.Name, g.Description, reqSuffix)
	} else {
		fmt.Fprintf(w, "  %s%s\n", g.Name, reqSuffix)
	}
}

// scanDirEntries scans a directory for subdirectories, returning generatorInfo for each.
// Directories starting with _ or . are skipped (reserved: _shared, _bundles).
// When readDescription is true, it reads tag.template.json for a description field.
func scanDirEntries(dir string) []generatorInfo {
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
		if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") || name == types.HistoryDir {
			continue
		}

		info := generatorInfo{Name: name}

		// Try tag.template.json first.
		configPath := filepath.Join(dir, name, types.TemplateConfigFile)
		data, readErr := os.ReadFile(configPath)
		if readErr == nil {
			if tc, parseErr := scaffold.ParseTemplateConfig(data); parseErr == nil {
				info.Description = tc.Description
				info.Requires = tc.Requires
			}
		}

		// Fall back to frontmatter "desc" field from the first template file.
		if info.Description == "" {
			info.Description = readFrontmatterDesc(filepath.Join(dir, name))
		}

		result = append(result, info)
	}
	return result
}

// scanGenerators scans a directory for generator subdirectories with descriptions.
func scanGenerators(dir string) []generatorInfo {
	return scanDirEntries(dir)
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

		// Read description and requires from bundle manifest (<name>/<name>.json).
		manifestPath := filepath.Join(dir, name, name+types.BundleExtension)
		data, readErr := os.ReadFile(manifestPath)
		if readErr == nil {
			var bundle engine.Bundle
			if jsonErr := json.Unmarshal(data, &bundle); jsonErr == nil {
				info.Description = bundle.Description
				info.Requires = bundle.Requires
			}
		}

		result = append(result, info)
	}
	return result
}

// readFrontmatterDesc reads the first template file in a generator directory
// and returns the "desc" field from its frontmatter, if present.
func readFrontmatterDesc(genDir string) string {
	entries, err := os.ReadDir(genDir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		data, readErr := os.ReadFile(filepath.Join(genDir, entry.Name()))
		if readErr != nil {
			continue
		}

		metaRaw, _, extractErr := template.ExtractMetadata(string(data))
		if extractErr != nil || metaRaw == "" {
			continue
		}

		meta, parseErr := template.ParseMetadata(metaRaw)
		if parseErr != nil {
			continue
		}

		if meta.Description != "" {
			return meta.Description
		}
	}

	return ""
}

// filterByRequirements returns only generators/bundles whose requirements
// are all met by the given variables. Items with no requirements pass through.
func filterByRequirements(items []generatorInfo, vars map[string]any) []generatorInfo {
	var filtered []generatorInfo
	for _, item := range items {
		if requirementsMet(item.Requires, vars) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
