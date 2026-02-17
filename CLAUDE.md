# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Mandatory

1. Before pushing code, run the linter with `make lint`
2. Before pushing code, run the tests with `make test`
3. Use serena MCP as much as possible for code operations.
4. Use sequential thinking MCP for complexe thinking operations.
5. Use ref MCP server to look for documentation on well known libraries.

## Project Overview

TAG is a Go-based CLI tool for template-driven code generation and project scaffolding.

**Commands**:
- `tag init` - Initialize tag directory structure (`.tag/_shared`, `.tag/_bundles`)
- `tag new <name>` - Create a new generator template (`--lib` to target the library template from `.tagconfig.json`)
- `tag new-bundle <name>` (alias: `nb`) - Create a new bundle (`--lib` to target the library template from `.tagconfig.json`)
- `tag generate <bundle-or-generator> <name>` - Run generators/bundles within existing projects
- `tag scaffold <template> [project-name]` - Create new projects from local or remote templates
- `tag convert cookiecutter <source>` - Convert Cookiecutter templates to TAG format

**Key Features**:
- Template engine: Gonja (Jinja2-compatible)
- Remote templates: `gh:`, `gl:`, `bb:` shorthands, Git URLs, zip files
- Variable namespace: `{{ vars.* }}`
- Path placeholders: `{{ vars.name }}` and `{{ vars.name | filter }}` in file/directory names
- Template config: `tag.template.json` with JSON Schema validation
- Replay system: Auto-save inputs for reproducible scaffolding
- Hooks: Pre and post scaffold command execution
- Cookiecutter support: Auto-detection and conversion when scaffolding Cookiecutter templates

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
make install            # Build and install to ~/.local/bin
go build -o tag         # Alternative direct build
./tag --help            # Show CLI help
```

### Testing
```bash
go test ./...                           # Run all tests
go test -v ./internal/scaffold/...      # Run tests for specific package
go test -run TestUT_CommandError ./...   # Run single test by name
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
main.go                         CLI entry point (urfave/cli/v2)
    │
    ├── internal/commands/      Command handlers
    │       ├── init.go             - Initialize tag directory structure
    │       ├── new.go              - Create new generator template
    │       ├── bundle.go           - Create new bundle
    │       ├── generate.go         - Code generation command + execution
    │       ├── generate_list.go    - Generator/bundle discovery and display
    │       ├── generate_resolve.go - Generator/bundle path resolution
    │       ├── scaffold.go         - Project scaffolding command
    │       ├── convert.go          - Cookiecutter conversion command
    │       ├── library.go          - Template library commands
    │       ├── run.go              - Run command (template-bundled generators)
    │       ├── flags.go            - Shared CLI flag definitions
    │       └── validate.go         - Input validation helpers
    │
    ├── internal/scaffold/      Scaffold orchestration
    │       ├── scaffold.go         - Main scaffolding logic (phased via runContext)
    │       ├── variables.go        - Variable collection and prompting
    │       ├── types.go            - TemplateConfig, VariableDef, Options
    │       ├── processor.go        - Path placeholder processing (Gonja)
    │       ├── prompt.go           - Interactive prompting utilities
    │       ├── output.go           - Output directory handling
    │       ├── cookiecutter_detect.go - Cookiecutter template detection
    │       └── errors.go           - Custom error types
    │
    ├── internal/hooks/         Hook execution (extracted from scaffold)
    │       ├── hooks.go            - Runner interface, confirmation, execution
    │       ├── hookenv.go          - Hook environment variable building
    │       ├── interpreter.go      - Script interpreter resolution (shebang, extension)
    │       └── errors.go           - HookError type and sentinels
    │
    ├── internal/convert/       Cookiecutter conversion
    │       ├── cookiecutter.go     - Converter orchestration
    │       ├── variables.go        - Variable conversion
    │       ├── paths.go            - Path placeholder conversion
    │       ├── content.go          - Content analysis for compatibility
    │       ├── hooks.go            - Hook detection and copying
    │       ├── types.go            - Options, Result, Incompatibility types
    │       └── errors.go           - Custom error types
    │
    ├── internal/template/      Gonja template engine wrapper
    │       ├── engine.go           - Template execution
    │       ├── filters.go          - Custom filters (snake, pascal, etc.)
    │       ├── methods.go          - Custom string methods (replace, etc.)
    │       ├── loader.go           - Template file loader
    │       ├── context.go          - Template context building
    │       ├── metadata.go         - Template metadata handling
    │       ├── interfaces.go       - TemplateRenderer interface
    │       ├── errors.go           - Error types and sentinels
    │       └── types.go            - Type definitions
    │
    ├── internal/remote/        Remote template resolution
    │       ├── remote.go           - Resolver orchestration
    │       ├── reference.go        - Reference parsing (gh:, gl:, bb:, URLs)
    │       ├── git.go              - Git cloning and checkout
    │       ├── zip.go              - ZIP download and extraction
    │       ├── cache.go            - Template caching system
    │       ├── auth.go             - Auth provider (Bearer for Bitbucket, Basic for GitHub/GitLab)
    │       └── errors.go           - Custom error types with provider hints
    │
    ├── internal/engine/        Code generation engine + template parsing
    │       ├── engine.go           - Generator execution
    │       ├── parser.go           - Template parsing logic (merged from parser/)
    │       ├── parser_types.go     - Parser type definitions
    │       ├── types.go            - Generator bundle types
    │       └── interfaces.go       - Generator interface definitions
    │
    ├── internal/library/       Template library management
    │       ├── library.go          - Library operations (install, list, resolve)
    │       ├── registry.go         - Registry file management
    │       ├── types.go            - Library entry types
    │       └── errors.go           - Custom error types
    │
    ├── internal/parse/         Shared parsing utilities
    │       └── meta.go             - Meta-flag key=value parser (used by commands + scaffold)
    │
    ├── internal/config/        Configuration management
    │       ├── config.go           - Config file loading
    │       └── validate.go         - Config validation
    │
    ├── internal/formats/       String formatting utilities
    │       └── cases.go            - Case conversions (snake, pascal, camel, etc.)
    │
    ├── internal/replay/        Replay system for saved inputs
    ├── internal/schema/        JSON Schema validation
    ├── internal/chalk/         Terminal color/styling
    ├── internal/types/         Type definitions and flag constants
    ├── internal/writer/        File writing and code injection
    ├── internal/xdg/           XDG base directory support
    │
    ├── pkg/app/                Error handling (CommandError, Errorf)
    └── pkg/prettylog/          Custom slog handler with colored output
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

### Generator File Processing
Generator directories (used with `tag generate`) load **all files** in the directory regardless of extension. Files keep their natural extension (e.g., `.go`, `.ts`, `.py`). Each file contains YAML frontmatter (between `---` delimiters) specifying the action (create/inject/append) and destination path, followed by the template body.

### Path Processing
Paths with `{{ vars.name }}` or `{{ vars.name | filter }}` are processed using Gonja. Supports:
- Simple substitution: `{{ vars.project_name }}`
- Filters: `{{ vars.name | snake }}`
- Method calls: `{{ vars.name.lower().replace(' ', '_') }}`
- Nested templates (recursive rendering for derived variables)

### Remote Authentication
- **GitHub**: `GITHUB_TOKEN` env var (basic auth with `x-access-token`)
- **GitLab**: `GITLAB_TOKEN` env var (basic auth with `x-access-token`)
- **Bitbucket**: `BITBUCKET_TOKEN` env var (Bearer token auth)
- **SSH**: SSH agent or default key paths

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
  - `docs/commands/` - Command reference (scaffold, generate, convert)
  - `docs/templates/` - Template authoring guides (syntax, filters, hooks)
  - `docs/reference/` - Configuration reference (tag.template.json, remote-refs)
  - `docs/migration/` - Migration guide (v1-to-v2)
  - `docs/getting-started.md` - Getting started guide
