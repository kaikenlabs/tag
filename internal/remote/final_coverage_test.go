package remote

import (
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// git.go — coverage for CleanupTempDir, sanitizeErrorMessage edge cases,
// FetchAtCommit validation, NewGitFetcher
// ===========================================================================

func TestUT_NewGitFetcher_NilAuth(t *testing.T) {
	t.Parallel()

	fetcher := NewGitFetcher(nil)
	require.NotNil(t, fetcher)
	require.NotNil(t, fetcher.auth)
}

func TestUT_NewGitFetcher_WithAuth(t *testing.T) {
	t.Parallel()

	auth := NewEnvAuthProvider()
	fetcher := NewGitFetcher(auth)
	require.NotNil(t, fetcher)
	assert.Equal(t, auth, fetcher.auth)
}

func TestUT_CleanupTempDir_NonTempRejects(t *testing.T) {
	t.Parallel()

	err := CleanupTempDir("/home/user/my-project")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refusing to remove")
}

func TestUT_CleanupTempDir_GitPattern_NonExistent(t *testing.T) {
	t.Parallel()

	err := CleanupTempDir("/nonexistent/tag-git-abc123")
	assert.NoError(t, err)
}

func TestUT_FetchAtCommit_ShortSHA(t *testing.T) {
	t.Parallel()

	fetcher := NewGitFetcher(nil)
	_, err := fetcher.FetchAtCommit(nil, "https://github.com/user/repo.git", "abc1234", t.TempDir()) //nolint:staticcheck // nil context is fine for validation-only test
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be full 40-character hex")
}

func TestUT_FetchAtCommit_InvalidHexSHA(t *testing.T) {
	t.Parallel()

	fetcher := NewGitFetcher(nil)
	_, err := fetcher.FetchAtCommit(nil, "https://github.com/user/repo.git", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz", t.TempDir()) //nolint:staticcheck // nil context is fine for validation-only test
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be full 40-character hex")
}

func TestUT_SanitizeErrorMessage_GitlabToken(t *testing.T) {
	t.Parallel()

	msg := "clone failed with glpat-secrettoken123 auth"
	result := sanitizeErrorMessage(msg)
	assert.NotContains(t, result, "glpat-secrettoken123")
	assert.Contains(t, result, "[REDACTED]")
}

func TestUT_SanitizeErrorMessage_ATATTToken(t *testing.T) {
	t.Parallel()

	msg := "error with ATATTxyz123 in text"
	result := sanitizeErrorMessage(msg)
	assert.NotContains(t, result, "ATATTxyz123")
	assert.Contains(t, result, "[REDACTED]")
}

func TestUT_SanitizeErrorMessage_URLWithoutColon(t *testing.T) {
	t.Parallel()

	// URL without user:pass pattern — should not be redacted
	msg := "error: https://user@github.com/repo.git"
	result := sanitizeErrorMessage(msg)
	assert.Contains(t, result, "user@github.com")
}

func TestUT_SanitizeErrorMessage_URLWithCredentials(t *testing.T) {
	t.Parallel()

	msg := "clone failed: https://user:secretpass@github.com/repo.git"
	result := sanitizeErrorMessage(msg)
	assert.NotContains(t, result, "secretpass")
	assert.Contains(t, result, "[REDACTED]@")
}

func TestUT_LooksLikeCommitSHA_EmptyString(t *testing.T) {
	t.Parallel()
	assert.False(t, looksLikeCommitSHA(""))
}

func TestUT_LooksLikeCommitSHA_TooLong(t *testing.T) {
	t.Parallel()
	assert.False(t, looksLikeCommitSHA("abcdef1234567890abcdef1234567890abcdef123X"))
}

func TestUT_LooksLikeCommitSHA_UpperCase(t *testing.T) {
	t.Parallel()
	assert.True(t, looksLikeCommitSHA("ABCDEF1234567"))
}

func TestUT_IsBranchNotFoundError_NilError(t *testing.T) {
	t.Parallel()
	assert.False(t, isBranchNotFoundError(nil))
}

func TestUT_IsAuthError_NilError(t *testing.T) {
	t.Parallel()
	assert.False(t, isAuthError(nil))
}

func TestUT_CanFallbackToSSH_AllEmpty(t *testing.T) {
	t.Parallel()

	ref := &Reference{}
	assert.False(t, canFallbackToSSH(ref))
}

// ===========================================================================
// resolve.go — coverage for resolveHEADFromRefs
// ===========================================================================

func TestUT_ResolveHEADFromRefs_EmptyRefs(t *testing.T) {
	t.Parallel()

	_, err := resolveHEADFromRefs(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HEAD not found")
}

func TestUT_ResolveHEADFromRefs_DirectHashRef(t *testing.T) {
	t.Parallel()

	hash := plumbing.NewHash("abcdef1234567890abcdef1234567890abcdef12")
	refs := []*plumbing.Reference{
		plumbing.NewHashReference(plumbing.HEAD, hash),
	}

	sha, err := resolveHEADFromRefs(refs)
	require.NoError(t, err)
	assert.Equal(t, hash.String(), sha)
}

func TestUT_ResolveHEADFromRefs_SymbolicRef(t *testing.T) {
	t.Parallel()

	hash := plumbing.NewHash("1234567890abcdef1234567890abcdef12345678")
	mainRef := plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), hash)
	headRef := plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName("main"))

	refs := []*plumbing.Reference{headRef, mainRef}

	sha, err := resolveHEADFromRefs(refs)
	require.NoError(t, err)
	assert.Equal(t, hash.String(), sha)
}

func TestUT_ResolveHEADFromRefs_SymbolicUnresolvable(t *testing.T) {
	t.Parallel()

	// HEAD points to a branch that doesn't exist in the refs
	headRef := plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName("nonexistent"))

	refs := []*plumbing.Reference{headRef}

	_, err := resolveHEADFromRefs(refs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HEAD not found")
}

func TestUT_WrapCloneError_SanitizesCredentials(t *testing.T) {
	t.Parallel()

	fetcher := NewGitFetcher(nil)
	ref := &Reference{Provider: ProviderGitHub}

	// Error containing a token should be redacted
	err := fetcher.wrapCloneError(ref, &testError{msg: "clone error: ghp_mysecrettoken123"})
	assert.NotContains(t, err.Error(), "ghp_mysecrettoken123")
}

func TestUT_ProgressWriter_WithWriter(t *testing.T) {
	t.Parallel()

	fetcher := NewGitFetcher(nil)
	w := fetcher.progressWriter()
	assert.NotNil(t, w)
}
