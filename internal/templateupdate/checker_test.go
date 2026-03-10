package templateupdate

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/remote"
	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/types"
)

// mockResolver is a test double for LatestCommitResolver.
type mockResolver struct {
	sha string
	err error
}

func (m *mockResolver) ResolveLatestCommit(_ context.Context, _ *remote.Reference) (string, error) {
	return m.sha, m.err
}

func writeTagConfig(t *testing.T, dir string, cfg *scaffold.TagConfig) {
	t.Helper()
	data, err := json.MarshalIndent(cfg, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, types.TagConfigFile), data, 0o644))
}

func TestUT_Checker_Check_UpToDate(t *testing.T) {
	dir := t.TempDir()
	writeTagConfig(t, dir, &scaffold.TagConfig{
		SchemaVersion: 1,
		Template: &scaffold.TagTemplate{
			Source:    "gh:acme/template",
			CommitSHA: "abc123def456789012345678901234567890abcd",
		},
	})

	resolver := &mockResolver{sha: "abc123def456789012345678901234567890abcd"}
	checker := NewChecker(resolver)

	result, err := checker.Check(context.Background(), CheckOptions{ProjectDir: dir})
	require.NoError(t, err)
	assert.True(t, result.UpToDate)
	assert.Equal(t, "abc123def456789012345678901234567890abcd", result.CurrentSHA)
	assert.Equal(t, "abc123def456789012345678901234567890abcd", result.LatestSHA)
}

func TestUT_Checker_Check_OutOfDate(t *testing.T) {
	dir := t.TempDir()
	writeTagConfig(t, dir, &scaffold.TagConfig{
		SchemaVersion: 1,
		Template: &scaffold.TagTemplate{
			Source:    "gh:acme/template",
			CommitSHA: "abc123def456789012345678901234567890abcd",
		},
	})

	resolver := &mockResolver{sha: "def456789012345678901234567890abcdef12345"}
	checker := NewChecker(resolver)

	result, err := checker.Check(context.Background(), CheckOptions{ProjectDir: dir})
	require.NoError(t, err)
	assert.False(t, result.UpToDate)
	assert.Equal(t, "abc123def456789012345678901234567890abcd", result.CurrentSHA)
	assert.Equal(t, "def456789012345678901234567890abcdef12345", result.LatestSHA)
}

func TestUT_Checker_Check_MissingConfig(t *testing.T) {
	dir := t.TempDir()
	resolver := &mockResolver{sha: "abc123"}
	checker := NewChecker(resolver)

	_, err := checker.Check(context.Background(), CheckOptions{ProjectDir: dir})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load project config")
}

func TestUT_Checker_Check_NoCommitSHA(t *testing.T) {
	dir := t.TempDir()
	writeTagConfig(t, dir, &scaffold.TagConfig{
		SchemaVersion: 1,
		Template: &scaffold.TagTemplate{
			Source: "gh:acme/template",
			// No CommitSHA
		},
	})

	resolver := &mockResolver{sha: "abc123"}
	checker := NewChecker(resolver)

	_, err := checker.Check(context.Background(), CheckOptions{ProjectDir: dir})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no commit SHA")
}

func TestUT_Checker_Check_NoTemplate(t *testing.T) {
	dir := t.TempDir()
	writeTagConfig(t, dir, &scaffold.TagConfig{
		SchemaVersion: 1,
		// No Template
	})

	resolver := &mockResolver{sha: "abc123"}
	checker := NewChecker(resolver)

	_, err := checker.Check(context.Background(), CheckOptions{ProjectDir: dir})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no template metadata")
}

func TestUT_Checker_Check_ResolverError(t *testing.T) {
	dir := t.TempDir()
	writeTagConfig(t, dir, &scaffold.TagConfig{
		SchemaVersion: 1,
		Template: &scaffold.TagTemplate{
			Source:    "gh:acme/template",
			CommitSHA: "abc123def456789012345678901234567890abcd",
		},
	})

	resolver := &mockResolver{err: assert.AnError}
	checker := NewChecker(resolver)

	_, err := checker.Check(context.Background(), CheckOptions{ProjectDir: dir})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve latest commit")
}

func TestUT_Checker_Check_RefOverride(t *testing.T) {
	dir := t.TempDir()
	writeTagConfig(t, dir, &scaffold.TagConfig{
		SchemaVersion: 1,
		Template: &scaffold.TagTemplate{
			Source:    "gh:acme/template",
			CommitSHA: "abc123def456789012345678901234567890abcd",
			Ref:       "main",
		},
	})

	// Resolver always returns different SHA to verify the ref override is used.
	resolver := &mockResolver{sha: "different_sha_for_different_ref_1234567"}
	checker := NewChecker(resolver)

	result, err := checker.Check(context.Background(), CheckOptions{
		ProjectDir: dir,
		Ref:        "v2.0",
	})
	require.NoError(t, err)
	assert.False(t, result.UpToDate)
}
