package remote

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/kaikenlabs/tag/internal/fileutil"
	"github.com/kaikenlabs/tag/internal/types"
)

// DefaultCacheDir is the default cache directory relative to user home.
const DefaultCacheDir = ".tag/cache"

// EnvCacheDir overrides the cache base directory when set to a non-empty
// absolute path. It is consulted before os.UserHomeDir, so it works in
// sandboxes/containers where HOME is unset.
const EnvCacheDir = "TAG_CACHE_DIR"

// DefaultCacheTTL is the default TTL for non-pinned cache entries.
const DefaultCacheTTL = 24 * time.Hour

// CacheMeta contains metadata about a cached template.
type CacheMeta struct {
	OriginalRef string     `json:"original_ref"`
	ResolvedURL string     `json:"resolved_url"`
	Version     string     `json:"version,omitempty"`
	CommitSHA   string     `json:"commit_sha,omitempty"` // Resolved git commit SHA
	FetchedAt   time.Time  `json:"fetched_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"` // nil = never expires (pinned)
}

// Cache defines the interface for template caching.
type Cache interface {
	// Get returns the cached template path if it exists and is valid.
	// Returns the path, whether it was found, and any error.
	Get(key string) (path string, found bool, err error)

	// Set stores a template in the cache, copying from sourcePath.
	// Returns the cached path.
	Set(key string, sourcePath string, meta *CacheMeta) (cachedPath string, err error)

	// Invalidate removes a cache entry.
	Invalidate(key string) error

	// Path returns the full path for a cache key (without checking existence).
	Path(key string) string
}

// FSCache implements Cache using the filesystem.
type FSCache struct {
	baseDir string
	ttl     time.Duration
}

// NewFSCache creates a new filesystem-based cache.
// If baseDir is empty, uses ~/.tag/cache/
func NewFSCache(baseDir string) (*FSCache, error) {
	if baseDir == "" {
		if envDir := os.Getenv(EnvCacheDir); envDir != "" {
			if !filepath.IsAbs(envDir) {
				return nil, fmt.Errorf("%s must be an absolute path, got %q", EnvCacheDir, envDir)
			}
			baseDir = envDir
		}
	}

	if baseDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("cannot determine home directory: %w", err)
		}
		baseDir = filepath.Join(home, DefaultCacheDir)
	}

	return &FSCache{
		baseDir: baseDir,
		ttl:     DefaultCacheTTL,
	}, nil
}

// SetTTL sets the TTL for non-pinned cache entries.
func (c *FSCache) SetTTL(ttl time.Duration) {
	c.ttl = ttl
}

// Path returns the full path for a cache key.
func (c *FSCache) Path(key string) string {
	return filepath.Join(c.baseDir, key)
}

// metaPath returns the path to the metadata file for a cache key.
func (c *FSCache) metaPath(key string) string {
	return filepath.Join(c.baseDir, key, "_meta.json")
}

// Get returns the cached template path if it exists and is valid.
func (c *FSCache) Get(key string) (string, bool, error) {
	cachePath := c.Path(key)

	// Check if cache directory exists
	info, err := os.Stat(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, &CacheError{Key: key, Op: "get", Message: "stat failed", Err: err}
	}

	if !info.IsDir() {
		return "", false, &CacheError{Key: key, Op: "get", Message: "cache entry is not a directory"}
	}

	// Read and check metadata
	meta, err := c.readMeta(key)
	if err != nil {
		// Cache lookup errors are treated as cache misses; let caller re-fetch.
		return "", false, nil //nolint:nilerr // intentional: corrupted/missing metadata = cache miss
	}

	// Check expiration
	if meta.ExpiresAt != nil && time.Now().After(*meta.ExpiresAt) {
		// Expired - treat as cache miss
		return "", false, nil
	}

	return cachePath, true, nil
}

// Set stores a template in the cache by copying from sourcePath.
func (c *FSCache) Set(key, sourcePath string, meta *CacheMeta) (string, error) {
	cachePath := c.Path(key)

	// Remove existing cache entry if present
	if err := os.RemoveAll(cachePath); err != nil && !os.IsNotExist(err) {
		return "", &CacheError{Key: key, Op: "set", Message: "cannot remove existing cache", Err: err}
	}

	// Create cache directory
	if err := os.MkdirAll(cachePath, types.DirMode); err != nil {
		return "", &CacheError{Key: key, Op: "set", Message: "cannot create cache directory", Err: err}
	}

	// Copy contents from source to cache
	if err := fileutil.CopyDir(sourcePath, cachePath, types.DirMode); err != nil {
		// Clean up on error
		os.RemoveAll(cachePath)
		return "", &CacheError{Key: key, Op: "set", Message: "copy failed", Err: err}
	}

	// Set expiration if not pinned (no version specified)
	if meta.Version == "" && meta.ExpiresAt == nil {
		expires := time.Now().Add(c.ttl)
		meta.ExpiresAt = &expires
	}

	// Write metadata — cache entry is still valid without it, but log a warning.
	if err := c.writeMeta(key, meta); err != nil {
		slog.Warn("failed to write cache metadata", "key", key, "error", err)
	}

	return cachePath, nil
}

// Invalidate removes a cache entry.
func (c *FSCache) Invalidate(key string) error {
	cachePath := c.Path(key)

	if err := os.RemoveAll(cachePath); err != nil && !os.IsNotExist(err) {
		return &CacheError{Key: key, Op: "invalidate", Message: "removal failed", Err: err}
	}

	return nil
}

// readMeta reads the metadata file for a cache entry.
func (c *FSCache) readMeta(key string) (*CacheMeta, error) {
	metaFile := c.metaPath(key)

	data, err := os.ReadFile(metaFile)
	if err != nil {
		return nil, err
	}

	var meta CacheMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}

// writeMeta writes the metadata file for a cache entry.
func (c *FSCache) writeMeta(key string, meta *CacheMeta) error {
	metaFile := c.metaPath(key)

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metaFile, data, types.FileModePrivate)
}

// Cleanup removes expired cache entries and returns the count of removed entries.
func (c *FSCache) Cleanup() (int, error) {
	entries, err := os.ReadDir(c.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	removed := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		key := entry.Name()
		meta, err := c.readMeta(key)
		if err != nil {
			// Can't read metadata, skip
			continue
		}

		if meta.ExpiresAt != nil && time.Now().After(*meta.ExpiresAt) {
			// Expired, remove (ignore error - best effort cleanup)
			if invalidateErr := c.Invalidate(key); invalidateErr == nil {
				removed++
			}
		}
	}

	return removed, nil
}

// CacheEntry contains a cache key and its metadata.
type CacheEntry struct {
	Key  string     `json:"key"`
	Meta *CacheMeta `json:"meta"` // nil if metadata is missing/corrupt
}

// List returns all cache entries with their metadata.
func (c *FSCache) List() ([]CacheEntry, error) {
	entries, err := os.ReadDir(c.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return make([]CacheEntry, 0), nil
		}
		return nil, err
	}

	result := make([]CacheEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		key := entry.Name()
		meta, _ := c.readMeta(key) // nil if unreadable
		result = append(result, CacheEntry{Key: key, Meta: meta})
	}

	return result, nil
}

// ClearExpired removes only expired cache entries.
func (c *FSCache) ClearExpired() (int, error) {
	return c.Cleanup()
}

// ClearAll removes all cache entries.
func (c *FSCache) ClearAll() (int, error) {
	entries, err := os.ReadDir(c.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	removed := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Only remove directories TAG wrote. Since TAG_CACHE_DIR lets an
		// operator point the base anywhere, removing every subdirectory would
		// turn `cache clear --all` into a recursive delete of whatever that
		// path happens to contain. Cleanup already skips entries with no
		// readable metadata for its own reasons; this matches it.
		if _, statErr := os.Stat(c.metaPath(entry.Name())); statErr != nil {
			continue
		}
		if err := c.Invalidate(entry.Name()); err != nil {
			return removed, err
		}
		removed++
	}

	return removed, nil
}
