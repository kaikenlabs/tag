# PR Review: Epic 1 - Template Engine (Gonja Integration)

**Ticket:** #6 - Epic 1: Template Engine (Gonja Integration)
**Review Date:** 2026-02-05
**Reviewers:** Claude, Gemini (gemini-2.5-flash), Codex (gpt-5.2-codex)
**Status:** APPROVED WITH FIXES REQUIRED

---

## Executive Summary

The implementation of the Gonja-based template engine is well-structured and follows Go idioms. The core functionality works correctly with comprehensive test coverage. However, several issues were identified that should be addressed before merging:

- **1 Critical** issue (path traversal security)
- **3 High** severity issues (panics, UTF-8 truncation, unused strict mode)
- **4 Medium** severity issues
- **5 Low** severity issues

---

## Consensus Decision

All three reviewers **APPROVE** the code for merging **after the Critical and High severity issues are addressed**.

---

## Findings by Severity

### CRITICAL

#### 1. Path Traversal Vulnerability in Loader
**File:** `loader.go`
**Functions:** `Loader.Load`, `resolvePath`

**Issue:** The `Load` method does not adequately sanitize the `path` argument. When `l.fs` is nil, `os.ReadFile(fullPath)` is used where `fullPath` is derived from `resolvePath`. If `resolvePath` does not properly sanitize inputs, a malicious template path (e.g., `../../../../etc/passwd`) could read arbitrary files.

Additionally, when `l.fs != nil`, the code calls `fs.ReadFile(l.fs, path)` but computes `fullPath` and ignores it, creating inconsistent behavior.

**Recommendation:**
- Normalize and validate `path` using `filepath.Clean`
- Reject absolute paths and `..` segments
- Ensure resolved path is within `baseDir`
- Add path traversal tests

---

### HIGH

#### 2. Panic on Filter Registration Failure
**File:** `filters.go`
**Function:** `mustRegister`

**Issue:** The function panics if filter registration or replacement fails. Panics in library code can crash production services and are difficult to recover from.

**Recommendation:**
- Return `error` from `RegisterFilters`
- Propagate errors through `createEnvironment` and `NewEngine`
- Allow callers to decide whether to fail fast

---

#### 3. UTF-8 Corruption in Truncate Filter
**File:** `filters.go`
**Function:** `filterTruncate`

**Issue:** `s[:length]` slices by bytes, not runes. This can split multi-byte UTF-8 codepoints, producing invalid strings. Additionally, `length` from `Integer()` may be negative and is not validated.

**Recommendation:**
- Validate `length >= 0`
- Truncate by rune count using `[]rune` conversion
- Add test cases with multi-byte UTF-8 characters

---

#### 4. Strict Mode Field is Unused
**File:** `engine.go`
**Field:** `Engine.strict`

**Issue:** The `strict` field is set via `WithStrictUndefined` option but never used. Callers expect strict mode to control undefined variable behavior, but it's not wired into the Gonja config.

**Recommendation:**
- Wire `strict` into `config.New()` or Gonja environment settings
- Or remove the field if strict mode is not supported by Gonja
- Document the actual behavior

---

### MEDIUM

#### 5. Memory Loader Key Collision for Empty Names
**File:** `engine.go`
**Function:** `parseWithName`

**Issue:** When `name` is empty, `loaderKey` becomes `/`, which could cause collisions if multiple unnamed templates are parsed.

**Recommendation:**
- Use a unique synthetic key for empty names (e.g., `"/_inline/<counter>"`)
- Or reject empty names with an error

---

#### 6. CreateFileSystemLoader Uses Must Function
**File:** `loader.go`
**Function:** `CreateFileSystemLoader`

**Issue:** Uses `loaders.MustNewFileSystemLoader` which can panic for errors other than non-existent directory (e.g., permission errors).

**Recommendation:**
- Use non-"Must" constructor if available
- Or wrap in recover and return error
- Document panic behavior

---

#### 7. Line/Column Always Zero in Parse Errors
**File:** `engine.go`
**Function:** `parseWithName`

**Issue:** `NewParseError(name, 0, 0, err)` always passes 0 for line and column, reducing debuggability.

**Recommendation:**
- Extract line/column from Gonja error if available
- Or remove unused Line/Column fields from error struct

---

#### 8. Inconsistent Filter Argument Handling
**File:** `filters.go`
**Functions:** Various filter functions

**Issue:** Some filters use `params.Take()` (validates no args), others access `params.Args` directly. This creates inconsistent arity validation.

**Recommendation:**
- Standardize argument handling with a helper function
- Ensure all filters validate argument count consistently

---

### LOW

#### 9. Magic String "cookiecutter" as Alias
**File:** `context.go`
**Function:** `NewContext`

**Issue:** The string `"cookiecutter"` is used directly without a constant, reducing maintainability.

**Recommendation:**
- Define a constant: `const CookiecutterAlias = "cookiecutter"`

---

#### 10. Context Shares Input Map Reference
**File:** `context.go`
**Function:** `NewContext`

**Issue:** `vars` is used directly as `ctx["vars"]` and `ctx["cookiecutter"]`. If caller mutates `vars` after calling `NewContext`, it affects the template context.

**Recommendation:**
- Document that `vars` is shared (not copied)
- Or shallow-copy `vars` for isolation

---

#### 11. Error Types Could Be More Specific
**File:** `filters.go`
**Functions:** Filter functions returning errors

**Issue:** Filters return `exec.AsValue(fmt.Errorf(...))`. Custom error types would allow more precise error checking.

**Recommendation:**
- Consider using `NewFilterError` for consistency
- Or define filter-specific error types

---

#### 12. Unused NewFilterError Function
**File:** `errors.go`
**Function:** `NewFilterError`

**Issue:** This function is defined but never used in the codebase.

**Recommendation:**
- Use it in filter error handling
- Or remove if not needed

---

#### 13. GonjaLoader Not Used in Engine
**File:** `loader.go`
**Types:** `GonjaLoader`, `Loader`

**Issue:** The custom `Loader` and `GonjaLoader` types are implemented but not used by the `Engine`. The engine uses Gonja's built-in loaders instead.

**Recommendation:**
- Integrate custom loader with Engine if needed
- Or document these as utility types for advanced use
- Or remove if not needed

---

## Test Coverage Gaps

The following test scenarios should be added:

1. **Path traversal tests** - Attempt `../../` patterns in loader
2. **UTF-8 truncation** - Test with multi-byte characters (emoji, CJK)
3. **Negative truncate length** - Validate error handling
4. **Empty template name** - Verify unique key generation
5. **Filter registration failure** - Test panic behavior (if kept)
6. **Strict mode** - Test undefined variable handling (once implemented)
7. **Permission errors** - Test loader with unreadable directories

---

## Files Reviewed

| File | Lines | Status |
|------|-------|--------|
| `internal/template/engine.go` | 177 | Needs fixes |
| `internal/template/filters.go` | 324 | Needs fixes |
| `internal/template/context.go` | 106 | Minor issues |
| `internal/template/loader.go` | 156 | Needs fixes |
| `internal/template/types.go` | 46 | OK |
| `internal/template/errors.go` | 84 | Minor issues |
| `internal/template/*_test.go` | ~600 | Good coverage |

---

## Action Items Summary

| Priority | Issue | File | Function |
|----------|-------|------|----------|
| **CRITICAL** | Path traversal | loader.go | `Load`, `resolvePath` |
| **HIGH** | Panic on filter registration | filters.go | `mustRegister` |
| **HIGH** | UTF-8 truncation | filters.go | `filterTruncate` |
| **HIGH** | Unused strict mode | engine.go | `Engine.strict` |
| MEDIUM | Empty name collision | engine.go | `parseWithName` |
| MEDIUM | MustNewFileSystemLoader panic | loader.go | `CreateFileSystemLoader` |
| MEDIUM | Line/column always zero | engine.go | `parseWithName` |
| MEDIUM | Inconsistent arg handling | filters.go | Various |

---

## Reviewer Notes

### Gemini
> "The primary concerns revolve around robustness of error handling (especially panics vs. errors), potential path traversal in loader.go, and ensuring adequate test coverage for edge cases and failure scenarios."

### Codex
> "The loader path handling is the primary security concern. If templates are user-supplied, ensure you consider sandboxing of filters/functions."

### Claude
> "The implementation is solid overall. The critical path traversal issue must be fixed before this can be safely used with user-supplied template paths. The other high-severity issues should also be addressed to ensure production readiness."
