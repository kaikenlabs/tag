package remote

import (
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_WrapCloneError_403Error(t *testing.T) {
	t.Parallel()
	fetcher := NewGitFetcher(nil)
	ref := &Reference{
		Provider: ProviderGitLab,
		URL:      "https://gitlab.com/org/repo.git",
		Original: "gl:org/repo",
	}

	err := fetcher.wrapCloneError(ref, &testError{msg: "403 forbidden"})
	var authErr *AuthError
	assert.ErrorAs(t, err, &authErr)
	assert.Equal(t, ProviderGitLab, authErr.Provider)
	assert.Equal(t, "repository access denied", authErr.Message)
}

func TestUT_WrapCloneError_404NotFound(t *testing.T) {
	t.Parallel()
	fetcher := NewGitFetcher(nil)
	ref := &Reference{
		Provider: ProviderBitbucket,
		URL:      "https://bitbucket.org/team/repo.git",
		Original: "bb:team/repo",
	}

	err := fetcher.wrapCloneError(ref, &testError{msg: "404 not found"})
	var fetchErr *FetchError
	assert.ErrorAs(t, err, &fetchErr)
	assert.ErrorIs(t, err, ErrNotFound)
	assert.Equal(t, "repository not found", fetchErr.Message)
}

func TestUT_IsAuthError_Variants(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil", nil, false},
		{"auth keyword", &testError{msg: "authentication required"}, true},
		{"401 in message", &testError{msg: "HTTP 401 Unauthorized"}, true},
		{"403 in message", &testError{msg: "HTTP 403 Forbidden"}, true},
		{"timeout", &testError{msg: "connection timed out"}, false},
		{"dns failure", &testError{msg: "no such host"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, isAuthError(tt.err))
		})
	}
}

func TestUT_CanFallbackToSSH_SSHAlreadyURL(t *testing.T) {
	t.Parallel()
	ref := &Reference{
		Host:  "github.com",
		Owner: "user",
		Repo:  "repo",
		URL:   "git@github.com:user/repo.git",
	}
	assert.False(t, canFallbackToSSH(ref), "should not fallback when URL is already SSH")
}

func TestUT_SanitizeErrorMessage_URLWithUserPass(t *testing.T) {
	t.Parallel()
	msg := "error: https://user:secretpass123@github.com/org/repo.git returned 401"
	result := sanitizeErrorMessage(msg)
	assert.NotContains(t, result, "secretpass123")
	assert.NotContains(t, result, "user:secretpass123")
	assert.Contains(t, result, "[REDACTED]@")
	assert.Contains(t, result, "github.com/org/repo.git")
}

func TestUT_SanitizeErrorMessage_URLWithoutCredentials(t *testing.T) {
	t.Parallel()
	msg := "error: https://github.com/user/repo.git returned 404"
	result := sanitizeErrorMessage(msg)
	assert.Equal(t, msg, result, "message without credentials should be unchanged")
}

func TestUT_SanitizeErrorMessage_TokenAtEndOfLine(t *testing.T) {
	t.Parallel()
	msg := "failed with token ghp_abcdef123456"
	result := sanitizeErrorMessage(msg)
	assert.NotContains(t, result, "ghp_abcdef123456")
	assert.Contains(t, result, "[REDACTED]")
}

func TestUT_LooksLikeCommitSHA_BoundaryLengths(t *testing.T) {
	t.Parallel()
	assert.False(t, looksLikeCommitSHA("abcde1"), "6 chars should be too short")
	assert.True(t, looksLikeCommitSHA("abcdef1"), "7 chars is minimum")
	assert.True(t, looksLikeCommitSHA("abcdef1234567890abcdef1234567890abcdef12"), "40 chars is maximum")
	assert.False(t, looksLikeCommitSHA("abcdef1234567890abcdef1234567890abcdef123"), "41 chars is too long")
}

func TestUT_CleanupTempDir_TagGitPattern(t *testing.T) {
	t.Parallel()
	// Nonexistent path with tag-git- pattern succeeds (os.RemoveAll returns nil)
	err := CleanupTempDir("/nonexistent/tag-git-test123")
	assert.NoError(t, err)
}

func TestUT_CleanupTempDir_TagZipPattern(t *testing.T) {
	t.Parallel()
	err := CleanupTempDir("/nonexistent/tag-zip-test123")
	assert.NoError(t, err)
}

func TestUT_CleanupTempDir_RejectsArbitraryPath(t *testing.T) {
	t.Parallel()
	err := CleanupTempDir("/home/user/projects")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "refusing to remove")
}

func TestUT_ProgressWriter_NilOut(t *testing.T) {
	t.Parallel()
	fetcher := &GitFetcher{auth: NewEnvAuthProvider(), out: nil}
	w := fetcher.progressWriter()
	assert.Equal(t, io.Discard, w)
}

func TestUT_ProgressWriter_CustomWriter(t *testing.T) {
	t.Parallel()
	fetcher := NewGitFetcher(nil) // defaults to os.Stderr
	w := fetcher.progressWriter()
	assert.NotNil(t, w)
}

func TestUT_FetchError_ErrorFormatting(t *testing.T) {
	t.Parallel()
	ref := &Reference{
		Provider: ProviderGitHub,
		Owner:    "user",
		Repo:     "repo",
		Original: "gh:user/repo",
	}

	errWithInner := &FetchError{Ref: ref, Message: "clone failed", Err: errors.New("timeout")}
	assert.Contains(t, errWithInner.Error(), "clone failed")
	assert.Contains(t, errWithInner.Error(), "timeout")
	assert.Equal(t, errors.New("timeout").Error(), errWithInner.Unwrap().Error())

	errNoInner := &FetchError{Ref: ref, Message: "not found"}
	assert.Contains(t, errNoInner.Error(), "not found")
	assert.Nil(t, errNoInner.Unwrap())
}

func TestUT_CacheError_ErrorFormatting(t *testing.T) {
	t.Parallel()
	errWithInner := &CacheError{Key: "k", Op: "set", Message: "disk full", Err: errors.New("no space")}
	assert.Contains(t, errWithInner.Error(), "cache set")
	assert.Contains(t, errWithInner.Error(), "disk full")
	assert.Contains(t, errWithInner.Error(), "no space")

	errNoInner := &CacheError{Key: "k", Op: "get", Message: "corrupted"}
	assert.Contains(t, errNoInner.Error(), `cache get "k"`)
	assert.Contains(t, errNoInner.Error(), "corrupted")
	assert.Nil(t, errNoInner.Unwrap())
}

func TestUT_AuthError_ProviderHints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		provider Provider
		hint     string
	}{
		{ProviderGitHub, "GITHUB_TOKEN"},
		{ProviderGitLab, "GITLAB_TOKEN"},
		{ProviderBitbucket, "BITBUCKET_TOKEN"},
		{ProviderGeneric, ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			t.Parallel()
			err := &AuthError{Provider: tt.provider, Message: "access denied"}
			msg := err.Error()
			if tt.hint != "" {
				assert.Contains(t, msg, tt.hint)
			}
			assert.Contains(t, msg, "access denied")
		})
	}
}

func TestUT_AuthError_Is(t *testing.T) {
	t.Parallel()
	err := &AuthError{Provider: ProviderGitHub, Message: "test"}
	assert.ErrorIs(t, err, ErrAuthRequired)
}

func TestUT_AuthError_WithInnerError(t *testing.T) {
	t.Parallel()
	inner := errors.New("network failure")
	err := &AuthError{Provider: ProviderGitHub, Message: "test", Err: inner}
	assert.Contains(t, err.Error(), "network failure")
	assert.Equal(t, inner, err.Unwrap())
}

func TestUT_AuthError_WithoutInnerError(t *testing.T) {
	t.Parallel()
	err := &AuthError{Provider: ProviderGitHub, Message: "no token"}
	assert.NotContains(t, err.Error(), "<nil>")
	assert.Nil(t, err.Unwrap())
}

func TestUT_ParseError_Format(t *testing.T) {
	t.Parallel()
	err := &ParseError{Input: "bad:ref", Message: "unsupported prefix"}
	assert.Contains(t, err.Error(), "bad:ref")
	assert.Contains(t, err.Error(), "unsupported prefix")
}

func TestUT_FetchResult_Fields(t *testing.T) {
	t.Parallel()
	r := &FetchResult{
		Path:      "/tmp/tag-git-12345",
		CommitSHA: "abc123",
		Version:   "v1.0.0",
	}
	require.Equal(t, "/tmp/tag-git-12345", r.Path)
	require.Equal(t, "abc123", r.CommitSHA)
	require.Equal(t, "v1.0.0", r.Version)
}
