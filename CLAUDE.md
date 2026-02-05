# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

TAG is a Go-based code generation CLI tool that uses Go's `text/template` engine to generate files from templates. It supports creating new files, appending to existing files, and injecting content before/after specific markers.

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

## Architecture

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

## Key Concepts

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
