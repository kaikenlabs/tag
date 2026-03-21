package vars

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"

	"github.com/kaikenlabs/tag/internal/fileutil"
	"github.com/kaikenlabs/tag/internal/tmplconfig"
	"github.com/kaikenlabs/tag/internal/types"
)

// varExprRegex matches {{ vars.NAME ... }} expressions, capturing the variable name.
var varExprRegex = regexp.MustCompile(`\{\{[^}]*\bvars\.([a-zA-Z_][a-zA-Z0-9_]*)\b[^}]*\}\}`)

// varStmtRegex matches {% ... vars.NAME ... %} statements (if, for, set, etc.).
var varStmtRegex = regexp.MustCompile(`\{%[^%]*\bvars\.([a-zA-Z_][a-zA-Z0-9_]*)\b[^%]*%\}`)

// commentRegex matches Gonja comments {# ... #}, including multi-line.
var commentRegex = regexp.MustCompile(`(?s)\{#.*?#\}`)

// Analyze scans a template directory and returns a Report of declared vs
// referenced variables, including undeclared and unused findings.
func Analyze(root string) (*Report, error) {
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

	// Parse root config.
	rootConfig, err := loadConfig(absRoot)
	if err != nil {
		return nil, err
	}

	// Analyze root scope (skips _generators/).
	rootResult, err := analyzeScope(absRoot, "root", rootConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("analyze root: %w", err)
	}

	report := &Report{Root: *rootResult}

	// Discover and analyze generators.
	genDir := filepath.Join(absRoot, types.GeneratorsDir)
	genEntries, err := os.ReadDir(genDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read generators dir: %w", err)
	}

	for _, entry := range genEntries {
		if !entry.IsDir() {
			continue
		}
		genPath := filepath.Join(genDir, entry.Name())
		genConfig, configErr := loadConfig(genPath)
		if configErr != nil {
			// Generator may not have its own config; that's fine.
			genConfig = &tmplconfig.TemplateConfig{
				Vars: make(map[string]tmplconfig.VariableDef),
			}
		}
		scopeName := types.GeneratorsDir + "/" + entry.Name()
		genResult, scanErr := analyzeScope(genPath, scopeName, genConfig, rootConfig)
		if scanErr != nil {
			return nil, fmt.Errorf("analyze generator %s: %w", entry.Name(), scanErr)
		}
		report.Generators = append(report.Generators, *genResult)
	}

	return report, nil
}

// loadConfig reads and parses the tag.template.json in the given directory.
func loadConfig(dir string) (*tmplconfig.TemplateConfig, error) {
	configPath := filepath.Join(dir, types.TemplateConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", types.TemplateConfigFile, err)
	}
	config, err := tmplconfig.ParseTemplateConfig(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", types.TemplateConfigFile, err)
	}
	return config, nil
}

// analyzeScope scans a single scope (root or generator directory) and
// cross-references declared variables against their usage.
// parentConfig is non-nil for generator scopes and provides root-level
// variable declarations that are also valid in generators.
func analyzeScope(
	dir, scopeName string,
	config *tmplconfig.TemplateConfig,
	parentConfig *tmplconfig.TemplateConfig,
) (*ScopeResult, error) {
	// Build declared vars map.
	declared := make(map[string]tmplconfig.VariableDef, len(config.Vars))
	maps.Copy(declared, config.Vars)

	// Build the full set of valid vars (includes parent for generators).
	validVars := make(map[string]struct{})
	for name := range declared {
		validVars[name] = struct{}{}
	}
	if parentConfig != nil {
		for name := range parentConfig.Vars {
			validVars[name] = struct{}{}
		}
	}

	// Collect references from template files.
	refs, err := scanFiles(dir, scopeName == "root")
	if err != nil {
		return nil, err
	}

	// Also scan derived variable defaults for references.
	for _, def := range config.Vars {
		if def.Default == nil {
			continue
		}
		defaultStr, ok := def.Default.(string)
		if !ok {
			continue
		}
		for _, name := range extractVarNames(defaultStr) {
			refs[name] = append(refs[name], Reference{
				File:       types.TemplateConfigFile,
				Line:       0,
				Expression: defaultStr,
			})
		}
	}

	// Build result.
	result := &ScopeResult{
		Scope:      scopeName,
		Declared:   make([]DeclaredVar, 0, len(declared)),
		Undeclared: []UndeclaredVar{},
		Unused:     []string{},
	}

	// Populate declared vars with reference counts.
	for name, def := range declared {
		dv := newDeclaredVar(name, def)
		if varRefs, ok := refs[name]; ok {
			dv.References = varRefs
			dv.ReferenceCount = len(varRefs)
			// Count unique files.
			files := make(map[string]struct{})
			for _, r := range varRefs {
				files[r.File] = struct{}{}
			}
			dv.FileCount = len(files)
		}
		result.Declared = append(result.Declared, dv)
	}

	// Sort declared vars by name for deterministic output.
	slices.SortFunc(result.Declared, func(a, b DeclaredVar) int {
		return strings.Compare(a.Name, b.Name)
	})

	// Find undeclared vars.
	for name, varRefs := range refs {
		if _, ok := validVars[name]; !ok {
			result.Undeclared = append(result.Undeclared, UndeclaredVar{
				Name:       name,
				References: varRefs,
			})
		}
	}
	slices.SortFunc(result.Undeclared, func(a, b UndeclaredVar) int {
		return strings.Compare(a.Name, b.Name)
	})

	// Find unused vars (only for vars declared in this scope, not parent).
	for name := range declared {
		if _, ok := refs[name]; !ok {
			result.Unused = append(result.Unused, name)
		}
	}
	slices.Sort(result.Unused)

	result.Summary = Summary{
		Declared:   len(result.Declared),
		Undeclared: len(result.Undeclared),
		Unused:     len(result.Unused),
	}

	return result, nil
}

// scanFiles walks a directory tree and collects all variable references.
// When isRoot is true, it skips the _generators/ directory.
// Returns a map of variable name → list of references.
func scanFiles(dir string, isRoot bool) (map[string][]Reference, error) {
	refs := make(map[string][]Reference)

	ignoreMatcher, err := loadIgnorePatterns(dir)
	if err != nil {
		return nil, fmt.Errorf("load ignore patterns: %w", err)
	}

	err = filepath.WalkDir(dir, func(srcPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relPath, relErr := filepath.Rel(dir, srcPath)
		if relErr != nil {
			return fmt.Errorf("relative path: %w", relErr)
		}

		if skip, skipDir := shouldSkipEntry(relPath, d, isRoot, ignoreMatcher); skip {
			if skipDir {
				return filepath.SkipDir
			}
			return nil
		}

		// Extract refs from path segments.
		for _, name := range extractVarNames(relPath) {
			refs[name] = append(refs[name], Reference{
				File: relPath,
				Line: 0,
			})
		}

		// Extract refs from file content.
		if !d.IsDir() {
			collectFileRefs(srcPath, relPath, refs)
		}

		return nil
	})

	return refs, err
}

// shouldSkipEntry returns whether to skip this entry and whether to skip the
// entire directory subtree.
func shouldSkipEntry(
	relPath string, d fs.DirEntry, isRoot bool, ignoreMatcher gitignore.Matcher,
) (skip, skipDir bool) {
	if relPath == "." {
		return true, false
	}

	// Skip symlinks.
	if d.Type()&os.ModeSymlink != 0 {
		return true, false
	}

	// Skip config files and special directories.
	if isSkippedEntry(relPath, d.Name(), isRoot) {
		return true, d.IsDir()
	}

	// Apply .tagignore patterns.
	if ignoreMatcher != nil {
		pathComponents := strings.Split(relPath, string(filepath.Separator))
		if ignoreMatcher.Match(pathComponents, d.IsDir()) {
			return true, d.IsDir()
		}
	}

	return false, false
}

// collectFileRefs reads a file and appends variable references to the refs map.
func collectFileRefs(absPath, relPath string, refs map[string][]Reference) {
	fileRefs := scanFileContent(absPath, relPath)
	for name, fileRefList := range fileRefs {
		refs[name] = append(refs[name], fileRefList...)
	}
}

// scanFileContent reads a file and extracts variable references from its content.
// Returns nil for unreadable or binary files.
func scanFileContent(absPath, relPath string) map[string][]Reference {
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil
	}

	// Skip binary files.
	if !fileutil.IsTextContent(content) {
		return nil
	}

	refs := make(map[string][]Reference)

	// Strip comments before scanning, preserving newlines for line numbers.
	cleaned := commentRegex.ReplaceAllStringFunc(string(content), func(match string) string {
		return strings.Repeat("\n", strings.Count(match, "\n"))
	})

	scanner := bufio.NewScanner(strings.NewReader(cleaned))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		for _, name := range extractVarNames(line) {
			refs[name] = append(refs[name], Reference{
				File:       relPath,
				Line:       lineNum,
				Expression: strings.TrimSpace(line),
			})
		}
	}

	return refs
}

// extractVarNames extracts all variable names from vars.NAME references in content.
func extractVarNames(content string) []string {
	seen := make(map[string]struct{})
	var names []string

	for _, re := range []*regexp.Regexp{varExprRegex, varStmtRegex} {
		matches := re.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			name := match[1]
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			names = append(names, name)
		}
	}
	return names
}

// isSkippedEntry returns true if the entry should be skipped during walking.
func isSkippedEntry(relPath, name string, isRoot bool) bool {
	atRoot := filepath.Dir(relPath) == "."

	// Root-only files.
	if atRoot && (name == types.TemplateConfigFile || name == types.CacheMetaFile || name == types.TagIgnoreFile) {
		return true
	}

	// _generators directory tree (only skip when scanning root scope).
	if isRoot {
		if relPath == types.GeneratorsDir || strings.HasPrefix(relPath, types.GeneratorsDir+string(filepath.Separator)) {
			return true
		}
	}

	// _dialects directory tree.
	if relPath == types.DialectsDir || strings.HasPrefix(relPath, types.DialectsDir+string(filepath.Separator)) {
		return true
	}

	// .tag directory tree.
	if relPath == types.TemplatesDir || strings.HasPrefix(relPath, types.TemplatesDir+string(filepath.Separator)) {
		return true
	}

	return false
}

//nolint:nilnil // nil matcher signals "no ignore file" — callers nil-check before use
func loadIgnorePatterns(templateRoot string) (gitignore.Matcher, error) {
	f, err := os.Open(filepath.Join(templateRoot, types.TagIgnoreFile))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("open %s: %w", types.TagIgnoreFile, err)
	}
	defer f.Close()

	var patterns []gitignore.Pattern
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, gitignore.ParsePattern(line, nil))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", types.TagIgnoreFile, err)
	}

	if len(patterns) == 0 {
		return nil, nil
	}
	return gitignore.NewMatcher(patterns), nil
}
