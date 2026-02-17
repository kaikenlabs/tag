package engine

import (
	"github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/types"
)

// TemplateParser wraps the Gonja template engine for parsing TAG templates.
type TemplateParser struct {
	gonjaEngine     template.TemplateExecutor
	templates       map[string]string
	sharedTemplates map[string]string
}

// TemplateData represents the parsed and rendered result for a single template.
type TemplateData struct {
	Name   string // Template name/path
	To     string // Output file path
	Output []byte // Rendered template content
	ParseData
}

// ParseData holds the parsing configuration and user input.
type ParseData struct {
	Action        template.Action    // File operation: Create, Append, or Inject
	InjectClause  types.InjectClause // Before or After (for inject action)
	InjectMatcher string
	Meta          map[string]string // User-provided metadata from --meta flags
	Notes         string
}

// InputData represents the input provided by the engine for template parsing.
type InputData struct {
	Name         string            // Primary name value
	Args         string            // Free-form arguments
	Meta         map[string]string // Key-value metadata from --meta flags
	ScaffoldVars map[string]any    // Variables from scaffold-time .tagconfig.json
}
