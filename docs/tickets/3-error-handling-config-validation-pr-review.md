# PR Review: Phase 3 - Error Handling Improvements & Config Validation

**Ticket:** #3
**Date:** 2026-02-05
**Reviewers:** Claude, Gemini (gemini-2.5-pro), Codex (gpt-5.2-codex)

---

## Summary

The implementation adds config validation and fixes a silenced error in the lexer. While the overall code quality is good, **critical issues were identified** that must be fixed before merging.

---

## Consensus Decision: REQUEST CHANGES

All three reviewers agree changes are needed before approval.

---

## Issues Found

### 🔴 CRITICAL: PATH Command Validation vs Execution Mismatch

**Severity:** Critical
**File:** `internal/config/validate.go` (validation) + `internal/commands/generate.go:163` (execution)
**Identified by:** Codex

**Problem:**
- Validation allows PATH-resolved commands (e.g., `echo`, `git`) via `exec.LookPath`
- However, `runCommand()` executes hooks using `filepath.Join(dir, hook[0])`
- This means `echo` becomes `/current/dir/echo` which doesn't exist
- **Result:** Validation passes, runtime fails

**Fix Required:**
Either:
1. Change `runCommand()` to use `exec.Command(hook[0], hook[1:]...)` directly (let OS resolve PATH)
2. Or change validation to reject bare commands without path separators

**Recommendation:** Option 1 - Fix `runCommand()` to match validation behavior.

---

### 🟡 HIGH: Missing Executable Permission Check

**Severity:** High
**File:** `internal/config/validate.go:89-96`
**Identified by:** Gemini, Codex

**Problem:**
- `validateHooks` uses `os.Stat` to check if hook file exists
- Does not verify the file has execute permissions
- Does not verify the path is not a directory
- **Result:** Validation passes for non-executable files, runtime fails with "permission denied"

**Fix Required:**
Add checks in `validateHooks` for path-based hooks:
1. Verify file is not a directory (`info.IsDir()`)
2. Verify file has execute permission (`info.Mode()&0111 != 0` on POSIX)

---

### 🟡 MEDIUM: Whitespace Normalization Inconsistency

**Severity:** Medium
**File:** `internal/config/validate.go:81`
**Identified by:** Codex

**Problem:**
- Validation trims whitespace: `cmd := strings.TrimSpace(hook[0])`
- Execution uses original value: `hook[0]`
- A hook like `["  echo"]` passes validation but fails execution

**Fix Required:**
Validation should reject commands with leading/trailing whitespace rather than silently trimming, OR the trimmed value should be used consistently.

---

### 🟢 LOW: Test Platform Dependency

**Severity:** Low
**File:** `internal/config/validate_test.go:94-104`
**Identified by:** Codex

**Problem:**
- `TestUT_Validate_ValidHookInPath` assumes `echo` is in PATH
- On Windows, `echo` is a shell builtin and `exec.LookPath("echo")` may fail
- Test is platform-flaky

**Fix Suggested:**
Use a more portable command or skip test on Windows.

---

### 🟢 LOW: Shared Template Parse Failure Handling

**Severity:** Low (design decision)
**File:** `internal/parser/lexer.go:82-87`
**Identified by:** Codex

**Problem:**
- Shared template parse failures are logged but ignored
- Execution continues with partially loaded templates
- Could lead to confusing downstream errors

**Note:** This matches the ticket specification (warn, don't fail). No change required unless design changes.

---

## Security Concerns (Informational)

Both Gemini and Codex noted:

1. **Arbitrary Command Execution:** Hooks allow execution of any command from config. This is by design for a developer tool but should be documented as a security consideration.

2. **TOCTOU Race Condition:** Theoretical race between validation and execution. Acceptable for CLI tools.

**No code changes required** - recommend adding security note to documentation.

---

## Missing Edge Cases

| Edge Case | Status | Action |
|-----------|--------|--------|
| Non-executable hook files | Not tested | Add validation check |
| Hook path is directory | Not tested | Add validation check |
| PATH commands in hooks | Broken | Fix `runCommand()` |
| Whitespace in hook commands | Inconsistent | Normalize or reject |
| Symlink hooks | Not explicitly handled | Document behavior |
| Shell builtins (`cd`, `source`) | Won't work | Document limitation |

---

## Required Fixes Before Approval

1. ✅ **`internal/commands/generate.go:163`** - Fix `runCommand()` to not prepend working directory for all commands - **FIXED**
2. ✅ **`internal/config/validate.go:89-96`** - Add directory check and executable permission check for path-based hooks - **FIXED**
3. ✅ **`internal/config/validate.go:81`** - Either reject whitespace-padded commands or document trimming behavior - **FIXED** (rejects whitespace)

---

## Recommended Fixes (Non-Blocking)

1. **`internal/config/validate_test.go`** - Use cross-platform command for PATH test
2. **Documentation** - Add security note about hook command execution

---

## Files Requiring Changes

| File | Changes Needed |
|------|----------------|
| `internal/commands/generate.go` | Fix `runCommand()` to handle PATH commands |
| `internal/config/validate.go` | Add executable/directory checks, handle whitespace |
| `internal/config/validate_test.go` | Add tests for new validation, fix platform-specific test |

---

## Review Sign-off

| Reviewer | Decision | Notes |
|----------|----------|-------|
| Claude | Request Changes | Critical PATH/execution mismatch must be fixed |
| Gemini | Request Changes | Executable permission check needed |
| Codex | Request Changes | Multiple issues identified, PATH mismatch is critical |
