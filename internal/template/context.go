package template

import (
	"strings"

	"github.com/kaikenlabs/tag/internal/formats"
)

// computeNameOptions creates name variant options from a name string.
func computeNameOptions(name string) map[string]any {
	return map[string]any{
		"snake_case":  formats.CaseSnake(name),
		"pascal_case": formats.CasePascal(name),
		"camel_case":  formats.CaseCamel(name),
		"kebab_case":  formats.CaseKebab(name),
		"lower_case":  strings.ToLower(name),
		"upper_case":  strings.ToUpper(name),
	}
}

// ContextBuilder provides a fluent API for constructing template contexts.
// It supports both scaffold and generate context shapes.
type ContextBuilder struct {
	ctx Context
}

// NewContextBuilder creates a new ContextBuilder.
func NewContextBuilder() *ContextBuilder {
	return &ContextBuilder{ctx: make(Context)}
}

// WithVars adds the "vars" namespace.
func (b *ContextBuilder) WithVars(vars map[string]any) *ContextBuilder {
	if vars == nil {
		vars = make(map[string]any)
	}
	b.ctx["vars"] = vars
	return b
}

// WithName adds the "name" key and "n" namespace with pre-computed name variants.
func (b *ContextBuilder) WithName(name string) *ContextBuilder {
	b.ctx["name"] = name
	b.ctx["n"] = computeNameOptions(name)
	return b
}

// Build returns the constructed context.
func (b *ContextBuilder) Build() Context {
	return b.ctx
}
