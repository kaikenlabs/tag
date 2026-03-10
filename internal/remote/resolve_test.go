package remote

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_ResolveLatestCommit_NilReference(t *testing.T) {
	fetcher := NewGitFetcher(NewEnvAuthProvider())
	_, err := fetcher.ResolveLatestCommit(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil reference")
}

func TestUT_ResolveLatestCommit_EmptyURL(t *testing.T) {
	fetcher := NewGitFetcher(NewEnvAuthProvider())
	ref := &Reference{Original: "test", URL: ""}
	_, err := fetcher.ResolveLatestCommit(context.Background(), ref)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty URL")
}

func TestUT_ResolveLatestCommit_InvalidHost(t *testing.T) {
	fetcher := NewGitFetcher(NewEnvAuthProvider())
	ref := &Reference{
		Original: "invalid",
		URL:      "https://invalid-host-for-test-12345.example/repo.git",
		Version:  "main",
	}
	_, err := fetcher.ResolveLatestCommit(context.Background(), ref)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ls-remote")
}

func TestUT_ResolveHEADFromRefs_Empty(t *testing.T) {
	_, err := resolveHEADFromRefs(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HEAD not found")
}

func TestUT_LatestCommitResolver_Interface(t *testing.T) {
	// Verify GitFetcher satisfies LatestCommitResolver at compile time.
	var _ LatestCommitResolver = (*GitFetcher)(nil)
}
