package graph

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kaikenlabs/tag/internal/engine"
	"github.com/kaikenlabs/tag/internal/fileutil"
	"github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/types"
)

// Action type constants.
const (
	actionCreate = "create"
	actionInject = "inject"
	actionAppend = "append"
)

// Analyze scans a template directory and returns a GraphReport describing
// the dependency graph between generators, files, and injection markers.
func Analyze(root string) (*GraphReport, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("path does not exist: %s", root)
		}
		return nil, fmt.Errorf("stat path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", root)
	}

	report := &GraphReport{}

	// Scan generators from .tag/ directory.
	tagDir := filepath.Join(absRoot, types.TemplatesDir)
	generators, warnings := scanGenerators(tagDir)
	report.Generators = generators
	report.Warnings = append(report.Warnings, warnings...)

	// Scan bundles from .tag/_bundles/ directory.
	bundlesDir := filepath.Join(tagDir, types.BundlesDir)
	report.Bundles = scanBundles(bundlesDir, report.Generators)

	// Scan scaffold source files for injection markers.
	allMarkers := collectAllMarkers(report.Generators)
	report.Markers = scanMarkers(absRoot, allMarkers)

	// Generate cross-reference warnings.
	report.Warnings = append(report.Warnings, generateWarnings(report)...)

	return report, nil
}

// scanGenerators reads .tag/ for generator subdirectories and extracts
// metadata from each generator's template files.
func scanGenerators(tagDir string) ([]GeneratorNode, []Warning) {
	entries, err := os.ReadDir(tagDir)
	if err != nil {
		return nil, nil
	}

	var generators []GeneratorNode
	var warnings []Warning

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Skip reserved directories.
		if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}

		genDir := filepath.Join(tagDir, name)
		actions, genWarnings := extractGeneratorActions(name, genDir)
		generators = append(generators, GeneratorNode{
			Name:    name,
			Actions: actions,
		})
		warnings = append(warnings, genWarnings...)
	}

	// Sort generators by name for deterministic output.
	slices.SortFunc(generators, func(a, b GeneratorNode) int {
		return strings.Compare(a.Name, b.Name)
	})

	return generators, warnings
}

// extractGeneratorActions loads template files from a generator directory
// and extracts metadata to determine what actions each template performs.
func extractGeneratorActions(genName, genDir string) ([]ActionInfo, []Warning) {
	templates, err := engine.LoadTemplateFiles(genDir)
	if err != nil {
		return nil, nil
	}

	var actions []ActionInfo
	var warnings []Warning

	for filePath, content := range templates {
		// Skip tag.template.json — not a template file.
		if filepath.Base(filePath) == types.TemplateConfigFile {
			continue
		}

		action, warn := parseTemplateAction(genName, filePath, content)
		if warn != nil {
			warnings = append(warnings, *warn)
			continue
		}
		if action != nil {
			actions = append(actions, *action)
		}
	}

	// Sort actions by target for deterministic output.
	slices.SortFunc(actions, func(a, b ActionInfo) int {
		return strings.Compare(a.Target, b.Target)
	})

	return actions, warnings
}

// parseTemplateAction extracts a single action from a template file's metadata.
// Returns nil action if the file has no metadata block.
func parseTemplateAction(genName, filePath, content string) (*ActionInfo, *Warning) {
	metaRaw, _, extractErr := template.ExtractMetadata(content)
	if extractErr != nil {
		if errors.Is(extractErr, template.ErrNoMetadataBlock) {
			return nil, nil
		}
		w := Warning{
			Code:      "malformed_metadata",
			Generator: genName,
			Message:   fmt.Sprintf("cannot extract metadata from %s: %v", filepath.Base(filePath), extractErr),
		}
		return nil, &w
	}

	meta, parseErr := template.ParseMetadata(metaRaw)
	if parseErr != nil {
		w := Warning{
			Code:      "malformed_metadata",
			Generator: genName,
			Message:   fmt.Sprintf("cannot parse metadata in %s: %v", filepath.Base(filePath), parseErr),
		}
		return nil, &w
	}

	action := ActionInfo{
		Target: meta.To,
	}

	switch meta.Action {
	case template.ActionInject:
		action.Type = actionInject
		action.Marker = meta.InjectMatcher
		action.Position = strings.ToLower(string(meta.InjectClause))
	case template.ActionAppend:
		action.Type = actionAppend
	default:
		action.Type = actionCreate
	}

	return &action, nil
}

// scanBundles reads .tag/_bundles/ for bundle definitions and validates
// their generator execution order.
func scanBundles(bundlesDir string, generators []GeneratorNode) []BundleInfo {
	entries, err := os.ReadDir(bundlesDir)
	if err != nil {
		return nil
	}

	// Build a lookup of generator actions by name.
	genActions := make(map[string][]ActionInfo)
	for _, gen := range generators {
		genActions[gen.Name] = gen.Actions
	}

	var bundles []BundleInfo

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		bundleFile := filepath.Join(bundlesDir, name, name+types.BundleExtension)
		data, readErr := os.ReadFile(bundleFile)
		if readErr != nil {
			continue
		}

		var bundle engine.Bundle
		if jsonErr := json.Unmarshal(data, &bundle); jsonErr != nil {
			continue
		}

		order := make([]string, 0, len(bundle.Generators))
		for _, gen := range bundle.Generators {
			order = append(order, gen.Name)
		}

		bundles = append(bundles, BundleInfo{
			Name:       name,
			Order:      order,
			ValidOrder: validateBundleOrder(order, genActions),
		})
	}

	// Sort bundles by name for deterministic output.
	slices.SortFunc(bundles, func(a, b BundleInfo) int {
		return strings.Compare(a.Name, b.Name)
	})

	return bundles
}

// validateBundleOrder checks if generators that create files appear before
// generators that inject into those files.
func validateBundleOrder(order []string, genActions map[string][]ActionInfo) bool {
	// Build index of which generator creates each target.
	createdBy := make(map[string]int) // target -> index of creating generator
	for i, genName := range order {
		for _, action := range genActions[genName] {
			if action.Type == actionCreate {
				createdBy[action.Target] = i
			}
		}
	}

	// Check if any inject targets are created by a later generator.
	for i, genName := range order {
		for _, action := range genActions[genName] {
			if action.Type == actionInject {
				if createIdx, ok := createdBy[action.Target]; ok && createIdx > i {
					return false
				}
			}
		}
	}

	return true
}

// collectAllMarkers returns a set of all injection markers used by generators.
func collectAllMarkers(generators []GeneratorNode) map[string][]string {
	// marker text -> list of generator names
	markers := make(map[string][]string)
	for _, gen := range generators {
		for _, action := range gen.Actions {
			if action.Type == actionInject && action.Marker != "" {
				markers[action.Marker] = append(markers[action.Marker], gen.Name)
			}
		}
	}
	return markers
}

// scanMarkers walks the scaffold source files looking for injection marker
// strings. It skips .tag/, _generators/, and config files.
func scanMarkers(root string, markers map[string][]string) []MarkerInfo {
	if len(markers) == 0 {
		return nil
	}

	var found []MarkerInfo

	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil //nolint:nilerr // best-effort scan
		}

		if skip, skipDir := shouldSkipMarkerEntry(root, path, d); skip {
			if skipDir {
				return filepath.SkipDir
			}
			return nil
		}

		fileMarkers := scanFileForMarkers(root, path, markers)
		found = append(found, fileMarkers...)

		return nil
	})

	// Sort by file then line for deterministic output.
	slices.SortFunc(found, func(a, b MarkerInfo) int {
		if cmp := strings.Compare(a.File, b.File); cmp != 0 {
			return cmp
		}
		return a.Line - b.Line
	})

	return found
}

// shouldSkipMarkerEntry returns whether to skip a path during marker scanning.
func shouldSkipMarkerEntry(root, path string, d fs.DirEntry) (skip, skipDir bool) {
	relPath, relErr := filepath.Rel(root, path)
	if relErr != nil {
		return true, false
	}

	if d.IsDir() {
		name := d.Name()
		if (name == types.TemplatesDir || name == types.GeneratorsDir || strings.HasPrefix(name, ".")) && relPath != "." {
			return true, true
		}
		return true, false
	}

	if filepath.Base(path) == types.TemplateConfigFile {
		return true, false
	}

	return false, false
}

// scanFileForMarkers reads a file and searches for marker strings.
func scanFileForMarkers(root, path string, markers map[string][]string) []MarkerInfo {
	content, readErr := os.ReadFile(path)
	if readErr != nil {
		return nil
	}

	if !fileutil.IsTextContent(content) {
		return nil
	}

	relPath, relErr := filepath.Rel(root, path)
	if relErr != nil {
		return nil
	}

	var found []MarkerInfo
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		for markerText, usedBy := range markers {
			if strings.Contains(line, markerText) {
				found = append(found, MarkerInfo{
					File:   relPath,
					Line:   lineNum,
					Text:   markerText,
					UsedBy: usedBy,
				})
			}
		}
	}

	return found
}

// generateWarnings produces cross-reference warnings from the analysis.
func generateWarnings(report *GraphReport) []Warning {
	var warnings []Warning
	warnings = append(warnings, checkFileConflicts(report.Generators)...)
	warnings = append(warnings, checkMissingTargets(report.Generators)...)
	warnings = append(warnings, checkBundleOrderViolations(report.Bundles)...)

	// Sort warnings for deterministic output.
	slices.SortFunc(warnings, func(a, b Warning) int {
		if cmp := strings.Compare(a.Code, b.Code); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.Message, b.Message)
	})

	return warnings
}

// checkFileConflicts warns when multiple generators create the same file.
func checkFileConflicts(generators []GeneratorNode) []Warning {
	// target -> list of generator names that create it
	creators := make(map[string][]string)
	for _, gen := range generators {
		for _, action := range gen.Actions {
			if action.Type == actionCreate {
				creators[action.Target] = append(creators[action.Target], gen.Name)
			}
		}
	}

	var warnings []Warning
	for target, gens := range creators {
		if len(gens) > 1 {
			warnings = append(warnings, Warning{
				Code:    "file_conflict",
				Message: fmt.Sprintf("file %q is created by multiple generators: %s", target, strings.Join(gens, ", ")),
			})
		}
	}
	return warnings
}

// checkMissingTargets warns when an inject target is not created by any generator.
func checkMissingTargets(generators []GeneratorNode) []Warning {
	createdTargets := make(map[string]struct{})
	for _, gen := range generators {
		for _, action := range gen.Actions {
			if action.Type == actionCreate {
				createdTargets[action.Target] = struct{}{}
			}
		}
	}

	var warnings []Warning
	for _, gen := range generators {
		for _, action := range gen.Actions {
			if action.Type != actionInject {
				continue
			}
			if _, ok := createdTargets[action.Target]; ok {
				continue
			}
			// Skip targets with template variables — can't evaluate statically.
			if strings.Contains(action.Target, "{{") {
				continue
			}
			warnings = append(warnings, Warning{
				Code:      "missing_target",
				Generator: gen.Name,
				Message:   fmt.Sprintf("generator %q injects into %q, but no generator creates it (may exist in scaffold)", gen.Name, action.Target),
			})
		}
	}
	return warnings
}

// checkBundleOrderViolations warns about bundles with bad execution order.
func checkBundleOrderViolations(bundles []BundleInfo) []Warning {
	var warnings []Warning
	for _, bundle := range bundles {
		if !bundle.ValidOrder {
			warnings = append(warnings, Warning{
				Code:    "order_violation",
				Message: fmt.Sprintf("bundle %q has generators that inject before their target is created", bundle.Name),
			})
		}
	}
	return warnings
}
