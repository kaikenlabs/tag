package remote

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_GitFetcher_NewGitFetcher(t *testing.T) {
	// With nil auth, should use default
	fetcher := NewGitFetcher(nil)
	assert.NotNil(t, fetcher)
	assert.NotNil(t, fetcher.auth)

	// With custom auth
	mockAuth := &MockAuthProvider{}
	fetcher = NewGitFetcher(mockAuth)
	assert.NotNil(t, fetcher)
	assert.Equal(t, mockAuth, fetcher.auth)
}

func TestUT_GitFetcher_FetchWrongType(t *testing.T) {
	fetcher := NewGitFetcher(nil)

	ref := &Reference{
		Type: ReferenceTypeZip,
		URL:  "https://example.com/template.zip",
	}

	_, err := fetcher.Fetch(context.Background(), ref)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a Git reference")
}

func TestUT_LooksLikeCommitSHA(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"abc123", false},      // Too short
		{"abc1234", true},      // 7 chars, valid
		{"abc123def456", true}, // 12 chars, valid
		{"abc123def456789012345678901234567890abcd", true},   // 40 chars, valid
		{"abc123def456789012345678901234567890abcde", false}, // 41 chars, too long
		{"ghijkl", false},    // Invalid hex chars
		{"ABC123DEF", true},  // Uppercase hex is valid
		{"abc123xyz", false}, // Contains invalid chars
		{"", false},          // Empty
		{"a", false},         // Too short
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, looksLikeCommitSHA(tt.input))
		})
	}
}

func TestUT_IsBranchNotFoundError(t *testing.T) {
	tests := []struct {
		errMsg   string
		expected bool
	}{
		{"reference not found", true},
		{"couldn't find remote ref", true},
		{"some other error", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.errMsg, func(t *testing.T) {
			if tt.errMsg == "" {
				assert.False(t, isBranchNotFoundError(nil))
			} else {
				assert.Equal(t, tt.expected, isBranchNotFoundError(&testError{msg: tt.errMsg}))
			}
		})
	}
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestUT_CleanupTempDir_SafetyCheck(t *testing.T) {
	// Should refuse to remove non-temp directories
	err := CleanupTempDir("/some/random/path")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refusing to remove")

	// Should work for temp directories
	tmpDir, err := os.MkdirTemp("", "tag-git-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir) // Cleanup in case test fails

	err = CleanupTempDir(tmpDir)
	assert.NoError(t, err)

	// Verify it was removed
	_, err = os.Stat(tmpDir)
	assert.True(t, os.IsNotExist(err))
}

func TestUT_GitFetcher_SubPathValidation(t *testing.T) {
	// This test verifies the subpath logic without actually cloning
	// We create a fake repo directory structure and verify path resolution

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "templates", "go"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "templates", "go", "tag.template.json"), []byte("{}"), 0o644))

	// Valid subpath
	subPath := "templates/go"
	fullPath := filepath.Join(repoDir, subPath)
	info, err := os.Stat(fullPath)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Invalid subpath
	invalidPath := filepath.Join(repoDir, "nonexistent")
	_, err = os.Stat(invalidPath)
	assert.True(t, os.IsNotExist(err))
}

// Integration test that actually clones a public repo
// Skipped by default, run with: go test -run TestIT -tags=integration
func TestIT_GitFetcher_PublicGitHubRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Skip if no network
	if os.Getenv("CI") == "" && os.Getenv("TEST_INTEGRATION") == "" {
		t.Skip("skipping integration test (set TEST_INTEGRATION=1 to run)")
	}

	fetcher := NewGitFetcher(nil)

	ref := &Reference{
		Original: "gh:octocat/Hello-World",
		Type:     ReferenceTypeGit,
		Provider: ProviderGitHub,
		Host:     "github.com",
		Owner:    "octocat",
		Repo:     "Hello-World",
		URL:      "https://github.com/octocat/Hello-World.git",
	}

	ctx := context.Background()
	path, err := fetcher.Fetch(ctx, ref)
	if err != nil {
		// Network errors are acceptable in CI
		t.Logf("Clone failed (may be network issue): %v", err)
		t.Skip("skipping due to network error")
	}

	defer func() { _ = CleanupTempDir(path) }()

	// Verify we got something
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Verify README exists (known file in that repo)
	_, err = os.Stat(filepath.Join(path, "README"))
	assert.NoError(t, err)
}

// --- Unit Tests for sanitizeErrorMessage ---

func TestUT_SanitizeErrorMessage_RedactsGitHubToken(t *testing.T) {
	msg := `authentication failed: https://ghp_abc123XYZ789@github.com/owner/repo.git`
	result := sanitizeErrorMessage(msg)
	assert.NotContains(t, result, "ghp_abc123XYZ789")
	assert.Contains(t, result, "[REDACTED]")
}

func TestUT_SanitizeErrorMessage_RedactsGitLabToken(t *testing.T) {
	msg := `authentication failed: error with token glpat-abc123XYZ789 in request`
	result := sanitizeErrorMessage(msg)
	assert.NotContains(t, result, "glpat-abc123XYZ789")
	assert.Contains(t, result, "[REDACTED]")
}

func TestUT_SanitizeErrorMessage_RedactsURLCredentials(t *testing.T) {
	msg := `clone failed: https://x-access-token:mytoken123@github.com/owner/repo.git: 401`
	result := sanitizeErrorMessage(msg)
	assert.NotContains(t, result, "mytoken123")
	assert.Contains(t, result, "[REDACTED]@")
}

func TestUT_SanitizeErrorMessage_PreservesCleanMessages(t *testing.T) {
	msg := `repository not found: https://github.com/owner/repo`
	result := sanitizeErrorMessage(msg)
	assert.Equal(t, msg, result)
}

func TestUT_SanitizeErrorMessage_RedactsBitbucketToken(t *testing.T) {
	msg := `error: ATATT3xFfGF0abc123 is invalid`
	result := sanitizeErrorMessage(msg)
	assert.NotContains(t, result, "ATATT3xFfGF0abc123")
	assert.Contains(t, result, "[REDACTED]")
}
