package lint

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/kaikenlabs/tag/internal/schema"
	"github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/tmplconfig"
	"github.com/kaikenlabs/tag/internal/types"
)

// Linter validates a TAG template directory for correctness.
type Linter struct {
	root      string
	engine    *template.Engine
	validator *schema.Validator
	config    *tmplconfig.TemplateConfig
	vars      map[string]struct{}
	result    *Result
}

// NewLinter creates a new linter for the given template directory.
func NewLinter(root string) (*Linter, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("path does not exist: %s", root)
		}
		return nil, fmt.Errorf("stat path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", root)
	}

	configPath := filepath.Join(absRoot, types.TemplateConfigFile)
	if _, statErr := os.Stat(configPath); errors.Is(statErr, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s not found in %s", types.TemplateConfigFile, root)
	}

	engine, err := template.NewEngine()
	if err != nil {
		return nil, fmt.Errorf("create template engine: %w", err)
	}

	validator, err := schema.NewValidator()
	if err != nil {
		return nil, fmt.Errorf("create schema validator: %w", err)
	}

	return &Linter{
		root:      absRoot,
		engine:    engine,
		validator: validator,
		vars:      make(map[string]struct{}),
		result:    &Result{},
	}, nil
}

// Run executes all lint checks and returns the result.
func (l *Linter) Run() (*Result, error) {
	// Phase 1: Validate and parse tag.template.json
	l.lintSchema()

	// If config failed to parse, we can still try template syntax checks
	// but cannot do variable cross-referencing.
	if l.config != nil {
		for name := range l.config.Vars {
			l.vars[name] = struct{}{}
		}
		// Lint derived variable defaults for undefined references
		l.lintDerivedDefaults()
	}

	// Phase 2: Check generator/bundle names for reserved subcommand conflicts
	l.lintGeneratorNames()

	// Phase 3: Walk and lint template files
	if err := l.lintTemplateFiles(); err != nil {
		return nil, fmt.Errorf("walk template files: %w", err)
	}

	return l.result, nil
}
