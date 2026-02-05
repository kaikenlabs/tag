package scaffold

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gobuffalo/flect"
	"github.com/kaikenlabs/tag/internal/formats"
)

// PathProcessor handles path placeholder substitution.
type PathProcessor interface {
	// ProcessPath processes a path, replacing __var__ and __var | filter__ placeholders.
	ProcessPath(path string, vars map[string]any) (string, error)
}

// DefaultPathProcessor implements PathProcessor using the filter registry.
type DefaultPathProcessor struct{}

// NewPathProcessor creates a new path processor.
func NewPathProcessor() *DefaultPathProcessor {
	return &DefaultPathProcessor{}
}

// placeholderRegex matches __var__ and __var | filter__ patterns.
// Examples: __project_name__, __project_name | snake__, __var|filter__
var placeholderRegex = regexp.MustCompile(`__([a-zA-Z_][a-zA-Z0-9_]*)\s*(?:\|\s*([a-zA-Z_][a-zA-Z0-9_]*))?\s*__`)

// ProcessPath replaces placeholders in a path with variable values.
// Supports both __var__ and __var | filter__ syntax.
func (p *DefaultPathProcessor) ProcessPath(path string, vars map[string]any) (string, error) {
	// Split path into segments to process each part
	segments := strings.Split(path, string(filepath.Separator))
	processedSegments := make([]string, 0, len(segments))

	for _, segment := range segments {
		processed, err := p.processSegment(segment, vars)
		if err != nil {
			return "", NewPathError(path, "failed to process segment", err)
		}
		// Skip empty segments (from placeholders resolving to empty string)
		if processed != "" {
			processedSegments = append(processedSegments, processed)
		}
	}

	return filepath.Join(processedSegments...), nil
}

// processSegment processes placeholders in a single path segment.
func (p *DefaultPathProcessor) processSegment(segment string, vars map[string]any) (string, error) {
	var lastErr error

	result := placeholderRegex.ReplaceAllStringFunc(segment, func(match string) string {
		if lastErr != nil {
			return match // Skip if we already have an error
		}

		// Parse the placeholder
		submatches := placeholderRegex.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		varName := submatches[1]
		filterName := ""
		if len(submatches) >= 3 && submatches[2] != "" {
			filterName = submatches[2]
		}

		// Look up the variable
		value, ok := vars[varName]
		if !ok {
			lastErr = fmt.Errorf("undefined variable: %s", varName)
			return match
		}

		// Convert value to string
		strValue := fmt.Sprintf("%v", value)

		// Apply filter if specified
		if filterName != "" {
			filtered, err := applyPathFilter(strValue, filterName)
			if err != nil {
				lastErr = fmt.Errorf("filter %q on variable %q: %w", filterName, varName, err)
				return match
			}
			strValue = filtered
		}

		return strValue
	})

	if lastErr != nil {
		return "", lastErr
	}

	return result, nil
}

// Whitelist of safe filters for path processing.
// These filters only transform strings and don't have side effects.
var safePathFilters = map[string]func(string) string{
	"snake":       formats.CaseSnake,
	"snake_case":  formats.CaseSnake,
	"pascal":      formats.CasePascal,
	"pascal_case": formats.CasePascal,
	"camel":       formats.CaseCamel,
	"camel_case":  formats.CaseCamel,
	"kebab":       formats.CaseKebab,
	"kebab_case":  formats.CaseKebab,
	"lower":       strings.ToLower,
	"upper":       strings.ToUpper,
	"plural":      flect.Pluralize,
	"pluralize":   flect.Pluralize,
	"singular":    flect.Singularize,
	"singularize": flect.Singularize,
}

// applyPathFilter applies a filter to a string value.
// Only filters in the whitelist are allowed for path processing.
func applyPathFilter(value, filterName string) (string, error) {
	filterFn, ok := safePathFilters[filterName]
	if !ok {
		return "", fmt.Errorf("unknown or unsupported filter: %s (supported: snake, pascal, camel, kebab, lower, upper, plural, singular)", filterName)
	}
	return filterFn(value), nil
}

// ExtractPlaceholders returns all placeholder variable names found in a path.
func ExtractPlaceholders(path string) []string {
	matches := placeholderRegex.FindAllStringSubmatch(path, -1)
	vars := make([]string, 0, len(matches))
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) >= 2 && !seen[match[1]] {
			vars = append(vars, match[1])
			seen[match[1]] = true
		}
	}

	return vars
}

// HasPlaceholders checks if a path contains any placeholders.
func HasPlaceholders(path string) bool {
	return placeholderRegex.MatchString(path)
}

// StripTemplateExtension removes the .tmpl extension from a filename.
func StripTemplateExtension(filename string) string {
	if strings.HasSuffix(filename, ".tmpl") {
		return strings.TrimSuffix(filename, ".tmpl")
	}
	return filename
}

// IsTemplateFile checks if a file should be processed as a template.
func IsTemplateFile(filename string) bool {
	return strings.HasSuffix(filename, ".tmpl")
}
