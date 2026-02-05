# PR Review: Epic 2 - Internal Migration (Go text/template to Gonja)

**Ticket**: #7
**Reviewers**: Claude, Gemini (gemini-2.5-flash), Codex (gpt-5.2-codex)
**Date**: 2026-02-05
**Status**: Requires Changes

---

## Summary

This PR migrates TAG's existing template processing from Go's `text/template` to Gonja (Jinja2-compatible). The changes include:

- New `internal/template/metadata.go` - Metadata extraction and parsing
- Updated `internal/parser/parser.go` - Uses Gonja engine
- Updated `internal/parser/types.go` - New `InputData` type
- Updated `internal/engine/engine.go` - Uses new parser interface
- Updated example templates to Jinja2 syntax

---

## Critical Issues (Must Fix)

### 1. Shared Templates Not Wired to Gonja Loader

**Location**: `internal/parser/parser.go:28-35`

**Issue**: Shared templates are loaded into `sharedTemplates` map but never connected to Gonja's loader. This means `{% include %}` and `{% extends %}` directives won't work.

**Fix Required**: Wire shared templates into Gonja's memory loader or file loader.

**Functions Affected**:
- `parser.New()` - needs to configure Gonja's loader with shared templates
- `parser.renderBody()` - ensure it uses the configured loader

### 2. Missing `To` Field Validation

**Location**: `internal/parser/parser.go:79-107`, `internal/template/metadata.go`

**Issue**: Templates with no metadata block or missing `to:` field produce `Metadata{Action: Create}` with empty `To`. This will propagate to `writer.WriteFile("", ...)` and fail or create files in unexpected locations.

**Fix Required**: Add validation that `To` is non-empty after metadata parsing, or error early.

**Functions Affected**:
- `ParseMetadata()` - could validate required fields
- `parseTemplate()` - could check `metadata.To` before proceeding

---

## Important Issues (Should Fix)

### 3. Inject Clause Without Inject Action

**Location**: `internal/template/metadata.go:164-188`

**Issue**: `ParseMetadata` allows `after:` or `before:` without `inject: true`. It also allows empty matcher values. This creates incoherent state.

**Fix Required**:
- Only process `after:`/`before:` if `inject: true` is set
- Validate matcher is non-empty when inject clause is present

**Functions Affected**:
- `ParseMetadata()` - add validation logic

### 4. Silent Failure in `generateMeta`

**Location**: `internal/engine/engine.go:86-101`

**Issue**: If any single `--meta` entry is malformed, the function returns an empty map, silently dropping ALL user-provided metadata. This is hard to diagnose.

**Fix Required**: Either return an error or log a warning for malformed entries while preserving valid ones.

**Functions Affected**:
- `generateMeta()` - improve error handling

### 5. Missing Tests for Gonja-Specific Features

**Location**: Test files

**Issue**: No tests cover Gonja-specific features like `{% include %}`, `{% extends %}`, complex filters, or control flow beyond basic cases.

**Fix Required**: Add integration tests for:
- Template includes
- Template extends/inheritance
- Complex filter chains
- Macros

---

## Minor Issues (Nice to Fix)

### 6. Whitespace Trimming in Metadata Values

**Location**: `internal/template/metadata.go:81-95`

**Issue**: Each metadata line is trimmed, which could unintentionally strip significant whitespace in values.

**Recommendation**: Trim only after splitting on `:` and only trim the key aggressively.

### 7. Shared Templates Loading Silently Ignores Errors

**Location**: `internal/parser/parser.go:35`

**Issue**: `sharedTemplates, _ := withTemplates(sharedPath, fileSuffix)` silently ignores errors.

**Recommendation**: At minimum, log a warning when shared template loading fails.

### 8. Nested Template Directories Not Supported

**Location**: `internal/parser/parser.go:205-224`

**Issue**: `withTemplates` only reads top-level directory, not subdirectories.

**Recommendation**: Document this limitation or implement recursive loading if needed.

### 9. Deprecated Fields in ParseData

**Location**: `internal/parser/types.go:39-58`

**Issue**: `Args` and `N` fields in `ParseData` are never populated but exist in the struct.

**Recommendation**: Either remove them or document why they're kept for compatibility.

---

## Positive Observations

1. **Clean Separation of Concerns**: `metadata.go` handles parsing, `parser.go` orchestrates - excellent maintainability
2. **Robust Metadata Handling**: Accounts for optional metadata, defaults, and clear CLI/template precedence
3. **Clear Error Types**: Specific error types (`ErrMalformedMetadata`, etc.) enable precise error handling
4. **Good Test Coverage for Metadata**: Solid coverage for booleans, malformed lines, custom fields
5. **Idiomatic Go**: Uses `slog` for logging, proper error propagation, clear function names
6. **Staged Pipeline**: The extract → render → parse → render body flow is easy to follow
7. **Conversion Helpers**: `convertAction()` and `convertInjectClause()` make cross-package boundaries explicit

---

## Consensus Decision

**REQUIRES CHANGES** before merge.

The critical issues (#1, #2) must be addressed. The shared templates regression breaks existing functionality, and missing `To` validation can cause file system errors.

---

## Action Items

1. [x] Fix shared templates loader wiring (Critical) - FIXED
2. [x] Add `To` field validation (Critical) - FIXED
3. [x] Add inject clause validation (Important) - FIXED
4. [x] Improve `generateMeta` error handling (Important) - FIXED
5. [ ] Add integration tests for Gonja features (Important) - Deferred
6. [ ] Address minor issues as time permits

---

## Fixes Applied

### Fix 1: Shared Templates Loader Wiring
- Added `CreateMemoryLoaderFromMap()` helper in `internal/template/loader.go`
- Updated `parser.New()` to wire shared templates into Gonja's loader
- Added debug logging for shared template loading

### Fix 2: To Field Validation
- Added `ErrMissingToField` error in `internal/template/metadata.go`
- Added validation in `parser.parseTemplate()` that errors if `To` is empty
- Added tests: `TestUT_Parse_MissingToFieldErrors`, `TestUT_Parse_NoMetadataBlockErrors`

### Fix 3: Inject Clause Validation
- Added `ErrEmptyInjectMatcher` error for empty matcher values
- Added `ErrOrphanInjectClause` warning (handled gracefully)
- Orphan `before:`/`after:` clauses without `inject: true` are now silently cleared
- Added tests: `TestUT_ParseMetadata_EmptyInjectMatcher`, `TestUT_ParseMetadata_OrphanInjectClause`

### Fix 4: generateMeta Error Handling
- Changed from returning empty map on malformed entry to logging warning and continuing
- Valid entries are now preserved even if some are malformed
