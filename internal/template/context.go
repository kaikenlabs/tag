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
func NewContext(name string, vars map[string]any, nameOpts *NameOptions) Context {
	ctx := make(Context)

	ctx["name"] = name

	// Set vars namespace (default to empty map if nil)
	if vars == nil {
		vars = make(map[string]any)
	}
	ctx["vars"] = vars

	// cookiecutter is an alias pointing to the same data as vars
	ctx["cookiecutter"] = vars

	// Set pre-computed name options
	if nameOpts != nil {
		ctx["n"] = map[string]any{
			"snake_case":  nameOpts.SnakeCase,
			"pascal_case": nameOpts.PascalCase,
			"camel_case":  nameOpts.CamelCase,
			"kebab_case":  nameOpts.KebabCase,
			"lower_case":  nameOpts.LowerCase,
			"upper_case":  nameOpts.UpperCase,
		}
	} else {
		// Compute name options from the name if not provided
		ctx["n"] = computeNameOptions(name)
	}

	return ctx
}

// NewContextWithMeta creates a context with additional metadata.
// This is useful for backward compatibility with existing TAG templates.
func NewContextWithMeta(name string, vars map[string]any, meta map[string]string, nameOpts *NameOptions) Context {
	ctx := NewContext(name, vars, nameOpts)

	// Add meta as a separate namespace for backward compatibility
	if meta != nil {
		metaAny := make(map[string]any, len(meta))
		for k, v := range meta {
			metaAny[k] = v
		}
		ctx["meta"] = metaAny
	}

	return ctx
}

// computeNameOptions creates NameOptions from a name string.
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

// NewNameOptions creates a NameOptions struct from a name string.
func NewNameOptions(name string) NameOptions {
	return NameOptions{
		SnakeCase:  formats.CaseSnake(name),
		PascalCase: formats.CasePascal(name),
		CamelCase:  formats.CaseCamel(name),
		KebabCase:  formats.CaseKebab(name),
		LowerCase:  strings.ToLower(name),
		UpperCase:  strings.ToUpper(name),
	}
}

// Set adds or updates a value in the context.
func (c Context) Set(key string, value any) {
	c[key] = value
}

// Get retrieves a value from the context.
func (c Context) Get(key string) (any, bool) {
	v, ok := c[key]
	return v, ok
}

// Merge combines another context into this one.
// Values from the other context override existing values.
func (c Context) Merge(other Context) {
	for k, v := range other {
		c[k] = v
	}
}
