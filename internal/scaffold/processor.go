package scaffold

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kaikenlabs/tag/internal/template"
)

// PathProcessor handles path placeholder substitution.
type PathProcessor interface {
	// ProcessPath processes a path, replacing Jinja2 expressions like {{ vars.name }}.
	ProcessPath(path string, vars map[string]any) (string, error)
}

// DefaultPathProcessor implements PathProcessor using the Gonja template engine.
type DefaultPathProcessor struct {
	engine *template.Engine
}

// NewPathProcessor creates a new path processor.
func NewPathProcessor() (*DefaultPathProcessor, error) {
	engine, err := template.NewEngine()
	if err != nil {
		return nil, fmt.Errorf("failed to create path processor: %w", err)
	}
	return &DefaultPathProcessor{engine: engine}, nil
}

// placeholderDetectRegex detects if a string contains Jinja2-style template syntax.
// Matches both {{ expressions }} and {% statements %} (e.g., conditionals).
// This is a simple check - the actual parsing is done by Gonja.
var placeholderDetectRegex = regexp.MustCompile(`\{\{.+\}\}|\{%.+%\}`)

// ProcessPath replaces placeholders in a path with variable values.
// Supports any valid Jinja2 expression including method calls.
// Examples: {{ vars.name }}, {{ vars.name | snake }}, {{ cookiecutter.name.lower() }}
func (p *DefaultPathProcessor) ProcessPath(path string, vars map[string]any) (string, error) {
	// Split path into segments to process each part
	segments := strings.Split(path, string(filepath.Separator))
	processedSegments := make([]string, 0, len(segments))

	for i, segment := range segments {
		processed, err := p.processSegment(segment, vars)
		if err != nil {
			return "", NewPathError(path, "failed to process segment", err)
		}

		// If the last segment renders to empty, return empty path to signal
		// the entry should be skipped (conditional exclusion).
		// Example: {% if vars.feature %}file.go{% endif %} with feature=false
		if i == len(segments)-1 && processed == "" {
			return "", nil
		}

		// Skip empty intermediate segments (from placeholders resolving to empty string)
		if processed != "" {
			processedSegments = append(processedSegments, processed)
		}
	}

	if len(processedSegments) == 0 {
		return "", nil
	}

	return filepath.Join(processedSegments...), nil
}

// maxRenderIterations limits recursive template rendering to prevent infinite loops.
const maxRenderIterations = 5

// processSegment processes Jinja2 expressions in a single path segment.
// Handles nested templates (when a variable's value contains another template expression)
// by re-rendering until no more placeholders remain.
func (p *DefaultPathProcessor) processSegment(segment string, vars map[string]any) (string, error) {
	// Quick check: if no {{ }} present, return as-is
	if !placeholderDetectRegex.MatchString(segment) {
		return segment, nil
	}

	// Build context with both "vars" and "cookiecutter" namespaces
	ctx := template.Context{
		"vars":         vars,
		"cookiecutter": vars, // Alias for compatibility
	}

	result := segment
	for i := 0; i < maxRenderIterations; i++ {
		// If no more placeholders, we're done
		if !placeholderDetectRegex.MatchString(result) {
			break
		}

		// Render the current result
		rendered, err := p.engine.ExecuteToString(result, ctx)
		if err != nil {
			return "", fmt.Errorf("failed to process path segment %q: %w", segment, err)
		}
		rendered = strings.TrimSpace(rendered)

		// If rendering didn't change anything, we're done (prevents infinite loops)
		if rendered == result {
			break
		}
		result = rendered
	}

	return result, nil
}

// simpleVarRegex extracts simple variable names from {{ vars.name }} or {{ cookiecutter.name }} patterns.
// This is used for ExtractPlaceholders - complex expressions are not fully parsed.
var simpleVarRegex = regexp.MustCompile(`\{\{\s*(?:vars|cookiecutter)\.([a-zA-Z_][a-zA-Z0-9_]*)`)

// ExtractPlaceholders returns variable names found in simple {{ vars.name }} patterns.
// Note: This does not extract variables from complex expressions like method calls.
func ExtractPlaceholders(path string) []string {
	matches := simpleVarRegex.FindAllStringSubmatch(path, -1)
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

// HasPlaceholders checks if a path contains any Jinja2-style placeholders.
func HasPlaceholders(path string) bool {
	return placeholderDetectRegex.MatchString(path)
}

