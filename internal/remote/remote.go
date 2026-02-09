package remote

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ResolveOptions configures the resolution behavior.
type ResolveOptions struct {
	ForceUpdate bool // Ignore cache, always fetch fresh
	Offline     bool // Only use cache, don't fetch
}

// Resolver orchestrates reference parsing, caching, and fetching.
type Resolver struct {
	cache    Cache
	auth     AuthProvider
	fetchers map[ReferenceType]Fetcher
}

// NewResolver creates a new Resolver with default configuration.
func NewResolver() (*Resolver, error) {
	return NewResolverWithOptions("", nil)
}

// NewResolverWithOptions creates a new Resolver with custom configuration.
func NewResolverWithOptions(cacheDir string, auth AuthProvider) (*Resolver, error) {
	cache, err := NewFSCache(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("create cache: %w", err)
	}

	if auth == nil {
		auth = NewEnvAuthProvider()
	}

	return &Resolver{
		cache: cache,
		auth:  auth,
		fetchers: map[ReferenceType]Fetcher{
			ReferenceTypeGit: NewGitFetcher(auth),
			ReferenceTypeZip: NewZipFetcher(),
		},
	}, nil
}

// Resolve takes a template reference string and returns a local path.
// This is the main entry point for remote template resolution.
func (r *Resolver) Resolve(ctx context.Context, input string, opts ResolveOptions) (string, error) {
	// 1. Parse the reference
	ref, err := Parse(input)
	if err != nil {
		return "", fmt.Errorf("invalid template reference: %w", err)
	}

	// 2. If local directory, just return the path
	if ref.Type == ReferenceTypeLocal && !isZipFile(ref.URL) {
		return ref.URL, nil
	}

	// For local zip files, we still need to extract them
	// but we won't cache them (they're already local)
	if ref.Type == ReferenceTypeZip && !ref.IsRemote() {
		return r.fetchAndReturn(ctx, ref)
	}

	// 3. Check cache (unless force update)
	cacheKey := ref.CacheKey()
	if !opts.ForceUpdate {
		if path, found, err := r.cache.Get(cacheKey); err == nil && found { //nolint:govet // shadow in if-init is idiomatic
			// Apply subpath to cached path
			return r.applySubPath(path, ref.SubPath)
		}
	}

	// 4. If offline mode, fail
	if opts.Offline {
		return "", &FetchError{
			Ref:     ref,
			Message: "not cached and offline mode is enabled",
			Err:     ErrNotCached,
		}
	}

	// 5. Fetch using appropriate fetcher
	fetcher, ok := r.fetchers[ref.Type]
	if !ok {
		return "", &FetchError{
			Ref:     ref,
			Message: fmt.Sprintf("unsupported reference type: %s", ref.Type),
		}
	}

	tempPath, err := fetcher.Fetch(ctx, ref)
	if err != nil {
		return "", err
	}

	// 6. Store in cache
	meta := &CacheMeta{
		OriginalRef: ref.Original,
		ResolvedURL: ref.URL,
		Version:     ref.Version,
		FetchedAt:   time.Now(),
	}

	cachedPath, err := r.cache.Set(cacheKey, tempPath, meta)
	if err != nil {
		// Log warning but continue with temp path
		// The template is still usable, just not cached
		fmt.Fprintf(os.Stderr, "Warning: could not cache template: %v\n", err)
		return tempPath, nil
	}

	// 7. Clean up temp directory (ignore error - best effort)
	_ = CleanupTempDir(tempPath)

	// 8. Return cached path (subpath is already in the cached content)
	return cachedPath, nil
}

// fetchAndReturn fetches without caching (for local zip files).
func (r *Resolver) fetchAndReturn(ctx context.Context, ref *Reference) (string, error) {
	fetcher, ok := r.fetchers[ref.Type]
	if !ok {
		return "", &FetchError{
			Ref:     ref,
			Message: fmt.Sprintf("unsupported reference type: %s", ref.Type),
		}
	}

	return fetcher.Fetch(ctx, ref)
}

// applySubPath applies a subpath to a cached path.
// Note: For fetched content, the subpath is already applied by the fetcher.
// This is only needed when returning a cached path that doesn't include subpath in the key.
func (r *Resolver) applySubPath(basePath, subPath string) (string, error) {
	if subPath == "" {
		return basePath, nil
	}

	// Defense-in-depth: validate subpath even though it was checked at parse time
	if err := validateSubPath(subPath); err != nil {
		return "", &FetchError{
			Ref:     &Reference{SubPath: subPath},
			Message: fmt.Sprintf("invalid subpath: %v", err),
		}
	}

	// Use filepath.Join instead of string concatenation for safe path construction
	fullPath := filepath.Join(basePath, subPath)

	// Verify the resolved path stays within basePath (containment check)
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return "", fmt.Errorf("cannot resolve base path: %w", err)
	}
	absFull, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("cannot resolve full path: %w", err)
	}
	if !strings.HasPrefix(absFull, absBase+string(filepath.Separator)) && absFull != absBase {
		return "", &FetchError{
			Ref:     &Reference{SubPath: subPath},
			Message: fmt.Sprintf("subpath %q escapes base directory", subPath),
		}
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", &FetchError{
				Ref:     &Reference{SubPath: subPath},
				Message: fmt.Sprintf("subpath %q not found in cached template", subPath),
				Err:     ErrSubPathNotFound,
			}
		}
		return "", fmt.Errorf("cannot access subpath: %w", err)
	}

	if !info.IsDir() {
		return "", &FetchError{
			Ref:     &Reference{SubPath: subPath},
			Message: fmt.Sprintf("subpath %q is not a directory", subPath),
		}
	}

	return fullPath, nil
}

// isZipFile checks if a path is a zip file.
func isZipFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".zip")
}

// IsLocal checks if a reference string points to a local resource.
func IsLocal(input string) bool {
	ref, err := Parse(input)
	if err != nil {
		return false
	}
	return !ref.IsRemote()
}
