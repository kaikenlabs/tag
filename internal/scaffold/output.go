package scaffold

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaikenlabs/tag/internal/fileutil"
	"github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/types"
)

// OutputWriter handles file generation and copying during scaffolding.
type OutputWriter interface {
	// Write processes a template directory and writes output.
	Write(templateRoot, outputDir string, vars map[string]any) error
}

// DefaultOutputWriter implements OutputWriter.
type DefaultOutputWriter struct {
	engine        template.TemplateRenderer
	pathProcessor PathProcessor
}

// NewOutputWriter creates a new output writer.
func NewOutputWriter(engine template.TemplateRenderer, pathProcessor PathProcessor) *DefaultOutputWriter {
	return &DefaultOutputWriter{
		engine:        engine,
		pathProcessor: pathProcessor,
	}
}

// Write processes the template directory and writes to the output directory.
func (w *DefaultOutputWriter) Write(templateRoot, outputDir string, vars map[string]any) error {
	// Build template context
	ctx := buildTemplateContext(vars)

	// Walk the template directory
	return filepath.WalkDir(templateRoot, func(srcPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip symlinks to prevent exfiltration of files outside the template
		if d.Type()&os.ModeSymlink != 0 {
			fmt.Printf("Warning: skipping symlink %s\n", srcPath)
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Get relative path from template root
		relPath, err := filepath.Rel(templateRoot, srcPath)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		// Skip tag.template.json
		if d.Name() == types.TemplateConfigFile && filepath.Dir(relPath) == "." {
			return nil
		}

		// Skip _generators directory (will be handled separately)
		// Match exact directory name, not prefix (e.g., don't skip "_generators-old")
		if relPath == types.GeneratorsDir || strings.HasPrefix(relPath, types.GeneratorsDir+string(filepath.Separator)) {
			return nil
		}

		// Process path placeholders
		processedPath, err := w.pathProcessor.ProcessPath(relPath, vars)
		if err != nil {
			return NewPathError(relPath, "failed to process path", err)
		}

		// Skip entries where the path rendered to empty (conditional exclusion).
		// This happens when a filename has a conditional block like
		// {% if vars.feature %}file.go{% endif %} and the condition is false.
		if processedPath == "" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Build full destination path
		destPath := filepath.Join(outputDir, processedPath)

		// Security: Validate that destPath stays within outputDir
		if err := validatePathWithinDir(destPath, outputDir); err != nil {
			return NewPathError(relPath, "path traversal detected", err)
		}

		if d.IsDir() {
			// Create directory
			if err := os.MkdirAll(destPath, types.DirMode); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", destPath, err)
			}
			return nil
		}

		// Process file
		return w.processFile(srcPath, destPath, ctx, d)
	})
}

// processFile processes a single file from the template.
// Text files are rendered through the template engine; binary files are copied as-is.
func (w *DefaultOutputWriter) processFile(srcPath, destPath string, ctx template.Context, d fs.DirEntry) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), types.DirMode); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Get source file info for permissions
	srcInfo, err := d.Info()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Sanitize file mode to remove dangerous permission bits
	mode := sanitizeFileMode(srcInfo.Mode())

	// Read file content to determine if it's text or binary
	content, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", srcPath, err)
	}

	if fileutil.IsTextContent(content) {
		return w.processTemplate(srcPath, destPath, content, ctx, mode)
	}

	// Copy binary file as-is
	return os.WriteFile(destPath, content, mode)
}

// processTemplate processes a text file through the template engine.
func (w *DefaultOutputWriter) processTemplate(srcPath, destPath string, content []byte, ctx template.Context, mode fs.FileMode) error {
	// Parse and execute template
	tmpl, err := w.engine.ParseString(string(content))
	if err != nil {
		return NewTemplateError(srcPath, "failed to parse template", err)
	}

	result, err := tmpl.Execute(ctx)
	if err != nil {
		return NewTemplateError(srcPath, "failed to execute template", err)
	}

	// Write output
	if err := os.WriteFile(destPath, []byte(result), mode); err != nil {
		return NewTemplateError(srcPath, "failed to write output", err)
	}

	return nil
}

// buildTemplateContext builds the template context from variables.
func buildTemplateContext(vars map[string]any) template.Context {
	ctx := make(template.Context)

	// Add vars namespace
	ctx["vars"] = vars

	// Add cookiecutter alias for compatibility
	ctx["cookiecutter"] = vars

	// Add individual variables at root level for convenience
	for k, v := range vars {
		ctx[k] = v
	}

	return ctx
}

// CopyGenerators copies the _generators directory to .tag.templates in the output.
func CopyGenerators(templateRoot, outputDir string) error {
	generatorsDir := filepath.Join(templateRoot, types.GeneratorsDir)

	// Check if _generators exists
	if _, err := os.Stat(generatorsDir); os.IsNotExist(err) {
		// No _generators directory, create empty .tag.templates
		templatesDir := filepath.Join(outputDir, types.TemplatesDir)
		return os.MkdirAll(templatesDir, types.DirMode)
	}

	// Copy _generators to .tag.templates
	templatesDir := filepath.Join(outputDir, types.TemplatesDir)
	return copyDir(generatorsDir, templatesDir)
}

// copyDir recursively copies a directory.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip symlinks to prevent copying files outside the template
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, types.DirMode)
		}

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(destPath), types.DirMode); err != nil {
			return err
		}

		return fileutil.CopyFile(path, destPath)
	})
}

// validatePathWithinDir ensures that path is within the base directory.
// This prevents path traversal attacks where placeholders could escape the output dir.
func validatePathWithinDir(path, baseDir string) error {
	// Clean both paths to resolve any . or .. components
	cleanPath := filepath.Clean(path)
	cleanBase := filepath.Clean(baseDir)

	// Get absolute paths
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	absBase, err := filepath.Abs(cleanBase)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute base: %w", err)
	}

	// Check if path starts with base
	// Add separator to avoid matching partial directory names (e.g., /foo vs /foobar)
	if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) && absPath != absBase {
		return fmt.Errorf("path %q escapes base directory %q", absPath, absBase)
	}

	return nil
}

// sanitizeFileMode removes dangerous permission bits (setuid, setgid, sticky).
func sanitizeFileMode(mode fs.FileMode) fs.FileMode {
	// Remove setuid, setgid, and sticky bits
	return mode &^ (fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky)
}

// GenerateTagConfig generates a .tagconfig.json file in the output directory.
func GenerateTagConfig(outputDir string) error {
	config := map[string]any{
		"env": map[string]string{
			"TAG_PATH":        types.TemplatesDir,
			"TAG_SHARED_PATH": types.SharedDir,
			"TAG_BUNDLE_PATH": types.BundlesDir,
		},
		"hooks": map[string][]string{
			"pre":  {},
			"post": {},
		},
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tagconfig: %w", err)
	}

	configPath := filepath.Join(outputDir, ".tagconfig.json")
	if err := os.WriteFile(configPath, data, types.FileMode); err != nil {
		return fmt.Errorf("failed to write tagconfig: %w", err)
	}

	return nil
}
