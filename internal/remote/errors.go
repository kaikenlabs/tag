package remote

import (
	"errors"
	"fmt"
)

// Sentinel errors for remote operations.
var (
	// ErrNotCached indicates the template is not in the cache.
	ErrNotCached = errors.New("template not cached")

	// ErrAuthRequired indicates authentication is required but not provided.
	ErrAuthRequired = errors.New("authentication required")

	// ErrNotFound indicates the remote resource was not found.
	ErrNotFound = errors.New("resource not found")

	// ErrVersionNotFound indicates the specified version was not found.
	ErrVersionNotFound = errors.New("version not found")

	// ErrSubPathNotFound indicates the subpath doesn't exist in the template.
	ErrSubPathNotFound = errors.New("subpath not found")
)

// ParseError represents an error parsing a template reference.
type ParseError struct {
	Input   string
	Message string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("invalid reference %q: %s", e.Input, e.Message)
}

// FetchError represents an error fetching a remote template.
type FetchError struct {
	Ref     *Reference
	Message string
	Err     error
}

func (e *FetchError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("fetch %s: %s: %v", e.Ref.String(), e.Message, e.Err)
	}
	return fmt.Sprintf("fetch %s: %s", e.Ref.String(), e.Message)
}

func (e *FetchError) Unwrap() error {
	return e.Err
}

// CacheError represents an error with the cache.
type CacheError struct {
	Key     string
	Op      string // "get", "set", "invalidate"
	Message string
	Err     error
}

func (e *CacheError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("cache %s %q: %s: %v", e.Op, e.Key, e.Message, e.Err)
	}
	return fmt.Sprintf("cache %s %q: %s", e.Op, e.Key, e.Message)
}

func (e *CacheError) Unwrap() error {
	return e.Err
}

// AuthError represents an authentication error.
type AuthError struct {
	Provider Provider
	Message  string
	Err      error
}

func (e *AuthError) Error() string {
	hint := ""
	switch e.Provider {
	case ProviderGitHub:
		hint = " (hint: set GITHUB_TOKEN environment variable)"
	case ProviderGitLab:
		hint = " (hint: set GITLAB_TOKEN environment variable)"
	case ProviderBitbucket:
		hint = " (hint: set BITBUCKET_TOKEN environment variable with a workspace or repository access token)"
	}

	if e.Err != nil {
		return fmt.Sprintf("authentication failed for %s: %s%s: %v", e.Provider, e.Message, hint, e.Err)
	}
	return fmt.Sprintf("authentication failed for %s: %s%s", e.Provider, e.Message, hint)
}

func (e *AuthError) Unwrap() error {
	return e.Err
}

// Is implements error matching for AuthError.
func (e *AuthError) Is(target error) bool {
	return target == ErrAuthRequired
}
