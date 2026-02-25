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

// SSTIConfigurable is implemented by path processors that support
// derived-variable tracking for SSTI protection.
type SSTIConfigurable interface {
	SetDerivedVarNames(names map[string]bool)
}

// DefaultPathProcessor implements PathProcessor using the Gonja template engine.
type DefaultPathProcessor struct {
	engine               template.TemplateRenderer
	allowRecursiveRender bool
	derivedVarNames      map[string]bool // Variables whose defaults are template expressions
}

// NewPathProcessor creates a new path processor with the given template renderer.
func NewPathProcessor(engine template.TemplateRenderer) *DefaultPathProcessor {
	return &DefaultPathProcessor{engine: engine}
}

// SetAllowRecursiveRender controls whether user-provided variable values containing
// template syntax are rendered. When false (default), template delimiters in
// non-derived variable values are escaped to prevent SSTI attacks. Derived variables
// (whose defaults are template expressions) are always rendered regardless.
func (p *DefaultPathProcessor) SetAllowRecursiveRender(allow bool) {
	p.allowRecursiveRender = allow
}

// SetDerivedVarNames sets the list of derived variable names. These variables
// have template expressions as defaults and need recursive rendering to resolve.
func (p *DefaultPathProcessor) SetDerivedVarNames(names map[string]bool) {
	p.derivedVarNames = names
}

// placeholderDetectRegex detects if a string contains Jinja2-style template syntax.
// Matches both {{ expressions }} and {% statements %} (e.g., conditionals).
// This is a simple check - the actual parsing is done by Gonja.
var placeholderDetectRegex = regexp.MustCompile(`\{\{.+\}\}|\{%.+%\}`)

// ProcessPath replaces placeholders in a path with variable values.
// Supports any valid Jinja2 expression including method calls.
// Examples: {{ vars.name }}, {{ vars.name | snake }}, {{ vars.name.lower() }}
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
//
// When allowRecursiveRender is false, template delimiters in non-derived variable
// values are escaped to prevent Server-Side Template Injection (SSTI).
func (p *DefaultPathProcessor) processSegment(segment string, vars map[string]any) (string, error) {
	// Quick check: if no {{ }} present, return as-is
	if !placeholderDetectRegex.MatchString(segment) {
		return segment, nil
	}

	// Build safe vars: escape template syntax in non-derived variable values
	// when recursive render is disabled (default)
	safeVars := vars
	if !p.allowRecursiveRender {
		safeVars = p.escapeNonDerivedVars(vars)
	}

	ctx := template.Context{
		"vars": safeVars,
	}

	result := segment
	for range maxRenderIterations {
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

	// Restore escaped template delimiters to their original form so that
	// user-provided values containing {{ }} appear literally in paths.
	if !p.allowRecursiveRender {
		result = unescapeTemplateSyntax(result)
	}

	return result, nil
}

// escapeNonDerivedVars returns a copy of vars where template delimiters in
// non-derived string values are escaped with sentinel tokens. This prevents
// user-provided values containing {{ }}, {% %}, or {# #} from being
// interpreted as template code during rendering.
func (p *DefaultPathProcessor) escapeNonDerivedVars(vars map[string]any) map[string]any {
	safe := make(map[string]any, len(vars))
	for k, v := range vars {
		if p.derivedVarNames[k] {
			// Derived variables retain their template expressions
			safe[k] = v
			continue
		}
		if s, ok := v.(string); ok {
			safe[k] = escapeTemplateSyntax(s)
		} else {
			safe[k] = v
		}
	}
	return safe
}

// Sentinel tokens used to escape template delimiters in user-provided values.
const (
	sentinelOpenExpr     = "\x00TAG_LBRACE2\x00"
	sentinelCloseExpr    = "\x00TAG_RBRACE2\x00"
	sentinelOpenStmt     = "\x00TAG_LBRACE_PCT\x00"
	sentinelCloseStmt    = "\x00TAG_PCT_RBRACE\x00"
	sentinelOpenComment  = "\x00TAG_LBRACE_HASH\x00"
	sentinelCloseComment = "\x00TAG_HASH_RBRACE\x00"
)

// escapeTemplateSyntax replaces Jinja2 template delimiters in a string with
// sentinel tokens that won't be interpreted by the template engine.
func escapeTemplateSyntax(s string) string {
	if !strings.Contains(s, "{{") && !strings.Contains(s, "{%") && !strings.Contains(s, "{#") {
		return s
	}
	s = strings.ReplaceAll(s, "{{", sentinelOpenExpr)
	s = strings.ReplaceAll(s, "}}", sentinelCloseExpr)
	s = strings.ReplaceAll(s, "{%", sentinelOpenStmt)
	s = strings.ReplaceAll(s, "%}", sentinelCloseStmt)
	s = strings.ReplaceAll(s, "{#", sentinelOpenComment)
	s = strings.ReplaceAll(s, "#}", sentinelCloseComment)
	return s
}

// unescapeTemplateSyntax reverses escapeTemplateSyntax, restoring sentinel
// tokens back to their original Jinja2 template delimiters.
func unescapeTemplateSyntax(s string) string {
	if !strings.Contains(s, "\x00") {
		return s
	}
	s = strings.ReplaceAll(s, sentinelOpenExpr, "{{")
	s = strings.ReplaceAll(s, sentinelCloseExpr, "}}")
	s = strings.ReplaceAll(s, sentinelOpenStmt, "{%")
	s = strings.ReplaceAll(s, sentinelCloseStmt, "%}")
	s = strings.ReplaceAll(s, sentinelOpenComment, "{#")
	s = strings.ReplaceAll(s, sentinelCloseComment, "#}")
	return s
}
