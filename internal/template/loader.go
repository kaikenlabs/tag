package template

import (
	"fmt"
	"os"

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
