package lint

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"

	"github.com/kaikenlabs/tag/internal/fileutil"
	"github.com/kaikenlabs/tag/internal/schema"
	tmpl "github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/tmplconfig"
	"github.com/kaikenlabs/tag/internal/types"
)

// varExprRegex matches {{ vars.NAME ... }} expressions, capturing the variable name.
var varExprRegex = regexp.MustCompile(`\{\{[^}]*\bvars\.([a-zA-Z_][a-zA-Z0-9_]*)\b[^}]*\}\}`)

// varStmtRegex matches {% ... vars.NAME ... %} statements (if, for, set, etc.).
var varStmtRegex = regexp.MustCompile(`\{%[^%]*\bvars\.([a-zA-Z_][a-zA-Z0-9_]*)\b[^%]*%\}`)

// commentRegex matches Gonja comments {# ... #}, including multi-line.
var commentRegex = regexp.MustCompile(`(?s)\{#.*?#\}`)

// VarRef holds a variable reference found in template content.
type VarRef struct {
	Name string
	Line int
}

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
		refs := extractVarRefs(defaultStr, 0)
		for _, ref := range refs {
			if _, exists := l.vars[ref.Name]; !exists {
				l.result.Add(Issue{
					File:     types.TemplateConfigFile,
					Severity: SeverityError,
					Message:  fmt.Sprintf("derived variable %q references undefined variable %q", name, ref.Name),
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
	refs := extractVarRefs(relPath, 0)
	for _, ref := range refs {
		if _, exists := l.vars[ref.Name]; !exists {
			l.result.Add(Issue{
				File:     relPath,
				Severity: SeverityError,
				Message:  fmt.Sprintf("path contains undefined variable %q", ref.Name),
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
	// Strip comments before scanning, preserving newlines to keep line numbers accurate.
	cleaned := commentRegex.ReplaceAllStringFunc(content, func(match string) string {
		return strings.Repeat("\n", strings.Count(match, "\n"))
	})

	scanner := bufio.NewScanner(strings.NewReader(cleaned))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		refs := extractVarRefs(line, lineNum)
		for _, ref := range refs {
			if _, exists := l.vars[ref.Name]; !exists {
				l.result.Add(Issue{
					File:     relPath,
					Line:     ref.Line,
					Severity: SeverityError,
					Message:  fmt.Sprintf("undefined variable %q", ref.Name),
					Rule:     "undefined-variable",
				})
			}
		}
	}
}

// extractVarRefs extracts all vars.NAME references from a string.
func extractVarRefs(content string, lineNum int) []VarRef {
	var refs []VarRef
	seen := make(map[string]struct{})

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
			refs = append(refs, VarRef{Name: name, Line: lineNum})
		}
	}
	return refs
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

	// .tag directory tree
	if relPath == types.TemplatesDir || strings.HasPrefix(relPath, types.TemplatesDir+string(filepath.Separator)) {
		return true
	}

	return false
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
