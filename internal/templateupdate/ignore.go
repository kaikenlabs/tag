package templateupdate

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"

	"github.com/kaikenlabs/tag/internal/types"
)

// builtinIgnorePatterns are always excluded from template updates.
var builtinIgnorePatterns = []string{
	".git",
	types.TemplatesDir,  // .tag
	types.TagConfigFile, // .tagconfig.json
}

// IgnoreMatcher determines whether a file should be skipped during template
// updates. It merges patterns from four sources in priority order:
//
//  1. Built-in defaults (lowest priority)
//  2. .tagignore file (gitignore-style)
//  3. .tagconfig.json skip_patterns
//  4. CLI --skip flags (highest priority)
//
// All sources are combined into a single gitignore-style matcher. Because the
// go-git matcher evaluates patterns in order and the last matching pattern wins,
// we append sources from lowest to highest priority so that higher-priority
// sources can override lower ones (including negation patterns).
type IgnoreMatcher struct {
	matcher gitignore.Matcher
}

// IgnoreMatcherOptions configures the pattern sources for an IgnoreMatcher.
type IgnoreMatcherOptions struct {
	// ProjectRoot is the path to the project directory. Used to read
	// .tagignore and .tagconfig.json. May be empty if patterns are
	// supplied directly.
	ProjectRoot string

	// TagignorePatterns are patterns parsed from a .tagignore file.
	// If nil and ProjectRoot is set, patterns are loaded from the file.
	TagignorePatterns []string

	// TagconfigPatterns are patterns from .tagconfig.json skip_patterns.
	// If nil and ProjectRoot is set, patterns are loaded from the file.
	TagconfigPatterns []string

	// CLIPatterns are patterns from --skip CLI flags.
	CLIPatterns []string
}

// NewIgnoreMatcher creates an IgnoreMatcher by merging patterns from all
// configured sources.
func NewIgnoreMatcher(opts IgnoreMatcherOptions) (*IgnoreMatcher, error) {
	var allPatterns []gitignore.Pattern

	// 1. Built-in defaults (lowest priority).
	for _, p := range builtinIgnorePatterns {
		allPatterns = append(allPatterns, gitignore.ParsePattern(p, nil))
	}

	// 2. .tagignore patterns.
	tagignore := opts.TagignorePatterns
	if tagignore == nil && opts.ProjectRoot != "" {
		loaded, err := loadTagignorePatterns(opts.ProjectRoot)
		if err != nil {
			return nil, fmt.Errorf("load .tagignore: %w", err)
		}
		tagignore = loaded
	}
	for _, p := range tagignore {
		allPatterns = append(allPatterns, gitignore.ParsePattern(p, nil))
	}

	// 3. .tagconfig.json skip_patterns.
	tagconfig := opts.TagconfigPatterns
	if tagconfig == nil && opts.ProjectRoot != "" {
		loaded, err := loadTagconfigSkipPatterns(opts.ProjectRoot)
		if err != nil {
			return nil, fmt.Errorf("load tagconfig skip_patterns: %w", err)
		}
		tagconfig = loaded
	}
	for _, p := range tagconfig {
		allPatterns = append(allPatterns, gitignore.ParsePattern(p, nil))
	}

	// 4. CLI --skip flags (highest priority).
	for _, p := range opts.CLIPatterns {
		allPatterns = append(allPatterns, gitignore.ParsePattern(p, nil))
	}

	return &IgnoreMatcher{
		matcher: gitignore.NewMatcher(allPatterns),
	}, nil
}

// ShouldSkip reports whether the given relative path should be excluded from
// template update operations.
func (m *IgnoreMatcher) ShouldSkip(relPath string, isDir bool) bool {
	components := strings.Split(filepath.ToSlash(relPath), "/")
	return m.matcher.Match(components, isDir)
}

// loadTagignorePatterns reads pattern lines from a .tagignore file. Returns nil
// (not an error) when the file does not exist.
func loadTagignorePatterns(projectRoot string) ([]string, error) {
	f, err := os.Open(filepath.Join(projectRoot, types.TagIgnoreFile))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("open %s: %w", types.TagIgnoreFile, err)
	}
	defer f.Close()

	return parseIgnoreLines(f)
}

// parseIgnoreLines reads gitignore-style lines from a reader, skipping blanks
// and comments.
func parseIgnoreLines(r io.Reader) ([]string, error) {
	var patterns []string

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read patterns: %w", err)
	}

	return patterns, nil
}

// tagconfigPartial is the minimal structure needed to read skip_patterns from
// .tagconfig.json.
type tagconfigPartial struct {
	SkipPatterns []string `json:"skip_patterns"`
}

// loadTagconfigSkipPatterns reads the skip_patterns array from .tagconfig.json.
// Returns nil (not an error) when the file does not exist.
func loadTagconfigSkipPatterns(projectRoot string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(projectRoot, types.TagConfigFile))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", types.TagConfigFile, err)
	}

	var cfg tagconfigPartial
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", types.TagConfigFile, err)
	}

	return cfg.SkipPatterns, nil
}
