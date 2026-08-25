package convert

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaikenlabs/tag/internal/fileutil"
	"github.com/kaikenlabs/tag/internal/remote"
	"github.com/kaikenlabs/tag/internal/types"
)

// Converter handles the conversion of Cookiecutter templates to TAG format.
type Converter struct {
	resolver *remote.Resolver
	analyzer *ContentAnalyzer
}

// NewConverter creates a new Converter instance.
func NewConverter() (*Converter, error) {
	resolver, err := remote.NewResolver()
	if err != nil {
		return nil, fmt.Errorf("failed to create resolver: %w", err)
	}

	return &Converter{
		resolver: resolver,
		analyzer: NewContentAnalyzer(),
	}, nil
}

// Convert performs the Cookiecutter to TAG template conversion.
func (c *Converter) Convert(ctx context.Context, opts Options) (*Result, error) {
	// The slices are initialised empty rather than left nil so they serialise
	// as [] and not null, per internal/jsonout's convention of building a
	// slice at its assembly site instead of patching it up on the way out.
	result := &Result{
		Source:            opts.Source,
		DryRun:            opts.DryRun,
		Incompatibilities: []Incompatibility{},
		Warnings:          []string{},
		Files:             []PathConversion{},
		Variables:         []VariableConversion{},
	}

	// 1. Resolve source template (handles local and remote)
	templateDir, err := c.resolveSource(ctx, opts.Source)
	if err != nil {
		return nil, err
	}

	// 2. Verify it's a Cookiecutter template
	cookiecutterPath := filepath.Join(templateDir, types.CookiecutterConfigFile)
	if _, err := os.Stat(cookiecutterPath); os.IsNotExist(err) { //nolint:govet // shadow in if-init is idiomatic
		return nil, ErrNoCookiecutterConfig
	}

	// 3. Determine output directory
	destDir := opts.Destination
	if destDir == "" {
		// Default: same directory with -tag suffix
		baseName := filepath.Base(templateDir)
		baseName = strings.TrimPrefix(baseName, "cookiecutter-")
		destDir = baseName + "-tag"
	}
	result.Destination = destDir

	// 4. Check if output exists
	if !opts.DryRun {
		if prepErr := prepareDestDir(destDir, opts.Force); prepErr != nil {
			return nil, prepErr
		}
	}

	// 5. Read and convert cookiecutter.json
	configData, err := os.ReadFile(cookiecutterPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cookiecutter.json: %w", err)
	}

	tagConfig, conversions, warnings, err := ConvertCookiecutterConfig(configData)
	if err != nil {
		return nil, err
	}
	result.VariablesConverted = len(conversions)
	result.Variables = append(result.Variables, conversions...)
	result.Warnings = append(result.Warnings, warnings...)

	// 6. Process hooks
	hooksProcessor := NewHooksProcessor(templateDir, destDir, opts.DryRun)
	hookFindings, err := hooksProcessor.CopyHooks()
	if err != nil {
		return nil, fmt.Errorf("failed to process hooks: %w", err)
	}
	result.HooksCopied = len(hookFindings)
	for _, hf := range hookFindings {
		result.Warnings = append(result.Warnings, hf.Message)
	}

	// Add shell hooks to config
	preHooks, postHooks := SuggestTagHooksConfig(hookFindings)
	if len(preHooks) > 0 || len(postHooks) > 0 {
		tagConfig.Hooks = &types.HooksConfig{
			PreScaffold:  preHooks,
			PostScaffold: postHooks,
		}
	}

	// 7. Walk and convert template files
	err = c.processTemplateFiles(templateDir, destDir, result, opts.DryRun, remote.IsLocal(opts.Source))
	if err != nil {
		return nil, err
	}

	// Every converted template puts hooks/ beside the wrapper directory, so
	// without excluding it a scaffold run would treat the root as "mixed" and
	// stop unwrapping (#403) — and the hook script would leak into the
	// generated project. This runs after the walk so it appends to (rather
	// than gets clobbered by) a .tagignore the source template already ships.
	if !opts.DryRun && len(hookFindings) > 0 {
		if err := writeHooksTagIgnore(destDir); err != nil {
			return nil, err
		}
	}

	// 8. Write tag.template.json
	if !opts.DryRun {
		// Ensure destination directory exists
		if err := os.MkdirAll(destDir, types.DirMode); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}

		tagJSON, err := GenerateTagTemplateJSON(tagConfig, "", "Converted from Cookiecutter template")
		if err != nil {
			return nil, fmt.Errorf("failed to generate tag.template.json: %w", err)
		}

		tagConfigPath := filepath.Join(destDir, types.TemplateConfigFile)
		if err := os.WriteFile(tagConfigPath, tagJSON, types.FileMode); err != nil {
			return nil, fmt.Errorf("failed to write tag.template.json: %w", err)
		}
	}

	return result, nil
}

// prepareDestDir validates and, when force is set, clears an existing
// destination directory before a real (non-dry-run) conversion.
func prepareDestDir(destDir string, force bool) error {
	absDestDir, err := filepath.Abs(destDir)
	if err != nil {
		return fmt.Errorf("failed to resolve output path: %w", err)
	}
	if absDestDir == "/" || absDestDir == "." || destDir == "" {
		return fmt.Errorf("unsafe output directory: %s", destDir)
	}

	if info, statErr := os.Stat(destDir); statErr == nil && info.IsDir() {
		if !force {
			return fmt.Errorf("%w: %s (use --force to overwrite)", ErrOutputExists, destDir)
		}
		if err := os.RemoveAll(destDir); err != nil {
			return fmt.Errorf("failed to remove existing output: %w", err)
		}
	}

	return nil
}

// writeHooksTagIgnore ensures destDir/.tagignore excludes the hooks/
// directory that CopyHooks just populated, appending to an existing file
// rather than clobbering it.
func writeHooksTagIgnore(destDir string) error {
	path := filepath.Join(destDir, types.TagIgnoreFile)
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", types.TagIgnoreFile, err)
	}

	for line := range strings.SplitSeq(string(existing), "\n") {
		if strings.TrimSpace(line) == "hooks/" {
			return nil
		}
	}

	content := string(existing)
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "hooks/\n"

	// Rewriting the file must not widen a restrictive mode the source template
	// chose for its own .tagignore.
	mode := types.FileMode
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	}

	if err := fileutil.WriteFileAtomic(path, []byte(content), mode); err != nil {
		return fmt.Errorf("failed to write %s: %w", types.TagIgnoreFile, err)
	}
	return nil
}

// resolveSource resolves a source reference to a local directory.
func (c *Converter) resolveSource(ctx context.Context, source string) (string, error) {
	// Check if it's a local path
	if info, err := os.Stat(source); err == nil && info.IsDir() {
		absPath, err := filepath.Abs(source)
		if err != nil {
			return "", fmt.Errorf("failed to get absolute path: %w", err)
		}
		return absPath, nil
	}

	// Try to resolve as remote
	resolveResult, err := c.resolver.Resolve(ctx, source, remote.ResolveOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to resolve template: %w", err)
	}

	return resolveResult.Path, nil
}

// processTemplateFiles walks the template directory and converts files.
// Uses filepath.WalkDir instead of filepath.Walk to avoid following symlinked directories.
// processTemplateFiles walks srcDir and converts the files beneath it.
//
// resolveRoot must be set only for a source the user named on their own
// filesystem. filepath.WalkDir does not descend into a symlinked root, so
// without it a symlinked local template converts to nothing at exit 0. It is
// off for a fetched source because a repository can commit its subpath as a
// symlink pointing anywhere, and following that would copy the target into the
// converted template.
//
// The resolve lives here rather than in resolveSource because Convert derives
// the default destination from filepath.Base(templateDir); resolving earlier
// would turn `tag convert cookiecutter ./linked` output from linked-tag into
// <target>-tag. Everything else under templateDir reaches through the symlink
// as an intermediate component and already works.
func (c *Converter) processTemplateFiles(srcDir, destDir string, result *Result, dryRun, resolveRoot bool) error {
	if resolveRoot {
		resolved, err := fileutil.ResolveSymlinkedRoot(srcDir)
		if err != nil {
			return err
		}
		srcDir = resolved
	}

	return c.walkTemplateFiles(srcDir, destDir, result, dryRun)
}

func (c *Converter) walkTemplateFiles(srcDir, destDir string, result *Result, dryRun bool) error {
	return filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		// Skip root
		if relPath == "." {
			return nil
		}

		// Skip cookiecutter.json (we handle it separately)
		if relPath == types.CookiecutterConfigFile {
			return nil
		}

		// Skip hooks directory (handled separately)
		if IsHooksDir(relPath) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip .git directory only (preserve other dotfiles like .gitignore, .github)
		baseName := filepath.Base(relPath)
		if baseName == ".git" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip symlinks for security (prevent traversal attacks)
		// WalkDir does not follow symlinked directories, but we still skip symlinked files
		if d.Type()&os.ModeSymlink != 0 {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("skipped symlink: %s (symlinks not copied for security)", relPath))
			return nil
		}

		// Convert path placeholders
		convertedPath, pathChanged := ConvertPath(relPath)
		if pathChanged {
			if d.IsDir() {
				result.DirsRenamed++
			} else {
				result.FilesRenamed++
			}
		}

		destPath := filepath.Join(destDir, convertedPath)

		if d.IsDir() {
			// Create directory
			if !dryRun {
				var info fs.FileInfo
				info, err = d.Info()
				if err != nil {
					return fmt.Errorf("failed to get directory info %s: %w", destPath, err)
				}
				if err := os.MkdirAll(destPath, info.Mode()); err != nil { //nolint:govet // shadow in if-init is idiomatic
					return fmt.Errorf("failed to create directory %s: %w", destPath, err)
				}
			}
			return nil
		}

		// Process file
		result.FilesProcessed++
		result.Files = append(result.Files, PathConversion{From: relPath, To: convertedPath})

		// Read source content
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", relPath, err)
		}

		// Analyze content for incompatibilities
		findings := c.analyzer.Analyze(relPath, content)
		result.Incompatibilities = append(result.Incompatibilities, findings...)

		// Write to destination
		//nolint:nestif // dry-run branching requires nested conditions for file operations
		if !dryRun {
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(destPath), types.DirMode); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			// For text files, convert cookiecutter.* references to vars.*
			if fileutil.IsTextContent(content) {
				converted, changed := ConvertContent(string(content))
				if changed {
					info, infoErr := d.Info()
					if infoErr != nil {
						return fmt.Errorf("failed to get file info %s: %w", relPath, infoErr)
					}
					if err := os.WriteFile(destPath, []byte(converted), info.Mode()); err != nil {
						return fmt.Errorf("failed to write %s: %w", relPath, err)
					}
					return nil
				}
			}

			// Binary files or text without cookiecutter references: raw copy
			if err := fileutil.CopyFile(path, destPath); err != nil {
				return fmt.Errorf("failed to copy %s: %w", relPath, err)
			}
		}

		return nil
	})
}
