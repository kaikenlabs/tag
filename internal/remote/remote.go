package remote

import (
	"context"
	"fmt"
	"log/slog"
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

// Resolve takes a template reference string and returns a FetchResult containing
// the local path and, for git sources, the resolved commit SHA.
func (r *Resolver) Resolve(ctx context.Context, input string, opts ResolveOptions) (*FetchResult, error) {
	// 1. Parse the reference
	ref, err := Parse(input)
	if err != nil {
		return nil, fmt.Errorf("invalid template reference: %w", err)
	}

	// 2. If local directory, just return the path
	if ref.Type == ReferenceTypeLocal && !isZipFile(ref.URL) {
		return &FetchResult{Path: ref.URL}, nil
	}

	// For local zip files, we still need to extract them
	// but we won't cache them (they're already local)
	if ref.Type == ReferenceTypeZip && !ref.IsRemote() {
		return r.fetchAndReturn(ctx, ref)
	}

	// 3. Check cache unless:
	//    - ForceUpdate is explicitly requested, OR
	//    - the ref is floating (no pinned version/tag/commit), in which case we
	//      always fetch to pick up upstream changes
	cacheKey := ref.CacheKey()
	skipCache := opts.ForceUpdate || ref.Version == ""
	if !skipCache {
		if result, ok := r.tryCache(cacheKey, ref); ok {
			return result, nil
		}
	}

	// 4. If offline mode, fail
	if opts.Offline {
		return nil, &FetchError{
			Ref:     ref,
			Message: "not cached and offline mode is enabled",
			Err:     ErrNotCached,
		}
	}

	// 5. Fetch using appropriate fetcher
	fetcher, ok := r.fetchers[ref.Type]
	if !ok {
		return nil, &FetchError{
			Ref:     ref,
			Message: fmt.Sprintf("unsupported reference type: %s", ref.Type),
		}
	}

	fetchResult, err := fetcher.Fetch(ctx, ref)
	if err != nil {
		return nil, err
	}
	if fetchResult == nil {
		return nil, &FetchError{Ref: ref, Message: "fetcher returned nil result"}
	}

	// 6. Store in cache
	meta := &CacheMeta{
		OriginalRef: ref.Original,
		ResolvedURL: ref.URL,
		Version:     ref.Version,
		CommitSHA:   fetchResult.CommitSHA,
		FetchedAt:   time.Now(),
	}

	cachedPath, err := r.cache.Set(cacheKey, fetchResult.Path, meta)
	if err != nil {
		slog.Warn("could not cache template", "ref", ref.Original, "error", err)
		return fetchResult, nil
	}

	// 7. Clean up temp directory (ignore error - best effort)
	_ = CleanupTempDir(fetchResult.Path)

	// 8. Opportunistic cleanup of expired cache entries
	if fsCache, ok := r.cache.(*FSCache); ok {
		if _, cleanErr := fsCache.Cleanup(); cleanErr != nil {
			slog.Warn("cache cleanup failed", "error", cleanErr)
		}
	}

	// 9. Return cached path with commit SHA
	return &FetchResult{
		Path:      cachedPath,
		CommitSHA: fetchResult.CommitSHA,
		Version:   fetchResult.Version,
	}, nil
}

// tryCache attempts to return a cached result for the given key.
// Returns the FetchResult and true if a valid cache entry was found.
func (r *Resolver) tryCache(cacheKey string, ref *Reference) (*FetchResult, bool) {
	path, found, err := r.cache.Get(cacheKey)
	if err != nil || !found {
		return nil, false
	}

	resolvedPath, subErr := r.applySubPath(path, ref.SubPath)
	if subErr != nil {
		return nil, false
	}

	result := &FetchResult{Path: resolvedPath, Version: ref.Version}
	if fsCache, ok := r.cache.(*FSCache); ok {
		if meta, metaErr := fsCache.readMeta(cacheKey); metaErr == nil {
			result.CommitSHA = meta.CommitSHA
		}
	}
	return result, true
}

// fetchAndReturn fetches without caching (for local zip files).
func (r *Resolver) fetchAndReturn(ctx context.Context, ref *Reference) (*FetchResult, error) {
	fetcher, ok := r.fetchers[ref.Type]
	if !ok {
		return nil, &FetchError{
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
