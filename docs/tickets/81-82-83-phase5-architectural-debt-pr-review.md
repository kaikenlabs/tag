# PR Review: Phase 5 Architectural Debt Cleanup (#81, #82, #83)

**Branch**: `refactor/phase-5-architectural-debt`
**Tickets**: #81 (Migrate generate off engine.New()), #82 (Merge parser/ into engine/), #83 (Structural cleanup)
**Date**: 2026-02-08
**Reviewers**: Claude (primary), Gemini 2.5 Flash, Codex gpt-5.2-codex

## Summary

Net deletion of ~880 lines. Merged `internal/parser/` into `internal/engine/`, removed deprecated `engine.New()`, extracted `scaffold/hookenv.go`, refactored `scaffold.Run()`, and updated `_templates/` to Gonja syntax.

**Files changed**: 11 modified, 4 new, 3 deleted
**Tests**: All passing (`go test ./...`), lint clean (`make lint`)

## Consensus: APPROVE

All three reviewers agree the changes are correct, well-executed, and introduce no regressions. The findings below are improvements for follow-up, not blockers.

---

## Findings

### MEDIUM-1: CLAUDE.md architecture diagram is stale

**Severity**: MEDIUM (documentation)
**Location**: `CLAUDE.md:131-148`

The architecture tree still lists:
- `internal/parser/` (deleted in this PR)
- `parser/lexer.go`, `parser/errors.go` (deleted in PR #85)
- `formats/stringers.go` (deleted in PR #85)

And does not list the new files:
- `engine/parser.go`, `engine/parser_types.go`, `engine/parser_test.go`
- `scaffold/hookenv.go`

**Action**: Update the architecture diagram in CLAUDE.md to reflect current file layout.

### MEDIUM-2: No dedicated tests for extracted scaffold functions

**Severity**: MEDIUM (test gap)
**Source**: All three reviewers flagged this
**Location**: `scaffold/scaffold.go` — `resolveOutputDir()`, `prepareOutputDir()`, `validateSafeOutputDir()`

These functions are tested indirectly through `scaffold.Run()` integration tests, but lack dedicated unit tests. `validateSafeOutputDir` is security-critical (prevents `--force` from deleting `/`, `$HOME`, `/usr`, etc.).

**Action**: Add table-driven unit tests in `scaffold/scaffold_test.go` covering:
- `resolveOutputDir`: empty outputDir with/without `project_name`, relative/absolute paths
- `prepareOutputDir`: force=true/false, dir exists/doesn't exist, unsafe paths
- `validateSafeOutputDir`: root dirs, home dir, shallow paths, valid deep paths

### LOW-1: TemplateParser is a concrete struct, not an interface

**Severity**: LOW (architecture)
**Source**: Codex noted this
**Location**: `engine/parser_types.go`

`TemplateParser` is a concrete struct, which means `Core` cannot accept a mock parser for unit testing. Currently this is not a problem because `Core.Generate` is tested via the real engine, and the `newEngine` function variable in `commands/generate.go` allows test injection at a higher level.

**Action**: No action needed now. If `Core`-level unit tests are added in the future, consider extracting a `Parser` interface.

### LOW-2: `writer.New()` deprecation warning suppressed with nolint

**Severity**: LOW (code quality)
**Source**: Claude found during lint
**Location**: `commands/generate.go:57`

`writer.New()` is the only constructor available but is marked deprecated. The `//nolint:staticcheck` comment is appropriate but the underlying issue is that `writer` lacks a non-deprecated constructor.

**Action**: In a future PR, consider adding `writer.NewFileWriter(dryRun bool) (FileWriter, error)` that returns the interface type, then remove the deprecated `New()`.

### NIT-1: Repository constructor template output format changed slightly

**Severity**: NIT
**Source**: Codex noted this
**Location**: `_templates/repo/repository_constructor.tmpl`

The old template used `printf` with explicit `\n` formatting. The new Gonja version uses `{{ name | pascal }}` directly, producing slightly different whitespace (blank line after metadata delimiter). This only affects example templates used for testing, not production output.

**Action**: None needed.

---

## Review Details by Ticket

### #82: Merge parser/ into engine/

**Verdict**: Clean merge, no issues.

- Types renamed appropriately: `TemplateEngine` -> `TemplateParser`, functions prefixed to avoid collisions (`buildParserContext`, `mergeParserMetadata`)
- `LoadTemplateFiles` correctly exported for cross-package access
- All 20 parser tests migrated and passing
- No circular dependencies introduced

### #81: Migrate generate off engine.New()

**Verdict**: Correct and well-structured.

- Pipeline construction in `newEngine` is explicit and readable
- `NewCore()` constructor enables dependency injection
- Shared template loading error handling preserved (non-fatal, logged)
- Function variable pattern (`var newEngine = func(...)`) maintained for test injection

### #83.3: Extract hookenv.go

**Verdict**: Clean extraction, no issues.

- Same package, no API changes
- Existing tests in `hooks_test.go` still cover `sanitizeEnvValue`, `formatEnvKey`, etc.

### #83.4: Extract scaffold.Run() sub-methods

**Verdict**: Correct refactoring.

- `resolveOutputDir`, `prepareOutputDir`, `saveReplayData` are pure functions (no struct receiver needed)
- `Run()` is now significantly more readable as an orchestration method
- All existing scaffold tests pass

### #83.5: Update _templates/ to Gonja syntax

**Verdict**: Correct syntax migration.

- `{{ .Name | caseSnake }}` -> `{{ name | snake }}` (and similar for camel, lower, pascal)
- `{{ .N.PascalCase }}` -> `{{ name | pascal }}`
- Go template `$` variables and `printf` replaced with direct Gonja filter expressions
- `lower` filter confirmed to exist in `template/filters.go` (maps to `strings.ToLower`)
