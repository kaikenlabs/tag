package parser

import (
	"github.com/kaikenlabs/tag/internal/template"
)

// TemplateEngine wraps the Gonja template engine for parsing TAG templates.
//
// Deprecated: TemplateEngine is part of the legacy generate pipeline.
// New code should use template.TemplateExecutor directly via the scaffold package.
type TemplateEngine struct {
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

// ParseActions represents the type of file operation.
type ParseActions string

const (
	ActionCreate ParseActions = "Create"
	ActionAppend ParseActions = "Append"
	ActionInject ParseActions = "Inject"
)

// InjectClause represents where to inject content.
type InjectClause string

const (
	InjectBefore InjectClause = "Before"
	InjectAfter  InjectClause = "After"
)

// ParseData holds the parsing configuration and user input.
type ParseData struct {
	Action        ParseActions
	InjectClause  InjectClause
	InjectMatcher string
	Args          string            // Free-form arguments string
	N             NameOptions       // Pre-computed name variants (deprecated, use template context)
	Meta          map[string]string // User-provided metadata from --meta flags
	Notes         string
}

// NameOptions contains pre-computed name variants.
// Deprecated: Use template.NameOptions and the "n" context namespace instead.
type NameOptions struct {
	PascalCase string
	CamelCase  string
	SnakeCase  string
	KebabCase  string
	LowerCase  string
	UpperCase  string
}

// InputData represents the input provided by the engine for template parsing.
type InputData struct {
	Name string            // Primary name value
	Args string            // Free-form arguments
	Meta map[string]string // Key-value metadata from --meta flags
}
