# PR Review: #79 Template Caching + #80 Minor Performance Optimizations

**Tickets**: #79 (Phase 4.1-4.2: Template caching), #80 (Phase 4.3-4.5: Minor perf opts)
**Branch**: `perf/phase-4-template-caching-and-optimizations`
**Date**: 2026-02-08
**Reviewers**: Claude Opus 4.6, Gemini 2.5 Flash, GPT-5.2 Codex

## Summary

Four performance optimizations implemented:
1. Content-hash template caching in `template/engine.go` (SHA-256, sync.RWMutex)
2. Package-level `strings.NewReplacer` in `remote/reference.go`
3. `os.Getwd()` caching at construction in `writer/writer.go`
4. Binary file streaming via `io.Copy` in `scaffold/output.go`

## Consensus

All three reviewers approved the changes. Gemini gave unconditional approval. Codex raised 5 findings (2 High, 2 Medium, 1 Low) which were all evaluated and resolved as non-issues (see below).

## Findings

### Finding 1 (Codex, High): Cache key ignores template name

**Location**: `internal/template/engine.go:130`
**Concern**: Templates are cached by content hash only. If the Gonja loader key (name) affects execution output, reusing a cached `*exec.Template` parsed under a different name could produce wrong results.
**Verdict**: **Not a bug.** Verified in Gonja v2.5.2 source: the template identifier is stored in `nodes.Template.Identifier` but never referenced during rendering (`Renderer.Execute` walks AST nodes without checking it). It only affects error messages and `{% include %}`/`{% extends %}` resolution. Our `gonjaTemplate` wrapper stores the correct name per-caller for error reporting.
**Test coverage**: `TestUT_Engine_Cache_SameContentDifferentNames` validates same content with different names produces identical output.

### Finding 2 (Codex, High): Potential data race on `*exec.Template`

**Location**: `internal/template/engine.go:132`
**Concern**: Multiple goroutines calling `ExecuteToString` on the same cached `*exec.Template` could race if the struct has mutable state.
**Verdict**: **Not a bug.** Verified in Gonja v2.5.2 source: `*exec.Template` is immutable after construction. `ExecuteToString` creates a fresh `bytes.Buffer`, `Renderer`, and inherited `Context` per call. Shared environment state (`FilterSet`, `TestSet`, `MethodSet`) is `sync.Mutex`-protected.
**Test coverage**: `TestUT_Engine_Cache_ConcurrentAccess` validates correctness with 100 concurrent goroutines.

### Finding 3 (Codex, Medium): CWD caching changes behavior on Chdir

**Location**: `internal/writer/writer.go:18`
**Concern**: Caching `os.Getwd()` at construction means subsequent `os.Chdir()` calls won't be reflected.
**Verdict**: **By design.** The old code called `os.Getwd()` three times per operation (once per method), which created a TOCTOU risk if cwd changed between calls. Caching at construction provides a consistent base directory for path containment validation throughout the writer's lifetime. No callers in the codebase call `os.Chdir()` after writer construction.

### Finding 4 (Codex, Medium): Unbounded cache

**Location**: `internal/template/engine.go:18-21`
**Concern**: Cache grows without bound, could leak memory in long-running processes.
**Verdict**: **Accepted trade-off.** User explicitly chose "Unbounded map" during implementation planning. TAG is a CLI tool (not a long-running server), and template content per project is finite. Adding LRU would increase complexity without meaningful benefit.

### Finding 5 (Codex, Low): Short write in streamBinaryFile

**Location**: `internal/scaffold/output.go:186`
**Concern**: `dst.Write(sample)` doesn't check for short writes.
**Verdict**: **Not an issue.** Go's `os.File.Write` wraps `syscall.Write` in a retry loop and returns an error if not all bytes are written. Short writes only occur on non-blocking fds or pipes, not regular files. The return value check (`if _, err := dst.Write(sample); err != nil`) is sufficient.

## Benchmark Results

| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| ParseString (single) | 244 us | 293 ns | 833x faster |
| ExecuteToString (single) | 256 us | 12 us | 22x faster |
| Allocations (parse) | 547 allocs | 66 allocs | 88% reduction |

## Files Changed

- `internal/template/engine.go` - Template caching (cache struct, contentHash, parseWithName modification)
- `internal/template/cache_test.go` - New file: 7 tests + 4 benchmarks
- `internal/remote/reference.go` - Package-level pathSanitizer var
- `internal/writer/writer.go` - CWD caching, New() returns (Write, error)
- `internal/writer/write_files.go` - Added cwd field to Write struct
- `internal/writer/writer_test.go` - Updated tests for new Write struct, added TestUT_New_CachesGetwd
- `internal/engine/engine.go` - Updated writer.New() call to handle error
- `internal/scaffold/output.go` - Binary streaming (openRegularFile, streamBinaryFile, processFile refactor)
- `internal/scaffold/output_test.go` - Added tests for openRegularFile, streamBinaryFile, large binary e2e

## Action Items

None. All findings resolved as non-issues with existing test coverage.
