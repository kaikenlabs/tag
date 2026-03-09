package remote

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"

	"github.com/kaikenlabs/tag/internal/types"
)

// GitFetcher fetches templates from Git repositories.
type GitFetcher struct {
	auth AuthProvider
	out  io.Writer
}

// NewGitFetcher creates a new Git fetcher with the given auth provider.
func NewGitFetcher(auth AuthProvider) *GitFetcher {
	if auth == nil {
		auth = NewEnvAuthProvider()
	}
	return &GitFetcher{auth: auth, out: os.Stderr}
}

// Fetch clones the repository and returns the path to the template along with
// the resolved commit SHA.
func (f *GitFetcher) Fetch(ctx context.Context, ref *Reference) (*FetchResult, error) {
	if ref.Type != ReferenceTypeGit {
		return nil, &FetchError{Ref: ref, Message: "not a Git reference"}
	}

	// Create temporary directory for clone
	tmpDir, err := os.MkdirTemp("", "tag-git-*")
	if err != nil {
		return nil, &FetchError{Ref: ref, Message: "cannot create temp directory", Err: err}
	}

	// Clean up on error
	success := false
	defer func() {
		if !success {
			os.RemoveAll(tmpDir)
		}
	}()

	// Clone the repository (falls back to SSH on HTTPS auth failure)
	repo, err := f.clone(ctx, ref, tmpDir)
	if err != nil {
		return nil, err
	}

	// Resolve commit SHA
	var commitHash plumbing.Hash
	if ref.Version != "" {
		// Checkout specific version and get its resolved hash
		commitHash, err = f.checkout(repo, ref)
		if err != nil {
			return nil, err
		}
	} else {
		// No version specified — resolve HEAD of the default branch
		commitHash, err = resolveHEAD(repo)
		if err != nil {
			return nil, &FetchError{Ref: ref, Message: "cannot resolve HEAD", Err: err}
		}
	}

	// Determine the result path
	resultPath := tmpDir
	if ref.SubPath != "" {
		resultPath = filepath.Join(tmpDir, ref.SubPath)
		// Verify subpath exists
		info, err := os.Stat(resultPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, &FetchError{
					Ref:     ref,
					Message: fmt.Sprintf("subpath %q not found in repository", ref.SubPath),
					Err:     ErrSubPathNotFound,
				}
			}
			return nil, &FetchError{Ref: ref, Message: "cannot access subpath", Err: err}
		}
		if !info.IsDir() {
			return nil, &FetchError{
				Ref:     ref,
				Message: fmt.Sprintf("subpath %q is not a directory", ref.SubPath),
			}
		}
	}

	success = true
	return &FetchResult{
		Path:      resultPath,
		CommitSHA: commitHash.String(),
		Version:   ref.Version,
	}, nil
}

// FetchAtCommit clones a repository and checks out a specific commit SHA.
// Unlike Fetch, this performs a full clone (not shallow) since the target commit
// may not be reachable from advertised refs in a shallow clone.
func (f *GitFetcher) FetchAtCommit(ctx context.Context, url, commitSHA, destDir string) (string, error) {
	if !looksLikeCommitSHA(commitSHA) || len(commitSHA) != 40 {
		return "", fmt.Errorf("invalid commit SHA (must be full 40-character hex): %q", commitSHA)
	}

	// Full clone (no depth limit) to ensure the commit is available
	opts := &git.CloneOptions{
		URL:      url,
		Progress: f.progressWriter(),
	}

	// Get auth if possible (best-effort, URL-based ref)
	ref := &Reference{URL: url, Type: ReferenceTypeGit}
	if auth, err := f.auth.GitAuth(ref); err == nil {
		opts.Auth = auth
	}

	repo, err := git.PlainCloneContext(ctx, destDir, false, opts)
	if err != nil {
		return "", fmt.Errorf("clone for commit checkout: %s", sanitizeErrorMessage(err.Error()))
	}

	wt, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("get worktree: %w", err)
	}

	hash := plumbing.NewHash(commitSHA)
	if err := wt.Checkout(&git.CheckoutOptions{Hash: hash, Force: true}); err != nil {
		return "", fmt.Errorf("checkout commit %s: %w", commitSHA, err)
	}

	return destDir, nil
}

// clone performs the Git clone operation.
// If HTTPS auth fails and the ref has a known provider with SSH support,
// it falls back to cloning via SSH (git@host:owner/repo.git).
func (f *GitFetcher) clone(ctx context.Context, ref *Reference, destDir string) (*git.Repository, error) {
	// Get auth if available
	auth, err := f.auth.GitAuth(ref)
	if err != nil {
		return nil, &FetchError{Ref: ref, Message: "auth setup failed", Err: err}
	}

	repo, err := f.doClone(ctx, ref.URL, ref, destDir, auth)
	if err != nil && isAuthError(err) && canFallbackToSSH(ref) {
		// HTTPS auth failed — retry with SSH
		sshURL := fmt.Sprintf("git@%s:%s/%s.git", ref.Host, ref.Owner, ref.Repo)
		fallbackRef := &Reference{URL: sshURL, Provider: ref.Provider}
		sshAuth, sshErr := f.auth.GitAuth(fallbackRef)
		if sshErr == nil {
			// Clean destDir for retry (clone requires empty directory)
			os.RemoveAll(destDir)
			if mkErr := os.MkdirAll(destDir, types.DirMode); mkErr != nil {
				return nil, &FetchError{Ref: ref, Message: "cannot recreate temp directory", Err: mkErr}
			}

			// Build full ref for clone with version/subpath info
			cloneRef := *ref
			cloneRef.URL = sshURL
			repo, err = f.doClone(ctx, sshURL, &cloneRef, destDir, sshAuth)
		}
	}
	if err != nil {
		return nil, f.wrapCloneError(ref, err)
	}
	return repo, nil
}

// doClone performs the actual git clone with the given URL and auth.
func (f *GitFetcher) doClone(ctx context.Context, url string, ref *Reference, destDir string, auth transport.AuthMethod) (*git.Repository, error) {
	opts := &git.CloneOptions{
		URL:      url,
		Depth:    1, // Shallow clone for efficiency
		Auth:     auth,
		Progress: f.progressWriter(),
	}

	// If we have a specific version, we might need to fetch it
	// For tags/branches, we can set the reference name
	if ref.Version != "" {
		// Try as a branch first (most common for "main", "master", etc.)
		opts.ReferenceName = plumbing.NewBranchReferenceName(ref.Version)
		opts.SingleBranch = true
	}

	repo, err := git.PlainCloneContext(ctx, destDir, false, opts)
	if err != nil {
		// If branch failed, try without specific reference
		// (we'll checkout the tag/commit later)
		if ref.Version != "" && isBranchNotFoundError(err) {
			opts.ReferenceName = ""
			opts.SingleBranch = false
			repo, err = git.PlainCloneContext(ctx, destDir, false, opts)
		}
	}
	return repo, err
}

// canFallbackToSSH returns true if the reference has enough info to construct an SSH URL.
func canFallbackToSSH(ref *Reference) bool {
	return ref.Host != "" && ref.Owner != "" && ref.Repo != "" && !isSSHURL(ref.URL)
}

// isAuthError checks if an error is an authentication failure.
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "authentication") ||
		strings.Contains(errStr, "401") ||
		strings.Contains(errStr, "403")
}

// checkout checks out a specific version (tag, branch, or commit) and returns
// the resolved commit hash.
func (f *GitFetcher) checkout(repo *git.Repository, ref *Reference) (plumbing.Hash, error) {
	wt, err := repo.Worktree()
	if err != nil {
		return plumbing.ZeroHash, &FetchError{Ref: ref, Message: "cannot get worktree", Err: err}
	}

	// Try different reference types in order:
	// 1. Tag
	// 2. Branch
	// 3. Commit SHA

	// Try as tag
	tagRef := plumbing.NewTagReferenceName(ref.Version)
	if _, err := repo.Reference(tagRef, true); err == nil { //nolint:govet // shadow in if-init is idiomatic
		err = wt.Checkout(&git.CheckoutOptions{
			Branch: tagRef,
			Force:  true,
		})
		if err == nil {
			return resolveHEAD(repo)
		}
	}

	// Try as remote branch
	branchRef := plumbing.NewRemoteReferenceName("origin", ref.Version)
	if _, err := repo.Reference(branchRef, true); err == nil { //nolint:govet // shadow in if-init is idiomatic
		err = wt.Checkout(&git.CheckoutOptions{
			Branch: branchRef,
			Force:  true,
		})
		if err == nil {
			return resolveHEAD(repo)
		}
	}

	// Try as commit SHA
	if looksLikeCommitSHA(ref.Version) {
		hash := plumbing.NewHash(ref.Version)
		err = wt.Checkout(&git.CheckoutOptions{
			Hash:  hash,
			Force: true,
		})
		if err == nil {
			return hash, nil
		}
	}

	return plumbing.ZeroHash, &FetchError{
		Ref:     ref,
		Message: fmt.Sprintf("version %q not found (tried as tag, branch, and commit)", ref.Version),
		Err:     ErrVersionNotFound,
	}
}

// resolveHEAD returns the commit hash that HEAD currently points to.
func resolveHEAD(repo *git.Repository) (plumbing.Hash, error) {
	head, err := repo.Head()
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("resolve HEAD: %w", err)
	}
	return head.Hash(), nil
}

// wrapCloneError wraps clone errors with helpful messages.
// Error messages are sanitized to prevent leaking credential fragments.
func (f *GitFetcher) wrapCloneError(ref *Reference, err error) error {
	errStr := sanitizeErrorMessage(err.Error())

	// Check for auth errors
	if strings.Contains(errStr, "authentication") ||
		strings.Contains(errStr, "401") ||
		strings.Contains(errStr, "403") {
		return &AuthError{
			Provider: ref.Provider,
			Message:  "repository access denied",
			Err:      errors.New(errStr),
		}
	}

	// Check for not found
	if strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "404") ||
		strings.Contains(errStr, "repository not found") {
		return &FetchError{
			Ref:     ref,
			Message: "repository not found",
			Err:     ErrNotFound,
		}
	}

	return &FetchError{Ref: ref, Message: "clone failed", Err: errors.New(errStr)}
}

// progressWriter returns the configured output writer, defaulting to io.Discard.
func (f *GitFetcher) progressWriter() io.Writer {
	if f.out != nil {
		return f.out
	}
	return io.Discard
}

// sanitizeErrorMessage strips potential credential fragments from error messages.
func sanitizeErrorMessage(msg string) string {
	// Redact any token-like strings (long alphanumeric sequences that look like tokens)
	// Common token patterns: ghp_*, glpat-*, ATATT*, etc.
	tokenPrefixes := []string{"ghp_", "gho_", "ghs_", "ghr_", "glpat-", "ATATT"}
	for _, prefix := range tokenPrefixes {
		if idx := strings.Index(msg, prefix); idx >= 0 {
			// Find the end of the token (tokens are usually alphanumeric+special)
			end := idx + len(prefix)
			for end < len(msg) && msg[end] != ' ' && msg[end] != '"' && msg[end] != '\'' && msg[end] != '@' {
				end++
			}
			msg = msg[:idx] + "[REDACTED]" + msg[end:]
		}
	}

	// Redact URLs that contain credentials (user:pass@host format)
	if idx := strings.Index(msg, "://"); idx >= 0 {
		// Look for user:pass@ pattern after ://
		rest := msg[idx+3:]
		if beforeAt, afterAt, found := strings.Cut(rest, "@"); found {
			// Check if there's a colon before the @ (indicating user:pass)
			if strings.Contains(beforeAt, ":") {
				msg = msg[:idx+3] + "[REDACTED]@" + afterAt
			}
		}
	}

	return msg
}

// isBranchNotFoundError checks if the error is due to a branch not being found.
func isBranchNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "reference not found") ||
		strings.Contains(errStr, "couldn't find remote ref")
}

// looksLikeCommitSHA checks if the string looks like a commit SHA.
func looksLikeCommitSHA(s string) bool {
	if len(s) < 7 || len(s) > 40 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// FetchResult contains the result of a fetch operation.
type FetchResult struct {
	Path      string // Local filesystem path to template
	CommitSHA string // Resolved git commit SHA (empty for zip/local)
	Version   string // Tag/branch/commit ref used
}

// Fetcher is the interface for all fetchers.
type Fetcher interface {
	Fetch(ctx context.Context, ref *Reference) (*FetchResult, error)
}

// Ensure GitFetcher implements Fetcher.
var _ Fetcher = (*GitFetcher)(nil)

// CleanupTempDir removes a temporary directory created by Fetch.
// This should be called after the template has been cached.
func CleanupTempDir(path string) error {
	// Safety check: only remove paths that look like temp directories
	if !strings.Contains(path, "tag-git-") && !strings.Contains(path, "tag-zip-") {
		return errors.New("refusing to remove non-temp directory")
	}
	return os.RemoveAll(path)
}
