# PR Review: Epic 5 - Replay System (Input Persistence)

**Ticket**: #10
**Date**: 2026-02-05
**Reviewers**: Claude, Gemini (gemini-2.5-flash), Codex (gpt-5.2-codex)

## Summary

Implementation of the Replay System for TAG scaffold command, enabling automatic saving and replaying of scaffold inputs for reproducible project generation.

## Review Verdict

**APPROVED WITH MINOR CHANGES**

All three reviewers (Claude, Gemini, Codex) agree the implementation is solid with no critical issues. Minor improvements recommended before merge.

---

## Files Reviewed

| File | Lines Changed | Status |
|------|---------------|--------|
| `internal/replay/replay.go` | +25 | ✅ Clean |
| `internal/replay/errors.go` | +37 | ✅ Clean |
| `internal/replay/id.go` | +97 | ⚠️ Minor issues |
| `internal/replay/save.go` | +89 | ✅ Clean |
| `internal/replay/load.go` | +95 | ✅ Clean |
| `internal/replay/replay_test.go` | +350 | ✅ Comprehensive |
| `internal/scaffold/variables.go` | +93 | ✅ Clean |
| `internal/scaffold/scaffold.go` | +30 | ✅ Clean |
| `internal/scaffold/types.go` | +13 | ✅ Clean |
| `internal/commands/scaffold.go` | +19 | ✅ Clean |
| `internal/scaffold/variables_test.go` | +259 | ✅ Comprehensive |

---

## Issues Found

### Major Issues

**None identified.**

### Minor Issues

#### 1. Local Path Canonicalization (id.go - `GenerateTemplateID`)

**Severity**: Minor
**Consensus**: Claude + Codex agree
**Description**: Local paths are normalized using `filepath.Abs()` only. Symlinks and case differences (on macOS) can cause the same directory to generate different replay IDs.

**Current Code**:
```go
absPath, err := filepath.Abs(source)
if err != nil {
    absPath = source
}
return hashBasedID("local", absPath)
```

**Recommendation**: Consider adding `filepath.EvalSymlinks()` for better canonicalization, or document the limitation.

**Decision**: Document limitation for v1, consider enhancement in future.

---

#### 2. `sanitizeComponent` Loop Efficiency (id.go)

**Severity**: Minor
**Consensus**: Codex identified
**Description**: The double-underscore collapse uses a loop which is slightly inefficient for long strings.

**Current Code**:
```go
for strings.Contains(sanitized, "__") {
    sanitized = strings.ReplaceAll(sanitized, "__", "_")
}
```

**Recommendation**: Could use regex or single-pass replacement, but impact is negligible for typical template names.

**Decision**: No change needed - premature optimization.

---

#### 3. Version Field Documentation (replay.go)

**Severity**: Minor
**Consensus**: Gemini identified
**Description**: The `Version` field purpose is not documented clearly.

**Recommendation**: Add comment explaining it stores the template version from `tag.template.json`.

**Decision**: Add documentation comment.

---

### Informational Notes

#### Secret Handling Verification

**Consensus**: Codex flagged for review
**Analysis**: Verified that `Save()` is called in `scaffold.go:169` with the final `vars` map AND the `secrets` map built from `config.Vars`. The filtering happens at save time on the complete merged map, which is correct.

**Status**: No issue - implementation is correct.

#### Atomic Write on Windows

**Note from Codex**: `os.Rename` overwriting behavior differs on Windows.
**Status**: TAG is primarily a Unix tool, Windows behavior acceptable for v1.

---

## Security Review

| Check | Status | Notes |
|-------|--------|-------|
| File permissions (0600) | ✅ Pass | Replay files are user-read-only |
| Directory permissions (0700) | ✅ Pass | Replay directory is user-access-only |
| Path traversal | ✅ Pass | Template IDs are sanitized before use as filenames |
| Secret filtering | ✅ Pass | Secrets excluded from saved replay data |
| Atomic writes | ✅ Pass | Write to temp file then rename |

---

## Test Coverage Review

| Area | Coverage | Notes |
|------|----------|-------|
| ID Generation | ✅ Comprehensive | All source formats tested |
| Save/Load | ✅ Comprehensive | Success, errors, permissions |
| Secret Filtering | ✅ Covered | Secrets excluded in test |
| Priority Chain | ✅ Covered | defaults→replay→values→prompts→meta |
| Error Cases | ✅ Covered | Not found, corrupt, empty source |

**Identified Test Gaps** (low priority):
- Symlink canonicalization behavior
- Windows atomic rename behavior

---

## Required Changes Before Merge

1. **Add documentation comment to `Version` field** in `replay.go`

---

## Recommended Future Improvements

1. Consider `filepath.EvalSymlinks()` for local path canonicalization
2. Add `--list-replays` command to show saved replay files
3. Add `--clear-replay` command to delete replay data

---

## Approval

| Reviewer | Verdict |
|----------|---------|
| Claude | ✅ Approve with minor changes |
| Gemini | ✅ Approve with minor changes |
| Codex | ✅ Approve with minor changes |

**Final Decision**: APPROVED - Fix minor documentation issue then merge.
