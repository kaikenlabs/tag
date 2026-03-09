package scaffold

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"

	"github.com/kaikenlabs/tag/internal/fileutil"
	"github.com/kaikenlabs/tag/internal/history"
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
	engine               template.TemplateRenderer
	pathProcessor        PathProcessor
	allowRecursiveRender bool
	derivedVarNames      map[string]bool
	out                  io.Writer
	recorder             *history.Recorder // optional; nil = no recording
	dryRun               bool              // when true, log file paths instead of writing
}

// SetRecorder attaches a history recorder to this writer. When set, every
// file written during scaffold is recorded as a "create" entry.
func (w *DefaultOutputWriter) SetRecorder(r *history.Recorder) {
	w.recorder = r
}

// NewOutputWriter creates a new output writer.
func NewOutputWriter(engine template.TemplateRenderer, pathProcessor PathProcessor) *DefaultOutputWriter {
	return &DefaultOutputWriter{
		engine:        engine,
		pathProcessor: pathProcessor,
		out:           os.Stdout,
	}
}

// SetAllowRecursiveRender controls whether user-provided variable values
// containing template syntax are rendered in file content. When false (default),
// template delimiters in non-derived variable values are escaped to prevent SSTI.
func (w *DefaultOutputWriter) SetAllowRecursiveRender(allow bool) {
	w.allowRecursiveRender = allow
}

// SetDryRun enables dry-run mode, where file paths are logged instead of written.
func (w *DefaultOutputWriter) SetDryRun(v bool) {
	w.dryRun = v
}

// SetDerivedVarNames sets the derived variable names for SSTI protection.
// Derived variables are always rendered through the template engine.
func (w *DefaultOutputWriter) SetDerivedVarNames(names map[string]bool) {
	w.derivedVarNames = names
}

// escapeNonDerivedVars returns a copy of vars where template delimiters in
// non-derived string values are escaped with sentinel tokens, preventing SSTI.
func (w *DefaultOutputWriter) escapeNonDerivedVars(vars map[string]any) map[string]any {
	safe := make(map[string]any, len(vars))
	for k, v := range vars {
		if w.derivedVarNames[k] {
			safe[k] = v
			continue
		}
		if s, ok := v.(string); ok {
			safe[k] = escapeTemplateSyntax(s)
		} else {
			safe[k] = v
		}
	}
	return safe
}

// Ensure DefaultOutputWriter implements OutputWriter.
var _ OutputWriter = (*DefaultOutputWriter)(nil)

// Write processes the template directory and writes to the output directory.
//
//nolint:gocognit // file processing with multiple format-dependent branches
func (w *DefaultOutputWriter) Write(templateRoot, outputDir string, vars map[string]any) error {
	// Escape non-derived variable values to prevent SSTI in file content.
	// When allowRecursiveRender is false (default), template delimiters in
	// user-provided values are replaced with sentinel tokens before rendering.
	safeVars := vars
	if !w.allowRecursiveRender {
		safeVars = w.escapeNonDerivedVars(vars)
	}

	// Build template context with (possibly escaped) vars
	ctx := buildTemplateContext(safeVars)

	// Load .tagignore patterns (nil matcher if file absent or empty)
	ignoreMatcher, err := loadIgnorePatterns(templateRoot)
	if err != nil {
		return fmt.Errorf("load ignore patterns: %w", err)
	}

	// Walk the template directory
	return filepath.WalkDir(templateRoot, func(srcPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip symlinks to prevent exfiltration of files outside the template
		if d.Type()&os.ModeSymlink != 0 {
			fmt.Fprintf(w.out, "Warning: skipping symlink %s\n", srcPath)
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

		// Skip TAG-internal files and directories
		if isSkippedEntry(relPath, d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Apply .tagignore patterns
		if ignoreMatcher != nil {
			pathComponents := strings.Split(relPath, string(filepath.Separator))
			if ignoreMatcher.Match(pathComponents, d.IsDir()) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
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
			// In dry-run mode, skip directory creation.
			if w.dryRun {
				return nil
			}
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
	// Ensure parent directory exists (skip in dry-run).
	if !w.dryRun {
		if err := os.MkdirAll(filepath.Dir(destPath), types.DirMode); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}
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
		return w.processTextFile(srcPath, destPath, sample, f, readErr, ctx, mode)
	}

	// Binary file: in dry-run mode, print path and skip the write.
	if w.dryRun {
		fmt.Fprintf(w.out, "  (dry-run) would write: %s\n", destPath)
		return nil
	}

	// Binary file: stream to destination using io.Copy
	if err := streamBinaryFile(f, destPath, sample, mode); err != nil {
		return err
	}
	w.recordCreate(destPath)
	return nil
}

// processTextFile reads full content from an already-sampled source file,
// renders it as a template, and writes (or dry-run logs) the result.
func (w *DefaultOutputWriter) processTextFile(srcPath, destPath string, sample []byte, f *os.File, readErr error, ctx template.Context, mode fs.FileMode) error {
	var fullContent []byte
	if errors.Is(readErr, io.EOF) {
		// Entire file fit in the sample.
		fullContent = sample
	} else {
		var err error
		fullContent, err = io.ReadAll(io.MultiReader(bytes.NewReader(sample), f))
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", srcPath, err)
		}
	}
	if err := w.processTemplate(srcPath, destPath, fullContent, ctx, mode); err != nil {
		return err
	}
	if !w.dryRun {
		w.recordCreate(destPath)
	}
	return nil
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

	// Restore escaped template delimiters back to their original form.
	// This ensures user-provided values containing {{ }} appear literally
	// in the output rather than being executed by the template engine.
	if !w.allowRecursiveRender {
		result = unescapeTemplateSyntax(result)
	}

	// In dry-run mode, print the destination path and skip the write.
	if w.dryRun {
		fmt.Fprintf(w.out, "  (dry-run) would write: %s\n", destPath) //nolint:gosec // G705: destPath is sanitized by validatePathWithinDir; log injection not a concern in a CLI tool
		return nil
	}

	// Write output
	if err := os.WriteFile(destPath, []byte(result), mode); err != nil { //nolint:gosec // G703: destPath is sanitized by path placeholder processing
		return NewFileProcessingError(srcPath, "failed to write output", err)
	}

	return nil
}

// recordCreate records a newly written file with the history recorder, if set.
func (w *DefaultOutputWriter) recordCreate(destPath string) {
	if w.recorder == nil {
		return
	}
	hashAfter, err := history.HashFile(destPath)
	if err != nil {
		fmt.Fprintf(w.out, "Warning: could not hash %s for history: %v\n", destPath, err)
		return
	}
	w.recorder.RecordCreate(destPath, hashAfter)
}

// buildTemplateContext builds the template context from variables.
func buildTemplateContext(vars map[string]any) template.Context {
	return template.NewContextBuilder().
		WithVars(vars).
		Build()
}

// validatePathWithinDir ensures that path is within the base directory.
// This prevents path traversal attacks where placeholders could escape the output dir.
func validatePathWithinDir(path, baseDir string) error {
	return fileutil.ValidatePathContainment(baseDir, path)
}

// isSkippedEntry returns true for TAG-internal files/directories that should
// never appear in scaffold output: tag.template.json, .tagignore, _meta.json
// (all at root), and _generators/ and .tag/ directories at any depth.
func isSkippedEntry(relPath, name string) bool {
	atRoot := filepath.Dir(relPath) == "."

	// Root-only files
	if atRoot && (name == types.TemplateConfigFile || name == types.CacheMetaFile || name == types.TagIgnoreFile) {
		return true
	}

	// _generators directory tree
	if relPath == types.GeneratorsDir || strings.HasPrefix(relPath, types.GeneratorsDir+string(filepath.Separator)) {
		return true
	}

	// .tag directory tree
	if relPath == types.TemplatesDir || strings.HasPrefix(relPath, types.TemplatesDir+string(filepath.Separator)) {
		return true
	}

	return false
}

// loadIgnorePatterns reads a .tagignore file from the template root and returns
// a gitignore-style matcher. Returns nil, nil if the file does not exist or is empty.
//
//nolint:nilnil // nil matcher signals "no ignore file" — callers nil-check before use
func loadIgnorePatterns(templateRoot string) (gitignore.Matcher, error) {
	f, err := os.Open(filepath.Join(templateRoot, types.TagIgnoreFile))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("open %s: %w", types.TagIgnoreFile, err)
	}
	defer f.Close()

	var patterns []gitignore.Pattern
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, gitignore.ParsePattern(line, nil))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", types.TagIgnoreFile, err)
	}

	if len(patterns) == 0 {
		return nil, nil
	}
	return gitignore.NewMatcher(patterns), nil
}

// sanitizeFileMode removes dangerous permission bits (setuid, setgid, sticky).
func sanitizeFileMode(mode fs.FileMode) fs.FileMode {
	// Remove setuid, setgid, and sticky bits
	return mode &^ (fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky)
}

// TagConfigOptions provides template metadata for enriched .tagconfig.json generation.
type TagConfigOptions struct {
	TemplateType    types.TemplateType // "local" or "remote"
	TemplateSource  string             // Original ref (e.g., "gh:user/repo" or local path)
	TemplateName    string             // Library name (if any)
	TemplateVersion string             // From tag.template.json
	TemplateRef     string             // Branch/tag used (e.g., "main", "v1.2.0")
	CommitSHA       string             // Resolved git commit SHA (empty for local/zip)
	SkipPatterns    []string           // User-configurable update exclusions
	Variables       map[string]any     // Scaffold-time variable values
}

// tagConfigJSON is the serialization format for .tagconfig.json (v1 schema).
type tagConfigJSON struct {
	SchemaVersion int                 `json:"schema_version"`
	Template      tagTemplateJSON     `json:"template"`
	Variables     map[string]any      `json:"variables,omitempty"`
	SkipPatterns  []string            `json:"skip_patterns"`
	Env           map[string]string   `json:"env"`
	Hooks         map[string][]string `json:"hooks"`
}

// tagTemplateJSON is the template origin section of .tagconfig.json.
type tagTemplateJSON struct {
	Type      types.TemplateType `json:"type"`
	Source    string             `json:"source,omitempty"`
	Name      string             `json:"name,omitempty"`
	Version   string             `json:"version,omitempty"`
	Ref       string             `json:"ref,omitempty"`
	CommitSHA string             `json:"commit,omitempty"`
}

// GenerateTagConfig generates a .tagconfig.json file in the output directory.
// The template section is always written with an explicit type discriminator.
func GenerateTagConfig(outputDir string, opts TagConfigOptions) error {
	templateType := opts.TemplateType
	if templateType == "" {
		templateType = types.TemplateTypeLocal
	}

	skipPatterns := opts.SkipPatterns
	if skipPatterns == nil {
		skipPatterns = []string{}
	}

	cfg := tagConfigJSON{
		SchemaVersion: types.TagConfigSchemaVersion,
		Template: tagTemplateJSON{
			Type:      templateType,
			Source:    opts.TemplateSource,
			Name:      opts.TemplateName,
			Version:   opts.TemplateVersion,
			Ref:       opts.TemplateRef,
			CommitSHA: opts.CommitSHA,
		},
		Variables:    opts.Variables,
		SkipPatterns: skipPatterns,
		Env: map[string]string{
			"TAG_PATH":        types.TemplatesDir,
			"TAG_SHARED_PATH": types.SharedDir,
			"TAG_BUNDLE_PATH": types.BundlesDir,
		},
		Hooks: map[string][]string{
			"pre":  {},
			"post": {},
		},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tagconfig: %w", err)
	}

	configPath := filepath.Join(outputDir, types.TagConfigFile)
	if err := os.WriteFile(configPath, data, types.FileMode); err != nil {
		return fmt.Errorf("failed to write tagconfig: %w", err)
	}

	return nil
}

// TagConfig represents a parsed .tagconfig.json file.
// Used by the update system to read existing project configurations.
type TagConfig struct {
	SchemaVersion int                 `json:"schema_version,omitempty"`
	Template      *TagTemplate        `json:"template,omitempty"`
	Variables     map[string]any      `json:"variables,omitempty"`
	SkipPatterns  []string            `json:"skip_patterns,omitempty"`
	Env           map[string]string   `json:"env,omitempty"`
	Hooks         map[string][]string `json:"hooks,omitempty"`
}

// TagTemplate describes the template origin recorded in .tagconfig.json.
type TagTemplate struct {
	Type      types.TemplateType `json:"type,omitempty"`
	Source    string             `json:"source,omitempty"`
	Name      string             `json:"name,omitempty"`
	Version   string             `json:"version,omitempty"`
	Ref       string             `json:"ref,omitempty"`
	CommitSHA string             `json:"commit,omitempty"`
}

// LoadTagConfig reads and parses a .tagconfig.json from the given project directory.
func LoadTagConfig(projectDir string) (*TagConfig, error) {
	data, err := os.ReadFile(filepath.Join(projectDir, types.TagConfigFile))
	if err != nil {
		return nil, fmt.Errorf("read tagconfig: %w", err)
	}

	return ParseTagConfigJSON(data)
}

// ParseTagConfigJSON parses raw JSON bytes into a TagConfig.
func ParseTagConfigJSON(data []byte) (*TagConfig, error) {
	var cfg TagConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse tagconfig: %w", err)
	}

	cfg.normalize()
	return &cfg, nil
}

// normalize fills in defaults for legacy (pre-v1) configs.
func (c *TagConfig) normalize() {
	if c.SchemaVersion == 0 {
		c.SchemaVersion = types.TagConfigSchemaVersion
	}
	if c.SkipPatterns == nil {
		c.SkipPatterns = []string{}
	}
	if c.Template != nil && c.Template.Type == "" {
		c.Template.Type = inferTemplateType(c.Template.Source)
	}
}

// HasTemplateOrigin reports whether the config has enough template metadata
// for the update system to resolve the original template.
func (c *TagConfig) HasTemplateOrigin() bool {
	return c.Template != nil && c.Template.Source != ""
}

// inferTemplateType guesses the template type from the source string.
func inferTemplateType(source string) types.TemplateType {
	for _, prefix := range []string{"gh:", "gl:", "bb:", "http://", "https://", "git@", "git://", "git+ssh://"} {
		if strings.HasPrefix(source, prefix) {
			return types.TemplateTypeRemote
		}
	}
	return types.TemplateTypeLocal
}
