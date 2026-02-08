# Full Codebase Review - TAG Project

**Date**: 2026-02-08
**Reviewers**: Claude Opus 4.6, Gemini 2.5 Pro, GPT-5.2 Codex
**Scope**: Full codebase review targeting inconsistencies, security issues, and simplification opportunities
**Verdict**: REQUEST_CHANGES (consensus across all three reviewers)

---

## Summary

The TAG codebase is well-structured with good separation of concerns, comprehensive test coverage, and solid error handling patterns. However, all three reviewers independently identified several security concerns around untrusted template execution, symlink handling, and a few code quality issues that should be addressed before the project is considered production-hardened.

**Critical findings**: 2 | **High findings**: 5 | **Medium findings**: 7 | **Low findings**: 5

---

## CRITICAL Findings

### C1. Arbitrary Command Execution via Scaffold Hooks (Consensus: 3/3 reviewers)

- **File**: `internal/scaffold/hooks.go`
- **Function**: `(*ShellHookRunner).executeCommand`
- **Lines**: 82-133
- **Description**: Hook commands from `tag.template.json` are passed directly to `/bin/sh -c` (or `cmd.exe /C`). When scaffolding from a remote, untrusted template (e.g., `tag scaffold gh:malicious/template`), the template author can execute arbitrary commands on the user's machine before any output is created. Pre-scaffold hooks run *before* the output directory exists, meaning the user has no opportunity to inspect what will happen.
- **Impact**: Full remote code execution on the user's machine via a malicious template.
- **Recommendation**:
  1. Require explicit user confirmation before running hooks from remote templates (display the commands first).
  2. Add a `--allow-hooks` flag; disable hooks by default for remote templates.
  3. In non-interactive mode (`--no-input`), refuse to execute hooks from remote templates unless `--allow-hooks` is explicitly set.

### C2. Symlink Following in Template File Operations (Consensus: 3/3 reviewers)

- **File**: `internal/scaffold/output.go`
- **Functions**: `processFile` (L107-135), `copyDir` (L228-273)
- **Also affects**: `internal/remote/cache.go` `copyDir`/`copyFile` (L194-258)
- **Description**: When processing template files and copying `_generators`, symlinks are followed transparently. A malicious template can include a symlink pointing to sensitive local files (e.g., `~/.ssh/id_rsa`, `/etc/passwd`), which would be read and written into the scaffolded project output. The `convert/cookiecutter.go` correctly skips symlinks (L206-209), but `scaffold/output.go` and `remote/cache.go` do not.
- **Impact**: Exfiltration of sensitive local files into scaffolded output, potentially committed to version control.
- **Recommendation**: Add `Lstat` / `d.Type()&fs.ModeSymlink` checks in `scaffold/output.go:Write()`, `scaffold/output.go:copyDir()`, and `remote/cache.go:copyDir()`. Skip symlinks and emit a warning.

---

## HIGH Findings

### H1. Panic in `mergeInjection` When Matcher at Index 0 (Consensus: 2/3 reviewers)

- **File**: `internal/writer/injection.go`
- **Function**: `mergeInjection`
- **Line**: 52
- **Description**: The `InjectBefore` case uses `source[:(idx - 1)]` which will panic with an index-out-of-range error when the matcher is found at index 0 (start of file). This is a crash bug for valid inputs.
- **Recommendation**: Change to `source[:idx]` and handle the edge case where the matcher is at the beginning of the file.

### H2. Inconsistent Hook Execution Between Generate and Scaffold (Consensus: 3/3 reviewers)

- **File**: `internal/commands/generate.go` (L147-182) vs `internal/scaffold/hooks.go` (L60-133)
- **Description**: Two completely different hook systems exist:
  - `generate` hooks: Defined as `[][]string` (argv arrays), executed safely via `exec.Command` without a shell
  - `scaffold` hooks: Defined as `[]string` (shell strings), executed via `/bin/sh -c` with full shell expansion
  This inconsistency creates confusion for users and makes scaffold hooks vulnerable to command injection while generate hooks are safe.
- **Recommendation**: Standardize hook representation and execution. Either adopt the safer argv-based approach from `generate` for scaffold hooks, or create a shared hook execution engine used by both commands.

### H3. Dry-Run Reader Uses `filepath.Base` Instead of Full Path (Consensus: 2/3 reviewers)

- **File**: `internal/writer/write_logs.go`
- **Function**: `(*fileLog).ReadFile`
- **Line**: 21
- **Description**: The dry-run file reader strips the directory from the path using `filepath.Base(name)`, reading a completely different file than the real writer would. This causes incorrect behavior during dry-run injection operations where `ReadFile` is called to read the source before injection. The real writer at `write_files.go:28` correctly uses `filepath.Clean(name)`.
- **Recommendation**: Change `filepath.Base(name)` to `filepath.Clean(name)` to match the real writer's behavior.

### H4. Zip Extraction Size Limit Bypass (Consensus: 2/3 reviewers)

- **File**: `internal/remote/zip.go`
- **Functions**: `extract` (L154-197), `extractFile` (L226-257)
- **Description**: Total extracted size enforcement relies on `UncompressedSize64` from zip headers (which can be forged). Additionally, the `LimitReader` in `extractFile` applies `maxExtract` per-file rather than as a cumulative cap. A malicious zip can under-report sizes in headers while containing many files that together exhaust disk space.
- **Recommendation**: Track actual bytes extracted globally (not from headers) and enforce a cumulative cap across all files.

### H5. `AppendFile` and `InjectIntoFile` Missing Mutex Lock (Consensus: 1/3 reviewers - Gemini)

- **File**: `internal/writer/writer.go`
- **Functions**: `AppendFile` (L31-44), `InjectIntoFile` (L48-66)
- **Description**: Upon closer inspection, these methods DO have `w.mx.Lock()` / `defer w.mx.Unlock()` at their start. **Gemini's finding was incorrect on this point.** The mutex is properly used in all three write methods. However, note that the mutex uses `sync.RWMutex` but only ever takes write locks (`Lock()`), never read locks (`RLock()`). A plain `sync.Mutex` would be more appropriate.
- **Status**: DOWNGRADED to LOW (mutex is present; Gemini was wrong). Only the RWMutex vs Mutex type is a minor issue.

---

## MEDIUM Findings

### M1. Duplicated `isTextContent` Function (My analysis)

- **File**: `internal/scaffold/output.go` (L160-191) and `internal/convert/content.go` (L179-212)
- **Description**: Two nearly identical implementations of `isTextContent` exist in different packages. Both check for null bytes, UTF-8 validity, and non-printable character ratio with the same 8KB sample size and 10% threshold. This violates DRY.
- **Recommendation**: Extract to a shared utility package (e.g., `internal/formats` or a new `internal/fileutil`) and use from both locations.

### M2. Duplicated `copyFile` Function (My analysis)

- **Files**: `internal/scaffold/output.go:copyDir`, `internal/remote/cache.go:copyFile`, `internal/convert/cookiecutter.go:copyFile`, `internal/convert/hooks.go:copyFileWithMode`
- **Description**: Four separate implementations of file copying exist across the codebase. All open source, stat it, open destination, and `io.Copy`. They differ slightly in permission handling but share 90% of the logic.
- **Recommendation**: Create a shared `copyFile` utility function and use it across all packages.

### M3. Config File Created via `fmt.Sprintf` Instead of JSON Marshal (Consensus: 2/3 reviewers)

- **File**: `internal/config/config.go`
- **Function**: `CreateConfigFile`
- **Lines**: 58-84
- **Description**: The `.tagconfig.json` file is created by formatting a JSON string template with `fmt.Sprintf`, inserting user-provided values (path, extension, etc.) without escaping. If any value contains quotes, backslashes, or newlines, the resulting JSON will be malformed or contain unintended values. The scaffold command's `GenerateTagConfig` in `output.go` correctly uses `json.MarshalIndent`.
- **Recommendation**: Use `json.MarshalIndent` with a proper struct, matching the pattern in `scaffold/output.go:GenerateTagConfig`.

### M4. `app.Errorf` Complex Error Unwrapping Logic (Consensus: 2/3 reviewers)

- **File**: `pkg/app/errors.go`
- **Function**: `Errorf`
- **Lines**: 27-47
- **Description**: The error wrapping logic iterates over all args to find a wrapped error, using `errors.Is` to check the chain. This is overly complex and can be simplified using `errors.Unwrap` on the already-formatted error from `fmt.Errorf`.
- **Recommendation**: Simplify to:
  ```go
  func Errorf(format string, args ...any) error {
      err := fmt.Errorf(format, args...)
      return &CommandError{
          Message: err.Error(),
          Cause:   errors.Unwrap(err),
      }
  }
  ```

### M5. Inconsistent Use of `context.Background()` vs CLI Context (Consensus: 2/3 reviewers)

- **File**: `internal/commands/scaffold.go` (L124), `internal/commands/convert.go` (L109)
- **Description**: Both `scaffoldAction` and `convertCookiecutterAction` create `context.Background()` instead of using `c.Context` from the CLI framework. Using `c.Context` would allow Ctrl+C signals to propagate through the application and cancel long-running operations like git clones.
- **Recommendation**: Replace `context.Background()` with `c.Context` in all CLI action handlers.

### M6. `NewPathProcessor` Panics on Engine Creation Failure (My analysis)

- **File**: `internal/scaffold/processor.go`
- **Function**: `NewPathProcessor`
- **Lines**: 24-31
- **Description**: `NewPathProcessor` calls `template.NewEngine()` and panics on error. While the comment says "this should never fail with default options," panicking in a library function is not idiomatic Go. The caller (`NewOutputWriter` and `NewScaffold`) could handle an error return.
- **Recommendation**: Return `(*DefaultPathProcessor, error)` instead of panicking, and propagate the error to callers.

### M7. `path.Join` vs `filepath.Join` Mixed Usage (My analysis)

- **Files**: `internal/commands/init.go` (L30, L36), `internal/commands/new.go` (L48), `internal/commands/bundle.go` (L50)
- **Description**: These files use `path.Join` (which uses forward slashes, designed for URL paths) alongside `filepath.Dir` (which is OS-aware). On Windows, `path.Join` would produce incorrect paths. The newer code (scaffold, convert, remote) consistently uses `filepath.Join`.
- **Recommendation**: Replace all `path.Join` with `filepath.Join` in the commands package for cross-platform correctness.

---

## LOW Findings

### L1. Custom `trimSpace` Reimplementation (Consensus: 3/3 reviewers)

- **File**: `internal/replay/save.go`
- **Functions**: `trimSpace`, `isSpace`
- **Lines**: 105-122
- **Description**: A custom `trimSpace` function is implemented to "avoid importing strings package just for TrimSpace." This rationale is invalid -- `strings` is a standard library package with zero cost. The custom version only handles ASCII whitespace, missing Unicode whitespace characters that `strings.TrimSpace` handles.
- **Recommendation**: Remove `trimSpace` and `isSpace`, use `strings.TrimSpace` instead.

### L2. Deprecated `strings.Title` Usage (My analysis)

- **File**: `internal/template/filters.go`
- **Function**: `filterTitle`
- **Line**: 141
- **Description**: Uses `strings.Title` which is deprecated since Go 1.18 due to incorrect Unicode handling. The `//nolint:staticcheck` comment acknowledges this but doesn't fix it.
- **Recommendation**: Use `golang.org/x/text/cases` package's `cases.Title(language.English).String()` for correct behavior, or use the existing `flect.Titleize` which is already available.

### L3. Typo in Bundle Command Usage (My analysis)

- **File**: `internal/commands/bundle.go`
- **Line**: 28
- **Description**: Usage string says "creates a new **bunle**" instead of "bundle".
- **Recommendation**: Fix typo: `bunle` -> `bundle`.

### L4. `sync.RWMutex` Used as Plain Mutex (My analysis)

- **File**: `internal/writer/write_files.go` / `writer.go`
- **Description**: The `Write` struct uses `sync.RWMutex` but only ever calls `Lock()`, never `RLock()`. A plain `sync.Mutex` would be simpler and more semantically correct.
- **Recommendation**: Change to `sync.Mutex`.

### L5. Unused Variable Assignment in Hooks (My analysis)

- **File**: `internal/convert/hooks.go`
- **Lines**: 21-28
- **Description**: `var _ = map[string]struct{}{...}` creates a map that is assigned to blank identifier solely for documentation. A comment would be cleaner.
- **Recommendation**: Replace with a comment listing recognized hook patterns.

---

## Testing Gaps Identified

1. **No symlink tests** for scaffold output, `_generators` copy, or cache copy operations
2. **No test for `mergeInjection`** when matcher is at index 0 (crash bug)
3. **No zip bomb / size limit bypass tests** in `remote/zip.go`
4. **No test for `CreateConfigFile`** with values containing special JSON characters
5. **Missing integration tests** for the full scaffold-from-remote flow with hooks

---

## Architecture Notes

- The codebase shows a clear evolution: older code (`commands/init.go`, `commands/new.go`, `commands/bundle.go`, `engine/`, `writer/`, `parser/`, `config/`) uses Go `text/template` style patterns, while newer code (`scaffold/`, `remote/`, `convert/`, `replay/`, `template/`) uses the Gonja engine with more modern Go patterns. This is expected given the migration history.
- The newer packages have significantly better test coverage and error handling than the legacy code.
- Interface design is clean with proper compile-time checks (`var _ Interface = (*Impl)(nil)`).

---

## Reviewer Consensus

| Finding | Claude | Gemini | Codex | Consensus |
|---------|--------|--------|-------|-----------|
| C1 - Hook RCE | YES | YES | YES | **3/3** |
| C2 - Symlink following | YES | YES | YES | **3/3** |
| H1 - mergeInjection panic | YES | NO | YES | **2/3** |
| H2 - Inconsistent hooks | YES | YES | YES | **3/3** |
| H3 - Dry-run ReadFile bug | YES | NO | YES | **2/3** |
| H4 - Zip size bypass | YES | NO | YES | **2/3** |
| M1 - Duplicated isTextContent | YES | NO | NO | **1/3** |
| M2 - Duplicated copyFile | YES | NO | NO | **1/3** |
| M3 - Config Sprintf | YES | NO | YES | **2/3** |
| M4 - Errorf complexity | YES | YES | NO | **2/3** |
| M5 - context.Background | YES | YES | NO | **2/3** |
| M6 - Panic in NewPathProcessor | YES | NO | NO | **1/3** |
| M7 - path.Join vs filepath.Join | YES | NO | NO | **1/3** |
| L1 - Custom trimSpace | YES | YES | YES | **3/3** |
| L2 - Deprecated strings.Title | YES | NO | NO | **1/3** |
| L3 - Typo "bunle" | YES | NO | NO | **1/3** |
| L4 - RWMutex as Mutex | YES | NO | NO | **1/3** |
| L5 - Unused hook map | YES | NO | NO | **1/3** |

---

*Generated with Claude Opus 4.6, Gemini 2.5 Pro, and GPT-5.2 Codex*
