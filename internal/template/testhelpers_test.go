package template

import (
	"fmt"
)

// NewContext creates a new template context with the standard namespaces.
// This is a test helper used across many test files in this package.
func NewContext(name string, vars map[string]any) Context {
	return NewContextBuilder().
		WithName(name).
		WithVars(vars).
		Build()
}

// MustNewEngine creates a new template engine and panics on error.
// This is a test helper — use NewEngine in production code.
func MustNewEngine(opts ...Option) *Engine {
	e, err := NewEngine(opts...)
	if err != nil {
		panic(fmt.Sprintf("failed to create template engine: %v", err))
	}
	return e
}

// WithBaseDir sets the base directory for template loading.
// This is a test helper for configuring engines in tests.
func WithBaseDir(dir string) Option {
	return func(e *Engine) {
		e.baseDir = dir
	}
}
