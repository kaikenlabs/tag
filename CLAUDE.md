# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

TAG is a Go-based code generation CLI tool that supports creating new files, appending to existing files, and injecting content before/after specific markers. It is evolving into a complete project scaffolding solution (similar to Cookiecutter) with remote template support.

## Planned Changes (v2)

See `claudedocs/tag_scaffold_specification.md` for the full specification.

**Key changes:**
- **Template engine**: Gonja (Jinja2-compatible) replacing Go `text/template`
- **New commands**:
  - `tag scaffold <template>` - Project scaffolding from local/remote templates
  - `tag convert cookiecutter` - Migration tool for Cookiecutter templates
- **Remote templates**: Support for `gh:`, `gl:`, `bb:`, Git URLs, and zip files
- **Variable namespace**: `{{ vars.* }}` (aliased as `{{ cookiecutter.* }}` for compatibility)
- **Path placeholders**: `__var__` and `__var | filter__` syntax in file/directory names
- **Template config**: `tag.template.json` with JSON Schema validation
- **Replay system**: Auto-save inputs for reproducible scaffolding

**Jinja2 template syntax (upcoming):**
```jinja2
{{ vars.project_name|snake }}
{% if vars.use_docker %}...{% endif %}
{% for item in vars.features %}...{% endfor %}
```

## Commands

### Build and Run
```bash
make build              # Build binary to ./tag
go build -o tag         # Alternative direct build
./tag --help            # Show CLI help
```

### Testing
```bash
go test ./...                           # Run all tests
go test -v ./pkg/app/...                # Run tests for specific package
go test -run TestUT_CommandError ./...  # Run single test by name
make test-unit                          # Run unit tests with coverage (requires tools)
make test-integration                   # Build and run integration test
```

### Linting
```bash
go vet ./...            # Basic vet check
make lint               # Full lint (requires: make tools first)
make scan               # Security scanning (gosec + govulncheck)
```

### Setup Dev Tools
```bash
make tools              # Install golangci-lint, gofumpt, gotest, gosec, govulncheck
```

## Architecture (Current)

> Note: This reflects the current implementation. See the v2 specification for planned changes.

```
main.go                     CLI entry point (urfave/cli/v2)
    │
    ├── internal/commands/  Command handlers (init, new, new-bundle, generate)
    │       │
    │       └── generate.go orchestrates the generation flow:
    │               1. Load config
    │               2. Create engine
    │               3. Run pre-hooks → generate → run post-hooks
    │
    ├── internal/engine/    Core generation orchestrator
    │       │
    │       └── Core.Generate() coordinates:
    │               parser.Parse() → writer.WriteFile/AppendFile/InjectIntoFile
    │
    ├── internal/parser/    Template loading and parsing
    │       │
    │       ├── TemplateEngine.Parse() - processes templates with Go text/template
    │       ├── lexer.go - extracts metadata block (--- to ---) and template body
    │       └── Template functions: caseSnake, casePascal, pluralise, etc.
    │
    ├── internal/writer/    File output operations
    │       │
    │       ├── WriteFile() - create new files
    │       ├── AppendFile() - append to existing files
    │       └── InjectIntoFile() - insert before/after a marker string
    │
    ├── internal/config/    Config file handling (.tagconfig.json)
    │
    └── pkg/app/            Shared utilities
            └── errors.go   CommandError type with Errorf() helper
```

## Key Concepts (Current)

> Note: Template syntax will change to Jinja2 (Gonja) in v2. See specification for details.

### Template Metadata Block
Templates use a YAML-like header between `---` markers:
```
---
to: path/to/output.go
inject: true
after: // marker
---
Template content here with {{ .Name }}
```

### Template Actions
- `ActionCreate` (default): Write new file
- `ActionAppend` (`append: true`): Append to existing file
- `ActionInject` (`inject: true` + `before:/after:`): Insert at marker

### Template Data Available
- `.Name` - name passed via CLI
- `.Args` - free-form arguments string
- `.Meta` - key-value pairs from `--meta` flag
- `.N.PascalCase`, `.N.SnakeCase`, etc. - pre-formatted name variants

## Error Handling Pattern

Commands return errors using `app.Errorf()` which creates a `*CommandError`. Errors bubble up to `main.go` for centralized logging and exit code handling.

```go
if err != nil {
    return app.Errorf("cannot open file: %w", err)
}
```

## Testing Conventions

- Use testify (`assert`/`require`) for assertions
- Table-driven tests preferred
- Test naming: `TestUT_*` for unit tests
- Unit tests: Use mock interfaces (e.g., `fileReadWriteMock`) to avoid real filesystem
- Integration tests: Use `t.TempDir()` for isolated filesystem testing with auto-cleanup

## Documentation

- `claudedocs/tag_scaffold_specification.md` - Full v2 specification
- `claudedocs/research_cookiecutter_vs_tag.md` - Cookiecutter comparison and analysis
