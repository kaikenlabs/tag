// Package template provides a Jinja2-compatible template engine using Gonja.
// It supports aliased namespaces (vars.*, cookiecutter.*) and custom filters
// for case transformation, inflection, and string operations.
package template

// Context represents the template execution context.
// It provides data to templates through multiple namespaces.
type Context map[string]any

// NameOptions contains pre-computed name variants for convenience.
// These are available in templates via the "n" namespace.
type NameOptions struct {
	SnakeCase  string
	PascalCase string
	CamelCase  string
	KebabCase  string
	LowerCase  string
	UpperCase  string
}

// Template represents a parsed template ready for execution.
type Template interface {
	// Execute renders the template with the given context.
	Execute(ctx Context) (string, error)
}

// Option configures the template engine.
type Option func(*Engine)

// WithStrictUndefined configures whether undefined variables cause errors.
// When true (default), accessing undefined variables returns an error.
// When false, undefined variables resolve to empty strings.
//
// Note: This option is currently a placeholder for future implementation.
// Gonja's default behavior is used regardless of this setting.
// TODO: Wire this into Gonja's configuration when supported.
func WithStrictUndefined(strict bool) Option {
	return func(e *Engine) {
		e.strict = strict
	}
}

// WithBaseDir sets the base directory for template loading.
// This is used for resolving relative paths in extends and include directives.
func WithBaseDir(dir string) Option {
	return func(e *Engine) {
		e.baseDir = dir
	}
}
