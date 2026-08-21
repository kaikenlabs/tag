package remote

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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

// stagingPrefix marks a temp directory used to build a cache entry before
// it is published via rename. Any entry with this prefix is not a cache
// entry and must never be surfaced to callers.
const stagingPrefix = ".staging-"

// staleStagingAge is how long a staging directory is left alone before it is
// treated as debris from a crashed run. Cleanup judges staleness from the
// staging root's mtime, which writing into a subdirectory does not refresh,
// so this has to comfortably exceed the longest plausible copy or a live
// Set gets its own staging directory deleted mid-flight.
const staleStagingAge = 24 * time.Hour

// metaFileName is the filename of the metadata file within a cache entry.
const metaFileName = "_meta.json"

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
	return filepath.Join(c.baseDir, key, metaFileName)
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
// The new entry is built in a staging directory and published atomically
// via rename, so a concurrent or failed write can never leave a reader
// looking at a partially-written or mixed-content entry.
func (c *FSCache) Set(key, sourcePath string, meta *CacheMeta) (string, error) {
	cachePath := c.Path(key)

	if err := os.MkdirAll(c.baseDir, types.DirMode); err != nil {
		return "", &CacheError{Key: key, Op: "set", Message: "cannot create cache base directory", Err: err}
	}

	stage, err := fileutil.MkdirUnique(c.baseDir, stagingPrefix, types.DirMode)
	if err != nil {
		return "", &CacheError{Key: key, Op: "set", Message: "cannot create staging directory", Err: err}
	}
	defer os.RemoveAll(stage)

	if err := fileutil.CopyDir(sourcePath, stage, types.DirMode); err != nil {
		return "", &CacheError{Key: key, Op: "set", Message: "copy failed", Err: err}
	}

	// Set expiration if not pinned (no version specified)
	if meta.Version == "" && meta.ExpiresAt == nil {
		expires := time.Now().Add(c.ttl)
		meta.ExpiresAt = &expires
	}

	// Write metadata — cache entry is still valid without it, but log a warning.
	if err := c.writeMetaTo(filepath.Join(stage, metaFileName), meta); err != nil {
		slog.Warn("failed to write cache metadata", "key", key, "error", err)
	}

	if err := c.commit(stage, cachePath); err != nil {
		return "", &CacheError{Key: key, Op: "set", Message: "commit failed", Err: err}
	}

	return cachePath, nil
}

// commit publishes stage as final by renaming it into place.
//
// POSIX rename cannot replace a non-empty directory, so the existing entry
// moves aside first. A reader therefore sees the old entry, then briefly
// nothing, then the new one - never a partial tree. Verified on APFS: rename
// onto an existing directory fails with EEXIST even when that directory is
// empty, so the move-aside is not optional.
func (c *FSCache) commit(stage, final string) error {
	prev := stage + ".prev"
	if err := os.Rename(final, prev); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(stage, final); err != nil {
		if _, statErr := os.Stat(final); statErr == nil {
			// A concurrent writer committed a complete entry first; ours is redundant.
			os.RemoveAll(prev)
			return nil
		}
		if rollbackErr := os.Rename(prev, final); rollbackErr != nil {
			slog.Warn("failed to restore previous cache entry after failed commit", "path", final, "error", rollbackErr)
		}
		return err
	}
	os.RemoveAll(prev)
	return nil
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

// writeMetaTo writes the metadata file to an explicit path.
func (c *FSCache) writeMetaTo(path string, meta *CacheMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, types.FileModePrivate)
}

// reapIfStaleStaging removes name if it is a staging directory old enough to
// be crash debris, and reports whether name identifies a staging directory
// at all — callers use that to skip it from further processing regardless of
// whether it was actually removed. Cleanup and ClearAll both call this so
// "stale" means the same thing in both places.
//
// A staging directory this old is debris from a run that died between
// creating it and renaming it into place. A ".prev" one holds the entry
// that run was replacing; its key is not recoverable from the name, so a
// refetch is the only way back and dropping it is safe.
func (c *FSCache) reapIfStaleStaging(entry os.DirEntry) bool {
	name := entry.Name()
	if !strings.HasPrefix(name, stagingPrefix) {
		return false
	}

	if info, err := entry.Info(); err == nil && time.Since(info.ModTime()) > staleStagingAge {
		os.RemoveAll(filepath.Join(c.baseDir, name))
	}
	return true
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

		if c.reapIfStaleStaging(entry) {
			continue
		}

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
		if strings.HasPrefix(key, stagingPrefix) {
			continue
		}
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
		key := entry.Name()
		if c.reapIfStaleStaging(entry) {
			continue
		}
		// Only remove directories TAG wrote. Since TAG_CACHE_DIR lets an
		// operator point the base anywhere, removing every subdirectory would
		// turn `cache clear --all` into a recursive delete of whatever that
		// path happens to contain. Cleanup already skips entries with no
		// readable metadata for its own reasons; this matches it.
		if _, statErr := os.Stat(c.metaPath(key)); statErr != nil {
			continue
		}
		if err := c.Invalidate(key); err != nil {
			return removed, err
		}
		removed++
	}

	return removed, nil
}
