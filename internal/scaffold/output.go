package scaffold

import (
	"encoding/json"
	"fmt"
	"io"
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

// Ensure DefaultOutputWriter implements OutputWriter.
var _ OutputWriter = (*DefaultOutputWriter)(nil)

// Write processes the template directory and writes to the output directory.
//
//nolint:gocognit // file processing with multiple format-dependent branches
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

		// Skip _meta.json at root (remote cache artifact)
		if d.Name() == types.CacheMetaFile && filepath.Dir(relPath) == "." {
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
// Text files are rendered through the template engine; binary files are streamed as-is.
//
// Uses fd-based operations to prevent TOCTOU race conditions: the file is opened
// and verified via Lstat + f.Stat + os.SameFile before reading, ensuring that the
// file hasn't been swapped for a symlink between the directory walk and the read.
//
// Binary files are streamed via io.Copy to avoid loading large files entirely into memory.
// Text detection uses an 8KB sample from the beginning of the file.
func (w *DefaultOutputWriter) processFile(srcPath, destPath string, ctx template.Context, _ fs.DirEntry) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), types.DirMode); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// TOCTOU-safe file open: verify the file is regular (not a symlink) using
	// fd-based checks rather than relying on the stale DirEntry from WalkDir.
	f, mode, err := openRegularFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", srcPath, err)
	}
	defer f.Close()

	// Read a sample for text detection (same size as fileutil.IsTextContent uses)
	sample := make([]byte, 8192)
	n, readErr := f.Read(sample)
	sample = sample[:n]

	// Handle read errors (io.EOF is expected for small/empty files)
	if readErr != nil && readErr != io.EOF {
		return fmt.Errorf("failed to read file %s: %w", srcPath, readErr)
	}

	if fileutil.IsTextContent(sample) {
		// Text file: read the rest of the content and process as template
		var fullContent []byte
		if readErr == io.EOF {
			// Entire file fit in the sample
			fullContent = sample
		} else {
			// Read remaining content and combine with sample
			remainder, err := io.ReadAll(f)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", srcPath, err)
			}
			fullContent = make([]byte, len(sample)+len(remainder))
			copy(fullContent, sample)
			copy(fullContent[len(sample):], remainder)
		}
		return w.processTemplate(srcPath, destPath, fullContent, ctx, mode)
	}

	// Binary file: stream to destination using io.Copy
	return streamBinaryFile(f, destPath, sample, mode)
}

// streamBinaryFile writes a binary file to destPath by first writing the already-read
// sample bytes, then streaming the remainder directly from the source file descriptor.
func streamBinaryFile(src *os.File, destPath string, sample []byte, mode fs.FileMode) error {
	dst, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", destPath, err)
	}
	defer dst.Close()

	// Write the already-read sample bytes
	if len(sample) > 0 {
		if _, err := dst.Write(sample); err != nil {
			return fmt.Errorf("failed to write to %s: %w", destPath, err)
		}
	}

	// Stream the remainder directly from the source fd
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to stream to %s: %w", destPath, err)
	}

	return nil
}

// openRegularFile opens a file with TOCTOU-safe symlink verification.
// It performs: Lstat → Open → f.Stat → os.SameFile verification.
// Returns the open file handle and sanitized file mode. The caller must close the file.
func openRegularFile(path string) (*os.File, fs.FileMode, error) {
	// Step 1: Lstat to check the path without following symlinks
	lstatInfo, err := os.Lstat(path)
	if err != nil {
		return nil, 0, fmt.Errorf("lstat: %w", err)
	}
	if lstatInfo.Mode().Type()&os.ModeSymlink != 0 {
		return nil, 0, fmt.Errorf("symlink detected: %s", path)
	}
	if !lstatInfo.Mode().IsRegular() {
		return nil, 0, fmt.Errorf("not a regular file: %s", path)
	}

	// Step 2: Open the file to get a file descriptor
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("open: %w", err)
	}

	// Step 3: Stat the file descriptor (not the path)
	fstatInfo, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, fmt.Errorf("fstat: %w", err)
	}

	// Step 4: Verify the file descriptor matches what Lstat saw.
	// If the file was swapped between Lstat and Open, these won't match.
	if !os.SameFile(lstatInfo, fstatInfo) {
		f.Close()
		return nil, 0, fmt.Errorf("file changed between check and open (possible TOCTOU attack): %s", path)
	}

	// Sanitize file mode to remove dangerous permission bits
	mode := sanitizeFileMode(fstatInfo.Mode())

	return f, mode, nil
}

// processTemplate processes a text file through the template engine.
func (w *DefaultOutputWriter) processTemplate(srcPath, destPath string, content []byte, ctx template.Context, mode fs.FileMode) error {
	// Parse and execute template
	tmpl, err := w.engine.ParseString(string(content))
	if err != nil {
		return NewFileProcessingError(srcPath, "failed to parse template", err)
	}

	result, err := tmpl.Execute(ctx)
	if err != nil {
		return NewFileProcessingError(srcPath, "failed to execute template", err)
	}

	// Write output
	if err := os.WriteFile(destPath, []byte(result), mode); err != nil {
		return NewFileProcessingError(srcPath, "failed to write output", err)
	}

	return nil
}

// buildTemplateContext builds the template context from variables.
func buildTemplateContext(vars map[string]any) template.Context {
	return template.NewContextBuilder().
		WithVars(vars).
		Build()
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
	return fileutil.ValidatePathContainment(baseDir, path)
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
