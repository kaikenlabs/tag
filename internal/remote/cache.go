package remote

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kaikenlabs/tag/internal/fileutil"
)

// DefaultCacheDir is the default cache directory relative to user home.
const DefaultCacheDir = ".tag/cache"

// DefaultCacheTTL is the default TTL for non-pinned cache entries.
const DefaultCacheTTL = 24 * time.Hour

// CacheMeta contains metadata about a cached template.
type CacheMeta struct {
	OriginalRef string     `json:"original_ref"`
	ResolvedURL string     `json:"resolved_url"`
	Version     string     `json:"version,omitempty"`
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
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("cannot determine home directory: %w", err)
		}
		baseDir = filepath.Join(home, DefaultCacheDir)
	}

	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create cache directory: %w", err)
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
	if err := os.MkdirAll(cachePath, 0o755); err != nil {
		return "", &CacheError{Key: key, Op: "set", Message: "cannot create cache directory", Err: err}
	}

	// Copy contents from source to cache
	if err := c.copyDir(sourcePath, cachePath); err != nil {
		// Clean up on error
		os.RemoveAll(cachePath)
		return "", &CacheError{Key: key, Op: "set", Message: "copy failed", Err: err}
	}

	// Set expiration if not pinned (no version specified)
	if meta.Version == "" && meta.ExpiresAt == nil {
		expires := time.Now().Add(c.ttl)
		meta.ExpiresAt = &expires
	}

	// Write metadata (ignore error - cache entry is still valid without it)
	_ = c.writeMeta(key, meta)

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

	return os.WriteFile(metaFile, data, 0o600)
}

// copyDir recursively copies a directory.
func (c *FSCache) copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !srcInfo.IsDir() {
		return fmt.Errorf("source is not a directory: %s", src)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		// Skip symlinks to prevent copying files outside the source
		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}

		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Create subdirectory
			if err := os.MkdirAll(dstPath, 0o755); err != nil {
				return err
			}
			// Recursively copy
			if err := c.copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Copy file
			if err := c.copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file, skipping symlinks for security.
func (c *FSCache) copyFile(src, dst string) error {
	// Use Lstat to detect symlinks (Stat would follow them)
	linfo, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if linfo.Mode()&os.ModeSymlink != 0 {
		return nil // Skip symlinks
	}

	return fileutil.CopyFile(src, dst)
}

// Cleanup removes expired cache entries.
func (c *FSCache) Cleanup() error {
	entries, err := os.ReadDir(c.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

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
			_ = c.Invalidate(key)
		}
	}

	return nil
}

// List returns all cache keys.
func (c *FSCache) List() ([]string, error) {
	entries, err := os.ReadDir(c.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var keys []string
	for _, entry := range entries {
		if entry.IsDir() {
			keys = append(keys, entry.Name())
		}
	}

	return keys, nil
}
