package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kaikenlabs/tag/internal/chalk"
	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/engine"
	"github.com/kaikenlabs/tag/internal/jsonout"
	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/internal/types/flags"

	"github.com/urfave/cli/v2"
)

// generateListFlags returns the flags for `generate list` / `template list`.
// Both commands are built from this SAME slice so their flag sets, and their
// output, can never drift apart.
func generateListFlags() []cli.Flag {
	return []cli.Flag{
		&cli.BoolFlag{
			Name:  flags.AllFlag,
			Usage: "Show all generators and bundles, including those with unmet requirements",
		},
		formatFlag(formatText, formatJSON),
	}
}

func generateListCommand(cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name:    "list",
		Aliases: []string{"ls"},
		Usage:   "List available generators and bundles",
		Flags:   generateListFlags(),
		Action: func(c *cli.Context) error {
			format, err := resolveFormat(c, formatText, formatJSON)
			if err != nil {
				return err
			}
			return generateList(cfg, c.Bool(flags.AllFlag), cmdOut(c), format)
		},
	}
}

// GeneratorInfo holds display information about a generator or bundle, and
// doubles as the `--format json` entry shape for `generate list` /
// `template list`. The same struct backs both generator and bundle rows;
// Generators (member names) applies to bundle rows and is omitted for
// generator rows.
//
// There is deliberately no per-generator "bundle" field. The data model stores
// bundle -> members, never the reverse, so a single owning bundle cannot be
// substantiated: a generator may be declared by several bundles, by none, and
// template-scoped and local generators can share a name. Reporting one guessed
// winner would mislead a script in a way the text output never does — the same
// reasoning that keeps a "source" field out of `dialect list`. Membership is
// exact and derivable from bundles[].generators.
type GeneratorInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Source      string   `json:"-"` // "template" or "local"; not yet populated by any scan, kept out of the wire shape
	Requires    []string `json:"-"` // internal eligibility data; RequirementsMet is what a JSON consumer sees

	// Generators lists the member generator names of a bundle row. Always
	// omitted for generator rows themselves.
	Generators []string `json:"generators,omitempty"`
	// RequirementsMet mirrors the text output's "[requires: x]" suffix: it is
	// only ever false for an entry when --all surfaced it despite unmet
	// requirements, since a non---all listing filters unmet entries out
	// entirely before either format is written.
	RequirementsMet bool `json:"requirements_met"`
}

// generatorLists holds all collected generator/bundle data for a project.
type generatorLists struct {
	templateName    string
	templateSource  string
	templateVersion string
	templateGens    []GeneratorInfo
	templateBundles []GeneratorInfo
	localGens       []GeneratorInfo
	localBundles    []GeneratorInfo
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
			lists.templateGens, lists.templateBundles = libraryGeneratorsAndBundles(templateDir)
		}
	}
	if cfg.Env.Path != "" {
		lists.localGens = scanGenerators(cfg.Env.Path)
		lists.localBundles = scanBundles(filepath.Join(cfg.Env.Path, cfg.Env.BundlePath))
	}
	return lists
}

func libraryGeneratorsAndBundles(templateDir string) (gens, bundles []GeneratorInfo) {
	for _, root := range libraryGeneratorRoots(templateDir) {
		gens = appendNewByName(gens, scanGenerators(root))
		bundles = appendNewByName(bundles, scanBundles(filepath.Join(root, types.BundlesDir)))
	}
	return gens, bundles
}

func appendNewByName(dst, src []GeneratorInfo) []GeneratorInfo {
	for _, item := range src {
		if !slices.ContainsFunc(dst, func(d GeneratorInfo) bool { return d.Name == item.Name }) {
			dst = append(dst, item)
		}
	}
	return dst
}

func generateList(cfg *config.Config, showAll bool, w io.Writer, format string) error {
	if err := config.CheckConfig(cfg); err != nil {
		return err
	}

	lists := collectGeneratorLists(cfg)

	// Filter out generators/bundles with unmet requirements unless --all is set.
	// This runs once, before either format is written, so JSON and text agree
	// on which entries are hidden entirely vs. shown with unmet requirements.
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

	if format == formatJSON {
		return jsonout.Write(w, buildGeneratorListJSON(lists, vars))
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

func printGeneratorLine(w io.Writer, g GeneratorInfo) {
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

// scanDirEntries scans a directory for subdirectories, returning GeneratorInfo for each.
// Directories starting with _ or . are skipped (reserved: _shared, _bundles).
// When readDescription is true, it reads tag.template.json for a description field.
func scanDirEntries(dir string) []GeneratorInfo {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var result []GeneratorInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") || name == types.HistoryDir {
			continue
		}
		genDir := filepath.Join(dir, name)
		if !engine.HasTemplateFiles(genDir) {
			continue
		}

		info := GeneratorInfo{Name: name}

		// Try tag.template.json first.
		configPath := filepath.Join(genDir, types.TemplateConfigFile)
		data, readErr := os.ReadFile(configPath)
		if readErr == nil {
			if tc, parseErr := scaffold.ParseTemplateConfig(data); parseErr == nil {
				info.Description = tc.Description
				info.Requires = tc.Requires
			}
		}

		// Fall back to frontmatter "desc" field from the first template file.
		if info.Description == "" {
			info.Description = readFrontmatterDesc(genDir)
		}

		result = append(result, info)
	}
	return result
}

// scanGenerators scans a directory for generator subdirectories with descriptions.
func scanGenerators(dir string) []GeneratorInfo {
	return scanDirEntries(dir)
}

// scanBundles scans a bundles directory for bundle definitions.
func scanBundles(dir string) []GeneratorInfo {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var result []GeneratorInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}

		info := GeneratorInfo{Name: name}

		// Read description and requires from bundle manifest (<name>/<name>.json).
		manifestPath := filepath.Join(dir, name, name+types.BundleExtension)
		data, readErr := os.ReadFile(manifestPath)
		if readErr == nil {
			var bundle engine.Bundle
			if jsonErr := json.Unmarshal(data, &bundle); jsonErr == nil {
				info.Description = bundle.Description
				info.Requires = bundle.Requires
				info.Generators = bundleGeneratorNames(bundle.Generators)
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
func filterByRequirements(items []GeneratorInfo, vars map[string]any) []GeneratorInfo {
	var filtered []GeneratorInfo
	for _, item := range items {
		if requirementsMet(item.Requires, vars) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// bundleGeneratorNames extracts member generator names from a bundle
// manifest's Generators field.
func bundleGeneratorNames(refs []engine.GeneratorRef) []string {
	if len(refs) == 0 {
		return nil
	}
	names := make([]string, len(refs))
	for i, ref := range refs {
		names[i] = ref.Name
	}
	return names
}

// generatorListJSON is the top-level `--format json` shape shared by
// `generate list` and `template list`.
type generatorListJSON struct {
	Generators []GeneratorInfo `json:"generators"`
	Bundles    []GeneratorInfo `json:"bundles"`
}

// buildGeneratorListJSON flattens the already-filtered lists into the
// "generators" and "bundles" arrays, setting RequirementsMet from vars and
// annotating each generator with the bundle that lists it as a member, where
// one of the listed bundles does. It does not re-run or duplicate the
// filtering `generateList` already applied: entries hidden by unmet
// requirements under a non---all listing are absent from lists already, so
// they are absent from the JSON in exactly the same cases they are absent
// from the text output.
func buildGeneratorListJSON(lists generatorLists, vars map[string]any) generatorListJSON {
	bundles := make([]GeneratorInfo, 0, len(lists.templateBundles)+len(lists.localBundles))
	bundles = append(bundles, lists.templateBundles...)
	bundles = append(bundles, lists.localBundles...)

	for i := range bundles {
		bundles[i].RequirementsMet = requirementsMet(bundles[i].Requires, vars)
	}

	generators := make([]GeneratorInfo, 0, len(lists.templateGens)+len(lists.localGens))
	generators = append(generators, lists.templateGens...)
	generators = append(generators, lists.localGens...)
	for i := range generators {
		generators[i].RequirementsMet = requirementsMet(generators[i].Requires, vars)
	}

	return generatorListJSON{Generators: generators, Bundles: bundles}
}
