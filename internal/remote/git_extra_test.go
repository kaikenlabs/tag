package remote

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUT_CanFallbackToSSH(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		ref      *Reference
		expected bool
	}{
		{
			name:     "all fields present with HTTPS URL",
			ref:      &Reference{Host: "github.com", Owner: "user", Repo: "repo", URL: "https://github.com/user/repo.git"},
			expected: true,
		},
		{
			name:     "missing host",
			ref:      &Reference{Host: "", Owner: "user", Repo: "repo", URL: "https://github.com/user/repo.git"},
			expected: false,
		},
		{
			name:     "missing owner",
			ref:      &Reference{Host: "github.com", Owner: "", Repo: "repo", URL: "https://github.com/user/repo.git"},
			expected: false,
		},
		{
			name:     "missing repo",
			ref:      &Reference{Host: "github.com", Owner: "user", Repo: "", URL: "https://github.com/user/repo.git"},
			expected: false,
		},
		{
			name:     "already SSH URL",
			ref:      &Reference{Host: "github.com", Owner: "user", Repo: "repo", URL: "git@github.com:user/repo.git"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, canFallbackToSSH(tt.ref))
		})
	}
}

func TestUT_IsAuthError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"authentication error", &testError{msg: "authentication failed"}, true},
		{"401 error", &testError{msg: "server returned 401"}, true},
		{"403 error", &testError{msg: "error 403 forbidden"}, true},
		{"generic error", &testError{msg: "network timeout"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, isAuthError(tt.err))
		})
	}
}

func TestUT_SanitizeErrorMessage_MultipleTokens(t *testing.T) {
	t.Parallel()
	msg := `error with ghp_token123 and also gho_anothertoken456 in text`
	result := sanitizeErrorMessage(msg)
	assert.NotContains(t, result, "ghp_token123")
	assert.NotContains(t, result, "gho_anothertoken456")
	assert.Contains(t, result, "[REDACTED]")
}

func TestUT_SanitizeErrorMessage_NoCredentials(t *testing.T) {
	t.Parallel()
	msg := "simple error message without credentials"
	result := sanitizeErrorMessage(msg)
	assert.Equal(t, msg, result)
}

func TestUT_SanitizeErrorMessage_GHSToken(t *testing.T) {
	t.Parallel()
	msg := "auth error ghs_secretXYZ123"
	result := sanitizeErrorMessage(msg)
	assert.NotContains(t, result, "ghs_secretXYZ123")
	assert.Contains(t, result, "[REDACTED]")
}

func TestUT_SanitizeErrorMessage_GHRToken(t *testing.T) {
	t.Parallel()
	msg := "error: ghr_tokenValue789"
	result := sanitizeErrorMessage(msg)
	assert.NotContains(t, result, "ghr_tokenValue789")
	assert.Contains(t, result, "[REDACTED]")
}

func TestUT_LooksLikeCommitSHA_EdgeCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"exactly 7 chars", "abcdef1", true},
		{"exactly 40 chars", "abcdef1234567890abcdef1234567890abcdef12", true},
		{"6 chars too short", "abcde1", false},
		{"mixed case", "AbCdEf1234567", true},
		{"with g", "abcdefg", false},
		{"spaces", "abc def", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, looksLikeCommitSHA(tt.input))
		})
	}
}

func TestUT_IsBranchNotFoundError_Additional(t *testing.T) {
	t.Parallel()
	assert.False(t, isBranchNotFoundError(nil))
	assert.True(t, isBranchNotFoundError(&testError{msg: "couldn't find remote ref main"}))
	assert.True(t, isBranchNotFoundError(&testError{msg: "reference not found"}))
	assert.False(t, isBranchNotFoundError(&testError{msg: "permission denied"}))
}

func TestUT_WrapCloneError_AuthError(t *testing.T) {
	t.Parallel()
	fetcher := NewGitFetcher(nil)
	ref := &Reference{
		Provider: ProviderGitHub,
		URL:      "https://github.com/user/repo.git",
		Original: "gh:user/repo",
	}

	err := fetcher.wrapCloneError(ref, &testError{msg: "authentication required: 401"})
	var authErr *AuthError
	assert.ErrorAs(t, err, &authErr)
	assert.Equal(t, ProviderGitHub, authErr.Provider)
}

func TestUT_WrapCloneError_NotFoundError(t *testing.T) {
	t.Parallel()
	fetcher := NewGitFetcher(nil)
	ref := &Reference{
		Provider: ProviderGitHub,
		URL:      "https://github.com/user/repo.git",
		Original: "gh:user/repo",
	}

	err := fetcher.wrapCloneError(ref, &testError{msg: "repository not found: 404"})
	var fetchErr *FetchError
	assert.ErrorAs(t, err, &fetchErr)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestUT_WrapCloneError_GenericError(t *testing.T) {
	t.Parallel()
	fetcher := NewGitFetcher(nil)
	ref := &Reference{
		Provider: ProviderGitHub,
		URL:      "https://github.com/user/repo.git",
		Original: "gh:user/repo",
	}

	err := fetcher.wrapCloneError(ref, &testError{msg: "network timeout"})
	var fetchErr *FetchError
	assert.ErrorAs(t, err, &fetchErr)
	assert.Equal(t, "clone failed", fetchErr.Message)
}

func TestUT_ProgressWriter_NonNil(t *testing.T) {
	t.Parallel()
	fetcher := NewGitFetcher(nil)
	w := fetcher.progressWriter()
	assert.NotNil(t, w)
}

func TestUT_CleanupTempDir_ZipPattern(t *testing.T) {
	t.Parallel()
	// tag-zip- pattern should also be accepted (os.RemoveAll returns nil for nonexistent)
	err := CleanupTempDir("/nonexistent/tag-zip-12345")
	assert.NoError(t, err)
}
