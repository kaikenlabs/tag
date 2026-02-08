package template

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/nikolalohinski/gonja/v2"
	"github.com/nikolalohinski/gonja/v2/builtins"
	"github.com/nikolalohinski/gonja/v2/config"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/nikolalohinski/gonja/v2/loaders"
)

// templateCache provides thread-safe caching of parsed Gonja templates.
// Templates are keyed by SHA-256 hash of their content string.
type templateCache struct {
	mu    sync.RWMutex
	items map[string]*exec.Template
}

// get retrieves a cached template by content hash. Returns nil if not found.
func (c *templateCache) get(key string) *exec.Template {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.items[key]
}

// set stores a parsed template in the cache.
func (c *templateCache) set(key string, tmpl *exec.Template) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = tmpl
}

// contentHash computes a SHA-256 hash of the template content for use as a cache key.
func contentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// Engine wraps Gonja to provide a consistent template interface.
type Engine struct {
	config  *config.Config
	env     *exec.Environment
	baseDir string
	loader  loaders.Loader
	cache   *templateCache
}

// gonjaTemplate wraps a Gonja template to implement our Template interface.
type gonjaTemplate struct {
	tmpl   *exec.Template
	name   string
	engine *Engine
}

// NewEngine creates a new template engine with the given options.
// Returns an error if the engine cannot be initialized.
func NewEngine(opts ...Option) (*Engine, error) {
	e := &Engine{
		cache: &templateCache{items: make(map[string]*exec.Template)},
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
// Parsed templates are cached by content hash to avoid re-parsing identical content.
func (e *Engine) parseWithName(content, name string) (Template, error) {
	key := contentHash(content)

	// Fast path: check cache with read lock
	if cached := e.cache.get(key); cached != nil {
		return &gonjaTemplate{
			tmpl:   cached,
			name:   name,
			engine: e,
		}, nil
	}

	// Slow path: parse and cache
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
		// Don't cache parse errors — allow retry with corrected content
		return nil, NewParseError(name, 0, 0, err)
	}

	// Double-check and store: another goroutine may have cached the same
	// content between our read and this write. That's acceptable (idempotent).
	e.cache.set(key, tmpl)

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

// CacheLen returns the number of templates currently in the cache.
// This is intended for testing and diagnostics.
func (e *Engine) CacheLen() int {
	e.cache.mu.RLock()
	defer e.cache.mu.RUnlock()
	return len(e.cache.items)
}

// Compile-time interface checks
var (
	_ Template         = (*gonjaTemplate)(nil)
	_ TemplateExecutor = (*Engine)(nil)
)
