package remote

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUT_SanitizeErrorMessage_GHOToken(t *testing.T) {
	t.Parallel()
	msg := "error with gho_tokenABC123 in text"
	result := sanitizeErrorMessage(msg)
	assert.NotContains(t, result, "gho_tokenABC123")
	assert.Contains(t, result, "[REDACTED]")
}

func TestUT_SanitizeErrorMessage_URLWithPort(t *testing.T) {
	t.Parallel()
	msg := "clone failed: https://user:pass@gitlab.example.com:8443/repo.git"
	result := sanitizeErrorMessage(msg)
	assert.NotContains(t, result, "pass")
	assert.Contains(t, result, "[REDACTED]@")
}

func TestUT_SanitizeErrorMessage_URLNoCredentials(t *testing.T) {
	t.Parallel()
	msg := "clone failed: https://github.com/user/repo.git"
	result := sanitizeErrorMessage(msg)
	assert.Equal(t, msg, result)
}

func TestUT_WrapCloneError_404InMessage(t *testing.T) {
	t.Parallel()
	fetcher := NewGitFetcher(nil)
	ref := &Reference{Provider: ProviderGitLab}

	err := fetcher.wrapCloneError(ref, &testError{msg: "error 404 page not found"})
	var fetchErr *FetchError
	assert.ErrorAs(t, err, &fetchErr)
}

func TestUT_WrapCloneError_403InMessage(t *testing.T) {
	t.Parallel()
	fetcher := NewGitFetcher(nil)
	ref := &Reference{Provider: ProviderGitLab}

	err := fetcher.wrapCloneError(ref, &testError{msg: "HTTP 403 forbidden"})
	var authErr *AuthError
	assert.ErrorAs(t, err, &authErr)
}

func TestUT_ProgressWriter_NilOutField(t *testing.T) {
	t.Parallel()
	fetcher := &GitFetcher{auth: NewEnvAuthProvider(), out: nil}
	w := fetcher.progressWriter()
	assert.NotNil(t, w, "should return io.Discard when out is nil")
}
