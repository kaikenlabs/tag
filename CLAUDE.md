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

## Lessons Learned

- **The three variable-aware commands share ONE reference definition — keep it that way**: `tag template lint`, `tag template variables` and `tag template rename-var` must agree on what counts as a `vars.*` reference. Since #337 they do so structurally: `lint` and `variables` call `vars.ScanRefs`/`vars.ScanNames` (`internal/vars/scan.go`), and all of them are built on the same `scanBlock`/`skipQuoted`/`blockTag`/`isWholeReference` helpers in `internal/vars/rewrite.go`. Before that, two copy-pasted regexes drifted from the rewriter and produced four user-visible defects at once. Any change to what a reference is belongs in `scanBlockRefs` + `rewriteBlock`, never in a caller. `FuzzScanRefsAgreesWithRenameWalker` and `TestIT_TemplateVarScan_ThreeCommandsAgree` enforce the agreement — if you make them fail, fix the walker, don't relax the test.
- **A line-by-line regex scan cannot see a Gonja block that spans lines**: the pre-#337 scanner ran `bufio.Scanner` then matched `\{\{...\}\}` per line, so `{{ vars.a\n ~ vars.b }}` was invisible entirely — not partially wrong, completely unseen. Greedy `[^}]*`/`[^%]*` also meant only the LAST `vars.X` in a block ever matched. If you reach for a regex over template source here, you are reintroducing both bugs; walk the blocks instead.
- **`{% raw %}`: the BODY is literal, the opening TAG is not**: `MaskLiterals` (deleted in #337) blanked the whole raw span including its opener, but `renameInExpressions` rewrites the opener like any other block. The scanner now matches the rewriter — a `vars.*` in `{% raw vars.x %}` IS a reference, while the body is skipped. Pinned in `TestUT_ScanRefs_DeliberateEdgeCases`; don't "fix" it toward the old masking behaviour without also changing rename-var, or the three commands diverge again.
- **The `history.RecordingFileWriter` must never wrap the dry-run writer**: it hashes each file *after* write to record undo state, but the dry-run writer writes nothing, so `HashFile` fails with `hash after write ...` and aborts the whole run after the first file. Fixed in `engine.NewGeneratorWithRecorder` by only wrapping when `!dryRun` (history is persisted only for real runs anyway — see `generate.go`'s `if !dryRun`). If you add a new recording writer, gate it on `!dryRun` too.
- **The `n.*` generator namespace carries case transforms ONLY, and unknown attrs render empty & silent**: `computeNameOptions` (`internal/template/context.go`) provides exactly `snake_case`/`pascal_case`/`camel_case`/`kebab_case`/`lower_case`/`upper_case`/`past`. `n.snake` (no `_case`), `n.plural`, `n.humanize` all render `""` with no error — inflection lives on filters over `name` (`{{ name | plural }}`). The docs (`.skills/SKILL.md`) used to show the wrong `n.snake`/`n.plural` forms; they now match the map. Verify with a `ExecuteToString("{{ n.x }}", ...)` probe before documenting a new `n.` attribute.
- **The scaffold positional `[project-name]` has two decoupled roles**: it *defaults* the `project_name` variable (an explicit `-m project_name=` wins — see `collectVars`, which only sets `Meta["project_name"]` when absent) AND *drives the output dir* when `--output` is unset (see `planOutput`, which prefers `opts.ProjectName` over `vars["project_name"]`). Keep these two paths in sync: changing one without the other reintroduces "positional silently overrode --meta" (the old bug) or "meta override moved the output directory" (the over-correction).
- **`--format` is shared plumbing now — don't hand-roll it**: `formatFlag(allowed...)`, `resolveFormat(c, allowed...)`, `cmdOut(c)` and `unknownFlagError` live in `internal/commands/flags.go`; the encoder policy lives in `internal/jsonout.Write`. A new command that emits JSON uses those, and follows the envelope rule written down in `jsonout`'s package doc: a LIST wraps under a noun key (`{"entries":[...]}`), a single object or report goes bare. There is deliberately no `{"ok":...,"data":...}` wrapper and no `schema_version` — four commands shipped bare objects before the convention existed. `resolveFormat` falls back to text only when `!c.IsSet("format")`, so hand-built test contexts keep working while `--format=""` is still rejected.
- **Order inside a command Action is fixed: reparse → check positional count → resolveFormat → do the work → write → return the exit code**: `reparseTrailingFlags` must run first or a trailing `--format` is invisible, and after it you must use the returned slice — `c.Args()`/`c.NArg()` are stale because the reparser does not mutate the context. The exit code goes last so a JSON consumer never races the write (see `checkAction`). Pass the command's own `xxxFlags()` slice to the reparser, as `templateLintFlags`/`scaffoldFlags` do; using `c.Command.Flags` breaks under hand-built test contexts.
- **Text output of every converted command is pinned by golden fixtures in `internal/commands/testdata/golden/`**: `TestUT_TextGolden` compares full bytes, because `assert.Contains` cannot see tabwriter column-width or trailing-whitespace drift. If one fails, the change is wrong — do NOT rerun with `-update-golden`. The fixtures' provenance is documented at the top of `golden_text_test.go`; the six #346 ones were captured before any source change, the four #345 ones by building `main` in a worktree. Regenerating a fixture from the working tree launders the regression it exists to catch.

