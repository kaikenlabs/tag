# CLAUDE.md

## Mandatory Rules

1. Before pushing: `make lint` then `make test`
2. Prefer **Serena MCP** for symbol-level code operations:
   - `get_symbols_overview` / `find_symbol` over full file reads when navigating
   - `replace_symbol_body` / `insert_after_symbol` / `insert_before_symbol` for symbol edits
   - `find_referencing_symbols` for impact analysis before changes
   - Edit/Write are fine for new files, non-code files, and non-symbol edits (imports, comments, formatting)
3. Use **sequential thinking MCP** for complex reasoning
4. Use **ref MCP** for library documentation lookups
5. After code changes, update `.skills/` files if CLI commands, flags, behavior, or workflows changed
6. ALWAYS follow the instructions in the global $HOME/.claude/CLAUDE.md

## Project Overview

Go CLI for template-driven code generation and project scaffolding. Uses Gonja (Jinja2-compatible) templates with `{{ vars.* }}` namespace.

For command reference and template authoring details, see `.skills/SKILL.md` and `.skills/reference.md`.

## Commands

```bash
# Build
make build                              # Build binary to ./tag
make install                            # Build and install to ~/.local/bin

# Test
go test ./...                           # All tests
go test -v ./internal/scaffold/...      # Specific package
go test -run TestUT_CommandError ./...   # Single test
make test-unit                          # Unit tests with coverage
make test-integration                   # Integration tests (needs make build)

# Lint & Format
make lint                               # Full lint (golangci-lint v2, all linters enabled)
make fmt                                # Format (gofumpt + goimports)
make scan                               # Security scan (gosec + govulncheck)
make tools                              # Install dev tools
```

## Architecture

```
main.go                         CLI entry point (urfave/cli/v2)
├── internal/commands/          Command handlers (generate, scaffold, convert, library, etc.)
├── internal/scaffold/          Scaffold orchestration (phased via runContext)
├── internal/hooks/             Hook execution, env building, interpreter resolution
├── internal/tmplconfig/        Shared template config types (TemplateConfig, VariableDef)
├── internal/vars/              Variable reference scanning, usage analysis, rename-var (ScanRefs/ScanNames shared by lint)
├── internal/lint/              Template linting (schema, syntax, variable cross-reference; depends on internal/vars)
├── internal/convert/           Cookiecutter conversion (depends on tmplconfig, not scaffold)
├── internal/template/          Gonja engine wrapper, custom filters, context building
├── internal/remote/            Remote template resolution (git/zip/cache/auth)
├── internal/engine/            Code generation engine + template parsing
├── internal/library/           Template library management
├── internal/templateupdate/    Historical rendering, 3-way merge engine, ignore matcher, conflict resolution
├── internal/parse/             Shared parsing utilities (meta-flag parser)
├── internal/config/            Config file loading and validation
├── internal/validate/          Input name validation
├── internal/formats/           Case conversions (snake, pascal, camel, etc.)
├── internal/fileutil/          Copy, symlink checks, path helpers
├── internal/replay/            Replay system for saved inputs
├── internal/schema/            JSON Schema validation
├── internal/chalk/             Terminal color/styling
├── internal/writer/            File writing and code injection
├── internal/xdg/               XDG base directory support
├── pkg/app/                    Error handling (CommandError, Errorf)
└── pkg/prettylog/              Custom slog handler
```

Clean DAG — no dependency cycles. `commands/` has widest fan-out.

## Conventions

### Error Handling
```go
if err != nil {
    return app.Errorf("cannot open file: %w", err)
}
```
Errors bubble up via `*CommandError` to `main.go` for centralized logging and exit codes.

### Testing
- **Assertions**: testify (`assert`/`require`)
- **Style**: Table-driven tests preferred
- **Naming**: `TestUT_*` (unit), `TestIT_*` (integration)
- **Unit tests**: Mock interfaces (e.g., `MockPrompter`), no real filesystem/TTY
- **Integration tests**: `t.TempDir()` for isolation, live in `internal/integration/`
- **Gotcha**: Tests using `setupFakeLibrary` must NOT use `t.Parallel()` (mutates package-level var)
- **HTTP tests**: `httptest.NewServer` with parameterized base URLs

### Linting
- Config: `.golangci.yaml` — `default: all` with ~30 disabled linters
- Key limits: `cyclop` max 20, `funlen` max 100 lines / 60 statements, `gocognit` max 30
- Imports: `goimports` with local prefix `github.com/kaikenlabs/tag`
- Test files get relaxed rules (no `dupl`, `gosec`, `funlen`, `cyclop`, etc.)

### CI
PR checks (`.github/workflows/ci.yml`): golangci-lint v2.10 + `go vet` + unit tests + integration tests. Go version read from `go.mod` (currently 1.25.6).
