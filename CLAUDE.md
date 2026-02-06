# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

TAG is a Go-based code generation CLI tool with two main capabilities:

1. **Project Scaffolding** (`tag scaffold`) - Create new projects from local or remote templates
2. **Code Generation** (`tag generate`) - Generate code files from templates within existing projects

TAG uses Gonja (Jinja2-compatible) templates and supports Cookiecutter template conversion.

## Key Features

- **Template engine**: Gonja (Jinja2-compatible)
- **Commands**:
  - `tag scaffold <template>` - Project scaffolding from local/remote templates
  - `tag convert cookiecutter` - Migration tool for Cookiecutter templates
  - `tag generate <template>` - Generate code files in existing projects
- **Remote templates**: Support for `gh:`, `gl:`, `bb:`, Git URLs, and zip files
- **Variable namespace**: `{{ vars.* }}` (aliased as `{{ cookiecutter.* }}` for compatibility)
- **Path placeholders**: `{{ vars.name }}` and `{{ vars.name | filter }}` syntax in file/directory names
- **Template config**: `tag.template.json` with JSON Schema validation
- **Replay system**: Auto-save inputs for reproducible scaffolding
- **Hooks**: Pre and post scaffold command execution
- **Cookiecutter auto-detection**: Automatic conversion when scaffolding Cookiecutter templates

## Template Syntax

```jinja2
{{ vars.project_name | snake }}
{% if vars.use_docker %}...{% endif %}
{% for item in vars.features %}...{% endfor %}
```

### Derived Variables

Variables with template expressions as defaults (e.g., `"{{ vars.name | lower }}"`) are **not prompted** during scaffolding. They are computed from other variables, following Cookiecutter behavior.

```json
{
  "vars": {
    "display_name": "My Package",
    "package_name": "{{ vars.display_name | lower | replace(' ', '_') }}"
  }
}
```
Only `display_name` is prompted; `package_name` is computed automatically.

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
go test -v ./internal/scaffold/...      # Run tests for specific package
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

## Architecture

```
main.go                     CLI entry point (urfave/cli/v2)
    │
    ├── internal/commands/  Command handlers
    │       ├── scaffold.go     - Project scaffolding command
    │       ├── convert.go      - Cookiecutter conversion command
    │       └── generate.go     - Code generation command
    │
    ├── internal/scaffold/  Scaffold orchestration
    │       ├── scaffold.go     - Main scaffolding logic
    │       ├── variables.go    - Variable collection and prompting
    │       ├── types.go        - TemplateConfig, VariableDef, Options
    │       ├── processor.go    - Path placeholder processing
    │       ├── hooks.go        - Pre/post scaffold hook execution
    │       └── writer.go       - File output operations
    │
    ├── internal/convert/   Cookiecutter conversion
    │       ├── cookiecutter.go - Converter orchestration
    │       ├── variables.go    - Variable conversion
    │       └── paths.go        - Path placeholder conversion
    │
    ├── internal/template/  Gonja template engine wrapper
    │       ├── engine.go       - Template execution
    │       ├── filters.go      - Custom filters (snake, pascal, etc.)
    │       └── methods.go      - Custom string methods (replace, etc.)
    │
    ├── internal/remote/    Remote template resolution
    │       └── resolver.go     - GitHub, GitLab, Bitbucket, Git URLs
    │
    ├── internal/replay/    Replay system for saved inputs
    │
    ├── internal/schema/    JSON Schema validation
    │
    └── pkg/app/            Shared utilities
            └── errors.go   CommandError type with Errorf() helper
```

## Key Concepts

### Variable Collection Priority
Variables are resolved in this order (highest priority first):
1. `--meta` flag values
2. `--values` file
3. `--replay` saved values
4. Interactive prompts (if TTY)
5. Default values from `tag.template.json`

### Variable Types
- **Regular variables**: Prompted during scaffolding
- **Private variables** (prefix `_`): Not prompted, for internal use
- **Derived variables**: Default contains `{{ vars.* }}`, not prompted, computed from other vars

### Path Processing
Paths with `{{ vars.name }}` or `{{ vars.name | filter }}` are processed using Gonja. Supports:
- Simple substitution: `{{ vars.project_name }}`
- Filters: `{{ vars.name | snake }}`
- Method calls: `{{ vars.name.lower().replace(' ', '_') }}`
- Nested templates (recursive rendering for derived variables)

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
- Test naming: `TestUT_*` for unit tests, `TestIT_*` for integration tests
- Unit tests: Use mock interfaces (e.g., `MockPrompter`) to avoid real filesystem/TTY
- Integration tests: Use `t.TempDir()` for isolated filesystem testing with auto-cleanup

## Documentation

- `docs/` - User-facing documentation
  - `docs/commands/` - Command reference
  - `docs/templates/` - Template authoring guides
  - `docs/reference/` - Configuration reference
- `claudedocs/tag_scaffold_specification.md` - Technical specification
- `claudedocs/research_cookiecutter_vs_tag.md` - Cookiecutter comparison
