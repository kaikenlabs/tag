package template

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nikolalohinski/gonja/v2/loaders"
)

// Loader provides template loading functionality with support for
// template inheritance (extends) and includes.
type Loader struct {
	baseDir string
	fs      fs.FS
}

// NewLoader creates a new template loader with the given base directory.
func NewLoader(baseDir string) *Loader {
	return &Loader{
		baseDir: baseDir,
	}
}

// NewLoaderFS creates a new template loader with a custom filesystem.
func NewLoaderFS(baseDir string, fsys fs.FS) *Loader {
	return &Loader{
		baseDir: baseDir,
		fs:      fsys,
	}
}

// Load reads a template from the filesystem.
func (l *Loader) Load(path string) (string, error) {
	fullPath, err := l.resolvePath(path)
	if err != nil {
		return "", fmt.Errorf("failed to load template %q: %w", path, err)
	}

	var content []byte

	if l.fs != nil {
		// When using fs.FS, use the cleaned relative path
		cleanPath := filepath.Clean(path)
		content, err = fs.ReadFile(l.fs, cleanPath)
	} else {
		content, err = os.ReadFile(fullPath)
	}

	if err != nil {
		return "", fmt.Errorf("failed to load template %q: %w", path, err)
	}

	return string(content), nil
}

// Exists checks if a template exists at the given path.
func (l *Loader) Exists(path string) bool {
	fullPath, err := l.resolvePath(path)
	if err != nil {
		return false
	}

	if l.fs != nil {
		cleanPath := filepath.Clean(path)
		_, err := fs.Stat(l.fs, cleanPath)
		return err == nil
	}

	_, err = os.Stat(fullPath)
	return err == nil
}

// resolvePath resolves a template path relative to the base directory.
// It validates that the resolved path stays within the base directory
// to prevent path traversal attacks.
func (l *Loader) resolvePath(path string) (string, error) {
	// Reject absolute paths
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("absolute paths not allowed: %s", path)
	}

	// Clean the path to resolve any . or .. segments
	cleanPath := filepath.Clean(path)

	// Reject paths that try to traverse outside base directory
	if strings.HasPrefix(cleanPath, "..") || strings.Contains(cleanPath, string(filepath.Separator)+"..") {
		return "", fmt.Errorf("path traversal not allowed: %s", path)
	}

	// Join with base directory
	fullPath := filepath.Join(l.baseDir, cleanPath)

	// Double-check the resolved path is within base directory
	absBase, err := filepath.Abs(l.baseDir)
	if err != nil {
		return "", fmt.Errorf("invalid base directory: %w", err)
	}
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Ensure the resolved path starts with the base directory
	if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) && absPath != absBase {
		return "", fmt.Errorf("path escapes base directory: %s", path)
	}

	return fullPath, nil
}

// BaseDir returns the loader's base directory.
func (l *Loader) BaseDir() string {
	return l.baseDir
}

// GonjaLoader wraps our Loader to implement Gonja's loader interface.
type GonjaLoader struct {
	loader *Loader
}

// NewGonjaLoader creates a Gonja-compatible loader from our Loader.
func NewGonjaLoader(loader *Loader) *GonjaLoader {
	return &GonjaLoader{loader: loader}
}

// Resolve returns the absolute path for a template.
// This implements part of the Gonja Loader interface.
func (g *GonjaLoader) Resolve(path string) (string, error) {
	fullPath, err := g.loader.resolvePath(path)
	if err != nil {
		return "", err
	}
	if !g.loader.Exists(path) {
		return "", fmt.Errorf("template not found: %s", path)
	}
	return fullPath, nil
}

// Read returns the content of a template.
// This implements part of the Gonja Loader interface.
func (g *GonjaLoader) Read(path string) (string, error) {
	return g.loader.Load(path)
}

// CreateFileSystemLoader creates a Gonja FileSystemLoader for the given directory.
// This is the recommended way to set up template loading for Gonja.
func CreateFileSystemLoader(baseDir string) (loaders.Loader, error) {
	// Ensure the directory exists
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("template directory does not exist: %s", baseDir)
	}

	// Use Gonja's built-in filesystem loader
	return loaders.MustNewFileSystemLoader(baseDir), nil
}

// LoadTemplateFiles loads all template files from a directory.
// This is useful for batch processing templates.
func LoadTemplateFiles(dir string, suffix string) (map[string]string, error) {
	templates := make(map[string]string)

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, "."+suffix) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", path, err)
		}

		// Use relative path as template name
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			relPath = path
		}
		templates[relPath] = string(content)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load templates from %s: %w", dir, err)
	}

	return templates, nil
}
