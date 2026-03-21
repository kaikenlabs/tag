// Package templateupdate provides historical template rendering for template
// lifecycle operations (update, diff, check). It renders a template at a
// specific git commit without side effects (no hooks, no metadata files).
package templateupdate

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"

	"github.com/kaikenlabs/tag/internal/dialect"
	"github.com/kaikenlabs/tag/internal/fileutil"
	"github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/tmplconfig"
	"github.com/kaikenlabs/tag/internal/types"
)

// CommitFetcher checks out a git repository at a specific commit SHA.
type CommitFetcher interface {
	FetchAtCommit(ctx context.Context, url, commitSHA, destDir string) (string, error)
}

// RenderedFile represents a single file produced by rendering a template.
type RenderedFile struct {
	Content  []byte
	Mode     os.FileMode
	IsBinary bool
}

// HistoricalRenderer renders a template at a specific git commit using
// provided variables. It produces an in-memory snapshot of the rendered
// output without any side effects.
type HistoricalRenderer struct {
	fetcher CommitFetcher
}

// NewHistoricalRenderer creates a renderer that uses the given fetcher
// to check out templates at historical commits.
func NewHistoricalRenderer(fetcher CommitFetcher) *HistoricalRenderer {
	return &HistoricalRenderer{fetcher: fetcher}
}

// renderState holds the walk state for a single RenderAt invocation.
type renderState struct {
	checkoutDir   string
	engine        *template.Engine
	tmplCtx       template.Context
	ignoreMatcher gitignore.Matcher
	files         map[string]RenderedFile
	ctx           context.Context
}

// RenderAt checks out the template at commitSHA and renders it with vars.
// It returns a map of relative paths (using forward slashes) to rendered files.
// The caller-provided vars are used as-is; no variable collection or prompting occurs.
//
// The method creates a temporary directory for the checkout and cleans it up
// before returning, regardless of success or failure.
func (r *HistoricalRenderer) RenderAt(
	ctx context.Context,
	templateURL string,
	commitSHA string,
	vars map[string]any,
) (map[string]RenderedFile, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("render at commit %s: %w", commitSHA, err)
	}

	tmpDir, err := os.MkdirTemp("", "tag-historical-*")
	if err != nil {
		return nil, fmt.Errorf("create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	checkoutDir, err := r.fetchTemplate(ctx, templateURL, commitSHA, tmpDir)
	if err != nil {
		return nil, err
	}

	if configErr := validateTemplateConfig(checkoutDir); configErr != nil {
		return nil, configErr
	}

	// Load dialects (all 3 tiers) for the to() filter.
	reg, dialectErr := dialect.LoadForTemplate(checkoutDir, types.DialectsDir)
	if dialectErr != nil {
		slog.Debug("dialect loading failed, continuing without dialects", "error", dialectErr)
	}

	var engineOpts []template.Option
	if reg != nil {
		engineOpts = append(engineOpts, template.WithDialectRegistry(reg))
	}

	engine, err := template.NewEngine(engineOpts...)
	if err != nil {
		return nil, fmt.Errorf("create template engine: %w", err)
	}

	ignoreMatcher, err := loadIgnorePatterns(checkoutDir)
	if err != nil {
		return nil, fmt.Errorf("load ignore patterns: %w", err)
	}

	state := &renderState{
		checkoutDir:   checkoutDir,
		engine:        engine,
		tmplCtx:       template.NewContextBuilder().WithVars(vars).Build(),
		ignoreMatcher: ignoreMatcher,
		files:         make(map[string]RenderedFile),
		ctx:           ctx,
	}

	if err := filepath.WalkDir(checkoutDir, state.walkEntry); err != nil {
		return nil, fmt.Errorf("walk template directory: %w", err)
	}

	return state.files, nil
}

// LoadConfigAtCommit fetches the template at the given commit and returns
// the parsed TemplateConfig (tag.template.json). The checkout is created
// in a temporary directory and cleaned up before returning.
func (r *HistoricalRenderer) LoadConfigAtCommit(
	ctx context.Context,
	templateURL string,
	commitSHA string,
) (*tmplconfig.TemplateConfig, error) {
	tmpDir, err := os.MkdirTemp("", "tag-config-*")
	if err != nil {
		return nil, fmt.Errorf("create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	checkoutDir, err := r.fetchTemplate(ctx, templateURL, commitSHA, tmpDir)
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(checkoutDir, types.TemplateConfigFile)

	data, readErr := os.ReadFile(configPath)
	if readErr != nil {
		return nil, fmt.Errorf("read %s at commit %s: %w", types.TemplateConfigFile, shortSHA(commitSHA), readErr)
	}

	cfg, parseErr := tmplconfig.ParseTemplateConfig(data)
	if parseErr != nil {
		return nil, fmt.Errorf("parse %s at commit %s: %w", types.TemplateConfigFile, shortSHA(commitSHA), parseErr)
	}

	return cfg, nil
}

// fetchTemplate checks out the template at the given commit into a subdirectory of tmpDir.
func (r *HistoricalRenderer) fetchTemplate(ctx context.Context, templateURL, commitSHA, tmpDir string) (string, error) {
	checkoutDir := filepath.Join(tmpDir, "checkout")
	if mkErr := os.MkdirAll(checkoutDir, types.DirMode); mkErr != nil {
		return "", fmt.Errorf("create checkout directory: %w", mkErr)
	}

	fetchedDir, fetchErr := r.fetcher.FetchAtCommit(ctx, templateURL, commitSHA, checkoutDir)
	if fetchErr != nil {
		return "", fmt.Errorf("fetch template at commit %s: %w", commitSHA, fetchErr)
	}

	// Honor the fetcher's returned path (may differ from destDir if subdir applies).
	if fetchedDir != "" {
		return fetchedDir, nil
	}

	return checkoutDir, nil
}

// validateTemplateConfig loads and validates the template config file.
func validateTemplateConfig(checkoutDir string) error {
	configPath := filepath.Join(checkoutDir, types.TemplateConfigFile)

	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", types.TemplateConfigFile, err)
	}

	if _, err := tmplconfig.ParseTemplateConfig(configData); err != nil {
		return fmt.Errorf("parse %s: %w", types.TemplateConfigFile, err)
	}

	return nil
}

// walkEntry processes a single entry during the template directory walk.
func (s *renderState) walkEntry(srcPath string, d fs.DirEntry, walkErr error) error {
	if walkErr != nil {
		return walkErr
	}

	if err := s.ctx.Err(); err != nil {
		return err
	}

	// Skip symlinks.
	if d.Type()&os.ModeSymlink != 0 {
		return skipEntry(d)
	}

	relPath, err := filepath.Rel(s.checkoutDir, srcPath)
	if err != nil {
		return fmt.Errorf("get relative path: %w", err)
	}

	if relPath == "." {
		return nil
	}

	if isSkippedEntry(relPath, d.Name()) {
		return skipEntry(d)
	}

	if s.isIgnored(relPath, d.IsDir()) {
		return skipEntry(d)
	}

	renderedPath, err := s.renderPath(relPath)
	if err != nil {
		return err
	}

	if renderedPath == "" {
		return skipEntry(d)
	}

	if d.IsDir() {
		return nil
	}

	return s.processFile(srcPath, relPath, renderedPath, d)
}

// renderPath renders template placeholders in the path and validates the result.
func (s *renderState) renderPath(relPath string) (string, error) {
	rendered, err := s.engine.ExecuteToString(relPath, s.tmplCtx)
	if err != nil {
		return "", fmt.Errorf("render path %q: %w", relPath, err)
	}

	if strings.TrimSpace(rendered) == "" {
		return "", nil
	}

	cleanPath := filepath.Clean(rendered)
	normalizedPath := filepath.ToSlash(cleanPath)

	if filepath.IsAbs(cleanPath) || strings.HasPrefix(normalizedPath, "../") || normalizedPath == ".." {
		return "", fmt.Errorf("rendered path %q escapes output directory", relPath)
	}

	return normalizedPath, nil
}

// processFile reads, detects binary/text, renders if text, and stores the result.
func (s *renderState) processFile(srcPath, relPath, normalizedPath string, d fs.DirEntry) error {
	if _, exists := s.files[normalizedPath]; exists {
		return fmt.Errorf("duplicate rendered path %q (from source %q)", normalizedPath, relPath)
	}

	content, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("read file %s: %w", relPath, err)
	}

	info, err := d.Info()
	if err != nil {
		return fmt.Errorf("stat file %s: %w", relPath, err)
	}

	mode := fileutil.SanitizeFileMode(info.Mode())

	if !fileutil.IsTextContent(content) {
		s.files[normalizedPath] = RenderedFile{
			Content:  content,
			Mode:     mode,
			IsBinary: true,
		}
		return nil
	}

	rendered, err := s.engine.ExecuteToString(string(content), s.tmplCtx)
	if err != nil {
		return fmt.Errorf("render file %q: %w", relPath, err)
	}

	s.files[normalizedPath] = RenderedFile{
		Content:  []byte(rendered),
		Mode:     mode,
		IsBinary: false,
	}

	return nil
}

// isIgnored checks if a path matches .tagignore patterns.
func (s *renderState) isIgnored(relPath string, isDir bool) bool {
	if s.ignoreMatcher == nil {
		return false
	}

	pathComponents := strings.Split(relPath, string(filepath.Separator))
	return s.ignoreMatcher.Match(pathComponents, isDir)
}

// skipEntry returns filepath.SkipDir for directories, nil for files.
func skipEntry(d fs.DirEntry) error {
	if d.IsDir() {
		return filepath.SkipDir
	}
	return nil
}

// isSkippedEntry returns true for TAG-internal files and directories that
// should not appear in rendered output.
func isSkippedEntry(relPath, name string) bool {
	atRoot := filepath.Dir(relPath) == "."

	// Root-only files.
	if atRoot && (name == types.TemplateConfigFile || name == types.CacheMetaFile || name == types.TagIgnoreFile) {
		return true
	}

	// _generators directory tree.
	if relPath == types.GeneratorsDir || strings.HasPrefix(relPath, types.GeneratorsDir+string(filepath.Separator)) {
		return true
	}

	// _dialects directory tree.
	if relPath == types.DialectsDir || strings.HasPrefix(relPath, types.DialectsDir+string(filepath.Separator)) {
		return true
	}

	// .tag directory tree.
	if relPath == types.TemplatesDir || strings.HasPrefix(relPath, types.TemplatesDir+string(filepath.Separator)) {
		return true
	}

	return false
}

// loadIgnorePatterns reads .tagignore from the template root and returns
// a gitignore-style matcher. Returns a no-op matcher if the file does not exist.
//
//nolint:nilnil // returning nil matcher + nil error is the intended API for "no patterns"
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
