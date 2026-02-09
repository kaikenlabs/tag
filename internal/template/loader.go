package template

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nikolalohinski/gonja/v2/loaders"
)

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

// CreateMemoryLoaderFromMap creates a Gonja MemoryLoader from a map of template contents.
// The keys should be template names (paths), values are template contents.
// This is useful for loading shared templates into memory.
func CreateMemoryLoaderFromMap(templates map[string]string) loaders.Loader {
	// Ensure keys start with "/" for Gonja's memory loader
	normalized := make(map[string]string, len(templates))
	for name, content := range templates {
		key := name
		if key != "" && key[0] != '/' {
			key = "/" + key
		}
		normalized[key] = content
	}
	return loaders.MustNewMemoryLoader(normalized)
}

// LoadTemplateTree recursively loads all template files from a directory tree.
// Files are filtered by the given suffix. This is useful for batch processing templates.
func LoadTemplateTree(dir, suffix string) (map[string]string, error) {
	templates := make(map[string]string)

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip symlinks to prevent loading files outside the template directory
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
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
