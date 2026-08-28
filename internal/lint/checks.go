package lint

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

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"

	"github.com/kaikenlabs/tag/internal/engine"
	"github.com/kaikenlabs/tag/internal/fileutil"
	"github.com/kaikenlabs/tag/internal/schema"
	tmpl "github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/tmplconfig"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/internal/validate"
	"github.com/kaikenlabs/tag/internal/vars"
)

// lintSchema validates tag.template.json against the JSON Schema and parses it.
func (l *Linter) lintSchema() {
	configPath := filepath.Join(l.root, types.TemplateConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		l.result.Add(Issue{
			File:     types.TemplateConfigFile,
			Severity: SeverityError,
			Message:  fmt.Sprintf("cannot read file: %s", err),
			Rule:     "config-read",
		})
		return
	}

	if validateErr := l.validator.Validate(data); validateErr != nil {
		var validationErr *schema.ValidationError
		if errors.As(validateErr, &validationErr) {
			for _, e := range validationErr.Errors {
				l.result.Add(Issue{
					File:     types.TemplateConfigFile,
					Severity: SeverityError,
					Message:  e,
					Rule:     "schema-validation",
				})
			}
		} else {
			l.result.Add(Issue{
				File:     types.TemplateConfigFile,
				Severity: SeverityError,
				Message:  fmt.Sprintf("schema validation failed: %s", validateErr),
				Rule:     "schema-validation",
			})
		}
	}

	config, err := tmplconfig.ParseTemplateConfig(data)
	if err != nil {
		l.result.Add(Issue{
			File:     types.TemplateConfigFile,
			Severity: SeverityError,
			Message:  fmt.Sprintf("config parse error: %s", err),
			Rule:     "config-parse",
		})
		return
	}

	l.config = config
}

// lintDerivedDefaults checks derived and evaluated-default variable expressions
// for references to undefined variables.
func (l *Linter) lintDerivedDefaults() {
	names := sortedKeys(l.config.Vars)
	for _, name := range names {
		def := l.config.Vars[name]
		if !def.IsDerived() && !def.IsEvaluatedDefault() {
			continue
		}
		defaultStr, ok := def.Default.(string)
		if !ok {
			continue
		}
		for _, refName := range vars.ScanNames(defaultStr) {
			if _, exists := l.vars[refName]; !exists {
				l.result.Add(Issue{
					File:     types.TemplateConfigFile,
					Severity: SeverityError,
					Message:  fmt.Sprintf("derived variable %q references undefined variable %q", name, refName),
					Rule:     "undefined-variable",
				})
			}
		}
	}
}

// lintTemplateFiles walks the template directory and lints each file.
func (l *Linter) lintTemplateFiles() error {
	ignoreMatcher, err := loadIgnorePatterns(l.root)
	if err != nil {
		return fmt.Errorf("load ignore patterns: %w", err)
	}

	return filepath.WalkDir(l.root, func(srcPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(l.root, srcPath)
		if err != nil {
			return fmt.Errorf("relative path: %w", err)
		}

		if relPath == "." {
			return nil
		}

		// Skip symlinks (WalkDir does not follow them, but skip explicitly)
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		// Skip config files and special directories
		if isSkippedEntry(relPath, d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Apply .tagignore patterns
		if ignoreMatcher != nil {
			pathComponents := strings.Split(relPath, string(filepath.Separator))
			if ignoreMatcher.Match(pathComponents, d.IsDir()) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Lint path placeholders for variable references
		l.lintPath(relPath)

		if !d.IsDir() {
			l.lintFileContent(srcPath, relPath)
		}

		return nil
	})
}

// lintPath checks path segments for undefined variable references.
func (l *Linter) lintPath(relPath string) {
	if l.config == nil {
		return
	}
	for _, refName := range vars.ScanNames(relPath) {
		if _, exists := l.vars[refName]; !exists {
			l.result.Add(Issue{
				File:     relPath,
				Severity: SeverityError,
				Message:  fmt.Sprintf("path contains undefined variable %q", refName),
				Rule:     "undefined-variable",
			})
		}
	}
}

// lintFileContent parses a template file and checks for issues.
func (l *Linter) lintFileContent(absPath, relPath string) {
	content, err := os.ReadFile(absPath)
	if err != nil {
		l.result.Add(Issue{
			File:     relPath,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("cannot read file: %s", err),
			Rule:     "file-read",
		})
		return
	}

	// Skip binary files
	if !fileutil.IsTextContent(content) {
		return
	}

	// Gonja syntax validation (parse without execute)
	_, parseErr := l.engine.ParseStringNamed(string(content), relPath)
	if parseErr != nil {
		issue := Issue{
			File:     relPath,
			Severity: SeverityError,
			Message:  parseErr.Error(),
			Rule:     "template-syntax",
		}
		var tmplErr *tmpl.TemplateError
		if errors.As(parseErr, &tmplErr) {
			issue.Line = tmplErr.Line
			issue.Column = tmplErr.Column
			issue.Message = tmplErr.Err.Error()
		}
		l.result.Add(issue)
	}

	// Variable cross-reference
	if l.config == nil {
		return
	}
	l.lintVariableRefs(string(content), relPath)
}

// lintVariableRefs scans content for {{ vars.* }} references and checks against declared vars.
func (l *Linter) lintVariableRefs(content, relPath string) {
	// ScanRefs skips comments and {% raw %} bodies itself: their contents are
	// emitted literally, not evaluated, so they are not variable references.
	//
	// It reports every occurrence, which is what `tag template variables` needs
	// for its reference counts. A linter wants one finding per location, so
	// collapse repeats of the same name on the same line — otherwise
	// `{{ vars.x }} and {{ vars.x }}` emits the identical message twice.
	type nameOnLine struct {
		name string
		line int
	}
	reported := make(map[nameOnLine]struct{})

	for _, ref := range vars.ScanRefs(content) {
		if _, exists := l.vars[ref.Name]; exists {
			continue
		}
		key := nameOnLine{name: ref.Name, line: ref.Line}
		if _, dup := reported[key]; dup {
			continue
		}
		reported[key] = struct{}{}

		l.result.Add(Issue{
			File:     relPath,
			Line:     ref.Line,
			Severity: SeverityError,
			Message:  fmt.Sprintf("undefined variable %q", ref.Name),
			Rule:     "undefined-variable",
		})
	}
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

// isSkippedEntry determines if a file or directory should be skipped during linting.
func isSkippedEntry(relPath, name string) bool {
	atRoot := filepath.Dir(relPath) == "."

	// Root-only files
	if atRoot && (name == types.TemplateConfigFile || name == types.CacheMetaFile || name == types.TagIgnoreFile) {
		return true
	}

	// _generators directory tree
	if relPath == types.GeneratorsDir || strings.HasPrefix(relPath, types.GeneratorsDir+string(filepath.Separator)) {
		return true
	}

	// _dialects directory tree
	if relPath == types.DialectsDir || strings.HasPrefix(relPath, types.DialectsDir+string(filepath.Separator)) {
		return true
	}

	// .tag directory tree
	if relPath == types.TemplatesDir || strings.HasPrefix(relPath, types.TemplatesDir+string(filepath.Separator)) {
		return true
	}

	return false
}

// lintGeneratorNames checks generator and bundle names against reserved subcommand names.
func (l *Linter) lintGeneratorNames() {
	l.lintGeneratorDirs()
	l.lintBundleNames()
}

// lintGeneratorDirNames scans _generators/ for directories whose names conflict with subcommands.
// lintGeneratorDirs scans _generators/ for directories whose names conflict with
// subcommands and for directories that hold nothing "tag generate" would run.
//
// Only _generators/ is scanned, not the .tag/ layout a template may also ship
// (see #431): that keeps this check's scope identical to the reserved-name check
// it shares a loop with, and widening it would change that shipped rule too.
func (l *Linter) lintGeneratorDirs() {
	genDir := filepath.Join(l.root, types.GeneratorsDir)
	entries, err := os.ReadDir(genDir)
	if err != nil {
		return // _generators/ may not exist
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == types.BundlesDir || name == types.SharedDir {
			continue
		}
		relPath := filepath.Join(types.GeneratorsDir, name)
		if err := validate.GeneratorName(name); err != nil {
			l.result.Add(Issue{
				File:     relPath,
				Severity: SeverityError,
				Message:  fmt.Sprintf("generator name %q conflicts with a \"tag generate\" subcommand", name),
				Rule:     "reserved-name",
			})
		}
		if !engine.HasTemplateFiles(filepath.Join(genDir, name)) {
			l.result.Add(Issue{
				File:     relPath,
				Severity: SeverityWarning,
				Message: fmt.Sprintf(
					"generator %q holds no template files, so \"tag generate\" will not see it; %s and subdirectories do not count (template files are not loaded recursively)",
					name, types.TemplateConfigFile,
				),
				Rule: "empty-generator",
			})
		}
	}
}

// lintBundleNames scans _bundles/ for bundle JSON files and checks names.
func (l *Linter) lintBundleNames() {
	bundleDir := filepath.Join(l.root, types.GeneratorsDir, types.BundlesDir)
	entries, err := os.ReadDir(bundleDir)
	if err != nil {
		return // _bundles/ may not exist
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		bundleFile := filepath.Join(bundleDir, entry.Name(), entry.Name()+types.BundleExtension)
		data, readErr := os.ReadFile(bundleFile)
		if readErr != nil {
			continue
		}
		var bundle engine.Bundle
		if jsonErr := json.Unmarshal(data, &bundle); jsonErr != nil {
			continue
		}
		relPath := filepath.Join(types.GeneratorsDir, types.BundlesDir, entry.Name(), entry.Name()+types.BundleExtension)
		if err := validate.GeneratorName(bundle.Name); err != nil {
			l.result.Add(Issue{
				File:     relPath,
				Severity: SeverityError,
				Message:  fmt.Sprintf("bundle name %q conflicts with a \"tag generate\" subcommand", bundle.Name),
				Rule:     "reserved-name",
			})
		}
		for _, gen := range bundle.Generators {
			if err := validate.GeneratorName(gen.Name); err != nil {
				l.result.Add(Issue{
					File:     relPath,
					Severity: SeverityError,
					Message:  fmt.Sprintf("generator reference %q in bundle conflicts with a \"tag generate\" subcommand", gen.Name),
					Rule:     "reserved-name",
				})
			}
		}
	}
}

// sortedKeys returns the keys of a map in sorted order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
