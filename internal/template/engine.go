package template

import (
	"fmt"

	"github.com/nikolalohinski/gonja/v2"
	"github.com/nikolalohinski/gonja/v2/builtins"
	"github.com/nikolalohinski/gonja/v2/config"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/nikolalohinski/gonja/v2/loaders"
)

// Engine wraps Gonja to provide a consistent template interface.
type Engine struct {
	config  *config.Config
	env     *exec.Environment
	strict  bool
	baseDir string
	loader  loaders.Loader
}

// gonjaTemplate wraps a Gonja template to implement our Template interface.
type gonjaTemplate struct {
	tmpl   *exec.Template
	name   string
	engine *Engine
}

// NewEngine creates a new template engine with the given options.
// By default, the engine uses strict undefined variable handling.
// Returns an error if the engine cannot be initialized.
func NewEngine(opts ...Option) (*Engine, error) {
	e := &Engine{
		strict: true, // Default to strict mode
	}

	// Apply options
	for _, opt := range opts {
		opt(e)
	}

	// Create configuration and environment
	e.config = config.New()

	var err error
	e.env, err = e.createEnvironment()
	if err != nil {
		return nil, err
	}

	return e, nil
}

// MustNewEngine creates a new template engine and panics on error.
// Use this only when you're certain initialization will succeed.
func MustNewEngine(opts ...Option) *Engine {
	e, err := NewEngine(opts...)
	if err != nil {
		panic(fmt.Sprintf("failed to create template engine: %v", err))
	}
	return e
}

// createEnvironment creates a Gonja environment with our custom configuration.
func (e *Engine) createEnvironment() (*exec.Environment, error) {
	// Create custom methods with our modifications (e.g., replace with optional count)
	customMethods := builtins.Methods
	customMethods.Str = createCustomStringMethods()

	// Use builtins directly - they're already initialized properly
	env := &exec.Environment{
		Context:           exec.EmptyContext().Update(builtins.GlobalFunctions),
		Filters:           builtins.Filters,
		Tests:             builtins.Tests,
		ControlStructures: builtins.ControlStructures,
		Methods:           customMethods,
	}

	// Register our custom filters on top of builtins
	if err := RegisterFilters(env.Filters); err != nil {
		return nil, fmt.Errorf("failed to create environment: %w", err)
	}

	return env, nil
}

// ParseString parses a template from a string.
func (e *Engine) ParseString(content string) (Template, error) {
	return e.parseWithName(content, "<string>")
}

// ParseStringNamed parses a template from a string with a given name.
// The name is used for error reporting.
func (e *Engine) ParseStringNamed(content, name string) (Template, error) {
	return e.parseWithName(content, name)
}

// parseWithName parses a template with a given name for error reporting.
func (e *Engine) parseWithName(content, name string) (Template, error) {
	// Memory loader requires keys to start with '/'
	loaderKey := name
	if len(loaderKey) == 0 || loaderKey[0] != '/' {
		loaderKey = "/" + loaderKey
	}

	// Create a memory loader with the content
	loader := loaders.MustNewMemoryLoader(map[string]string{
		loaderKey: content,
	})

	// Create the template using the low-level API
	tmpl, err := exec.NewTemplate(loaderKey, e.config, loader, e.env)
	if err != nil {
		return nil, NewParseError(name, 0, 0, err)
	}

	return &gonjaTemplate{
		tmpl:   tmpl,
		name:   name,
		engine: e,
	}, nil
}

// ParseFile parses a template from a file.
func (e *Engine) ParseFile(path string) (Template, error) {
	var loader loaders.Loader
	var err error

	if e.loader != nil {
		loader = e.loader
	} else if e.baseDir != "" {
		loader, err = CreateFileSystemLoader(e.baseDir)
		if err != nil {
			return nil, NewParseError(path, 0, 0, err)
		}
	} else {
		// Use the default loader from gonja
		loader = gonja.DefaultLoader
	}

	tmpl, err := exec.NewTemplate(path, e.config, loader, e.env)
	if err != nil {
		return nil, NewParseError(path, 0, 0, err)
	}

	return &gonjaTemplate{
		tmpl:   tmpl,
		name:   path,
		engine: e,
	}, nil
}

// SetLoader sets a custom loader for template resolution.
func (e *Engine) SetLoader(loader loaders.Loader) {
	e.loader = loader
}

// Environment returns the underlying Gonja environment.
// This can be used for advanced customization.
func (e *Engine) Environment() *exec.Environment {
	return e.env
}

// Execute renders the template with the given context.
func (t *gonjaTemplate) Execute(ctx Context) (string, error) {
	// Convert our Context to Gonja's context
	gonjaCtx := exec.NewContext(ctx)

	// Execute the template to string
	result, err := t.tmpl.ExecuteToString(gonjaCtx)
	if err != nil {
		return "", NewExecuteError(t.name, err)
	}

	return result, nil
}

// ExecuteToString is a convenience method that parses and executes a template.
func (e *Engine) ExecuteToString(content string, ctx Context) (string, error) {
	tmpl, err := e.ParseString(content)
	if err != nil {
		return "", err
	}
	return tmpl.Execute(ctx)
}

// MustParseString parses a template from a string and panics on error.
// This is useful for templates that are known to be valid at compile time.
func (e *Engine) MustParseString(content string) Template {
	tmpl, err := e.ParseString(content)
	if err != nil {
		panic(fmt.Sprintf("template parse error: %v", err))
	}
	return tmpl
}

// Compile-time interface checks
var (
	_ Template         = (*gonjaTemplate)(nil)
	_ TemplateExecutor = (*Engine)(nil)
)
