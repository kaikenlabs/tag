package template

import (
	"strings"

	"github.com/kaikenlabs/tag/internal/formats"
)

// NewContext creates a new template context with the standard namespaces.
// The context includes:
//   - "name": the primary name value
//   - "vars": user-defined variables (TAG namespace)
//   - "cookiecutter": alias to vars (Cookiecutter compatibility)
//   - "n": pre-computed name variants
func NewContext(name string, vars map[string]any) Context {
	return NewContextBuilder().
		WithName(name).
		WithVars(vars).
		Build()
}

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

// WithVars adds the "vars" and "cookiecutter" namespaces.
// The cookiecutter namespace is an alias pointing to the same map.
func (b *ContextBuilder) WithVars(vars map[string]any) *ContextBuilder {
	if vars == nil {
		vars = make(map[string]any)
	}
	b.ctx["vars"] = vars
	b.ctx["cookiecutter"] = vars
	return b
}

// WithRootVars adds each variable to the root of the context for convenience.
// This allows templates to use {{ project_name }} instead of {{ vars.project_name }}.
func (b *ContextBuilder) WithRootVars(vars map[string]any) *ContextBuilder {
	for k, v := range vars {
		b.ctx[k] = v
	}
	return b
}

// WithName adds the "name" key and "n" namespace with pre-computed name variants.
func (b *ContextBuilder) WithName(name string) *ContextBuilder {
	b.ctx["name"] = name
	b.ctx["n"] = computeNameOptions(name)
	return b
}

// WithMeta adds the "meta" namespace for backward compatibility.
func (b *ContextBuilder) WithMeta(meta map[string]string) *ContextBuilder {
	if meta != nil {
		metaAny := make(map[string]any, len(meta))
		for k, v := range meta {
			metaAny[k] = v
		}
		b.ctx["meta"] = metaAny
	}
	return b
}

// Build returns the constructed context.
func (b *ContextBuilder) Build() Context {
	return b.ctx
}

