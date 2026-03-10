package remote

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
)

// LatestCommitResolver can resolve the latest commit SHA for a template
// reference without performing a full clone.
type LatestCommitResolver interface {
	ResolveLatestCommit(ctx context.Context, ref *Reference) (string, error)
}

// ResolveLatestCommit resolves the latest commit SHA for the given reference
// using git ls-remote. It does not clone the repository.
//
// Resolution precedence: exact tag match > exact branch match > HEAD (when ref
// has no version). Annotated tags are peeled to their target commit.
func (f *GitFetcher) ResolveLatestCommit(ctx context.Context, ref *Reference) (string, error) {
	if ref == nil {
		return "", errors.New("nil reference")
	}

	url := ref.URL
	if url == "" {
		return "", fmt.Errorf("empty URL in reference %q", ref.Original)
	}

	sha, err := f.lsRemoteResolve(ctx, ref, url)
	if err != nil && canFallbackToSSH(ref) {
		sshURL := fmt.Sprintf("git@%s:%s/%s.git", ref.Host, ref.Owner, ref.Repo)
		sshRef := &Reference{URL: sshURL, Provider: ref.Provider, Version: ref.Version}
		sha, err = f.lsRemoteResolve(ctx, sshRef, sshURL)
	}
	return sha, err
}

// lsRemoteResolve performs the actual ls-remote and resolves the ref version.
func (f *GitFetcher) lsRemoteResolve(ctx context.Context, ref *Reference, url string) (string, error) {
	remote := git.NewRemote(memory.NewStorage(), &gitconfig.RemoteConfig{
		Name: "origin",
		URLs: []string{url},
	})

	listOpts := &git.ListOptions{}
	if auth, err := f.auth.GitAuth(ref); err == nil {
		listOpts.Auth = auth
	}

	refs, err := remote.ListContext(ctx, listOpts)
	if err != nil {
		return "", fmt.Errorf("ls-remote %s: %s", url, sanitizeErrorMessage(err.Error()))
	}

	version := ref.Version
	if version == "" {
		return resolveHEADFromRefs(refs)
	}

	// Build lookup maps: tag refs and branch refs, peeling annotated tags.
	tagCommits := make(map[string]string)    // tag name → commit SHA
	branchCommits := make(map[string]string) // branch name → commit SHA
	peeledTags := make(map[string]string)    // tag name → peeled commit SHA

	for _, r := range refs {
		name := r.Name().String()

		switch {
		case strings.HasPrefix(name, "refs/tags/"):
			tagName := strings.TrimPrefix(name, "refs/tags/")
			// Peeled refs (annotated tags) end with ^{}
			if base, found := strings.CutSuffix(tagName, "^{}"); found {
				peeledTags[base] = r.Hash().String()
			} else {
				tagCommits[tagName] = r.Hash().String()
			}
		case strings.HasPrefix(name, "refs/heads/"):
			branchName := strings.TrimPrefix(name, "refs/heads/")
			branchCommits[branchName] = r.Hash().String()
		}
	}

	// Precedence: peeled tag > tag > branch
	if sha, ok := peeledTags[version]; ok {
		return sha, nil
	}
	if sha, ok := tagCommits[version]; ok {
		return sha, nil
	}
	if sha, ok := branchCommits[version]; ok {
		return sha, nil
	}

	return "", &FetchError{
		Ref:     ref,
		Message: fmt.Sprintf("ref %q not found (tried as tag and branch)", version),
		Err:     ErrVersionNotFound,
	}
}

// resolveHEADFromRefs finds HEAD in the ls-remote output.
func resolveHEADFromRefs(refs []*plumbing.Reference) (string, error) {
	// First pass: find HEAD directly.
	var headTarget plumbing.ReferenceName
	for _, r := range refs {
		if r.Name() == plumbing.HEAD {
			if r.Type() == plumbing.SymbolicReference {
				headTarget = r.Target()
				break
			}
			return r.Hash().String(), nil
		}
	}

	// Second pass: resolve symbolic HEAD target.
	if headTarget != "" {
		for _, r := range refs {
			if r.Name() == headTarget {
				return r.Hash().String(), nil
			}
		}
	}

	return "", errors.New("HEAD not found in remote refs")
}

// Ensure GitFetcher implements LatestCommitResolver.
var _ LatestCommitResolver = (*GitFetcher)(nil)
