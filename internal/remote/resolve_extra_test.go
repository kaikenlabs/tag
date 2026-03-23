package remote

import (
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_ResolveHEADFromRefs_DirectHash(t *testing.T) {
	t.Parallel()
	hash := plumbing.NewHash("abc123def456789012345678901234567890abcd")
	refs := []*plumbing.Reference{
		plumbing.NewHashReference(plumbing.HEAD, hash),
	}

	sha, err := resolveHEADFromRefs(refs)
	require.NoError(t, err)
	assert.Equal(t, hash.String(), sha)
}

func TestUT_ResolveHEADFromRefs_SymbolicReference(t *testing.T) {
	t.Parallel()
	mainHash := plumbing.NewHash("abc123def456789012345678901234567890abcd")
	refs := []*plumbing.Reference{
		plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName("main")),
		plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), mainHash),
	}

	sha, err := resolveHEADFromRefs(refs)
	require.NoError(t, err)
	assert.Equal(t, mainHash.String(), sha)
}

func TestUT_ResolveHEADFromRefs_SymbolicTargetMissing(t *testing.T) {
	t.Parallel()
	refs := []*plumbing.Reference{
		plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName("main")),
		// main branch reference is missing
	}

	_, err := resolveHEADFromRefs(refs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HEAD not found")
}

func TestUT_ResolveHEADFromRefs_NoHEAD(t *testing.T) {
	t.Parallel()
	hash := plumbing.NewHash("abc123def456789012345678901234567890abcd")
	refs := []*plumbing.Reference{
		plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), hash),
	}

	_, err := resolveHEADFromRefs(refs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HEAD not found")
}

func TestUT_LatestCommitResolver_InterfaceCheck(t *testing.T) {
	t.Parallel()
	// Compile-time interface check
	var _ LatestCommitResolver = (*GitFetcher)(nil)
}
