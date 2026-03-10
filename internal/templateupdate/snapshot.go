package templateupdate

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/kaikenlabs/tag/internal/fileutil"
	"github.com/kaikenlabs/tag/internal/types"
)

// ReadProjectFiles walks the project directory and returns an in-memory snapshot
// of all files as a map of forward-slash paths to RenderedFile pointers.
//
// It skips the .tag/ directory, .git/, and the .tagconfig.json file itself.
// Binary detection uses the same heuristic as template rendering.
func ReadProjectFiles(projectDir string) (map[string]*RenderedFile, error) {
	files := make(map[string]*RenderedFile)

	err := filepath.WalkDir(projectDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relPath, relErr := filepath.Rel(projectDir, path)
		if relErr != nil {
			return fmt.Errorf("get relative path: %w", relErr)
		}

		if relPath == "." {
			return nil
		}

		// Normalize to forward slashes for consistent keys.
		normalizedPath := filepath.ToSlash(relPath)

		// Skip internal directories.
		if d.IsDir() && shouldSkipDir(normalizedPath) {
			return filepath.SkipDir
		}

		// Skip symlinks and directories.
		if d.IsDir() || d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		// Skip TAG metadata files at root.
		if isProjectMetaFile(normalizedPath) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", normalizedPath, err)
		}

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("stat %s: %w", normalizedPath, err)
		}

		files[normalizedPath] = &RenderedFile{
			Content:  content,
			Mode:     sanitizeFileMode(info.Mode()),
			IsBinary: !fileutil.IsTextContent(content),
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk project directory: %w", err)
	}

	return files, nil
}

// shouldSkipDir returns true for directories that should be excluded from the
// project snapshot.
func shouldSkipDir(normalizedPath string) bool {
	switch normalizedPath {
	case ".git", types.TemplatesDir:
		return true
	default:
		return false
	}
}

// isProjectMetaFile returns true for root-level TAG metadata files.
func isProjectMetaFile(normalizedPath string) bool {
	dir := filepath.Dir(normalizedPath)
	if dir != "." {
		return false
	}

	name := filepath.Base(normalizedPath)
	return name == types.TagConfigFile || name == ".tagignore"
}

// ToPointerMap converts a value map to a pointer map for MergeTrees compatibility.
func ToPointerMap(m map[string]RenderedFile) map[string]*RenderedFile {
	result := make(map[string]*RenderedFile, len(m))
	for k, v := range m {
		result[k] = &v
	}
	return result
}
