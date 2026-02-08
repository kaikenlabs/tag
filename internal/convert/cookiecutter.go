package convert

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaikenlabs/tag/internal/fileutil"
	"github.com/kaikenlabs/tag/internal/remote"
	"github.com/kaikenlabs/tag/internal/scaffold"
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
	result := &Result{
		Source: opts.Source,
		DryRun: opts.DryRun,
	}

	// 1. Resolve source template (handles local and remote)
	templateDir, err := c.resolveSource(ctx, opts.Source)
	if err != nil {
		return nil, err
	}

	// 2. Verify it's a Cookiecutter template
	cookiecutterPath := filepath.Join(templateDir, "cookiecutter.json")
	if _, err := os.Stat(cookiecutterPath); os.IsNotExist(err) {
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
		// Safety check: prevent dangerous --force operations
		absDestDir, err := filepath.Abs(destDir)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve output path: %w", err)
		}
		if absDestDir == "/" || absDestDir == "." || destDir == "" {
			return nil, fmt.Errorf("unsafe output directory: %s", destDir)
		}

		if info, err := os.Stat(destDir); err == nil && info.IsDir() {
			if !opts.Force {
				return nil, fmt.Errorf("%w: %s (use --force to overwrite)", ErrOutputExists, destDir)
			}
			// Remove existing if force
			if err := os.RemoveAll(destDir); err != nil {
				return nil, fmt.Errorf("failed to remove existing output: %w", err)
			}
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
		tagConfig.Hooks = &scaffold.HooksConfig{
			PreScaffold:  preHooks,
			PostScaffold: postHooks,
		}
	}

	// 7. Walk and convert template files
	err = c.processTemplateFiles(templateDir, destDir, tagConfig, result, opts.DryRun)
	if err != nil {
		return nil, err
	}

	// 8. Write tag.template.json
	if !opts.DryRun {
		// Ensure destination directory exists
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}

		tagJSON, err := GenerateTagTemplateJSON(tagConfig, "", "Converted from Cookiecutter template")
		if err != nil {
			return nil, fmt.Errorf("failed to generate tag.template.json: %w", err)
		}

		tagConfigPath := filepath.Join(destDir, "tag.template.json")
		if err := os.WriteFile(tagConfigPath, tagJSON, 0o644); err != nil {
			return nil, fmt.Errorf("failed to write tag.template.json: %w", err)
		}
	}

	return result, nil
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
	resolvedPath, err := c.resolver.Resolve(ctx, source, remote.ResolveOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to resolve template: %w", err)
	}

	return resolvedPath, nil
}

// processTemplateFiles walks the template directory and converts files.
// Uses filepath.WalkDir instead of filepath.Walk to avoid following symlinked directories.
func (c *Converter) processTemplateFiles(srcDir, destDir string, config *scaffold.TemplateConfig, result *Result, dryRun bool) error {
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
		if relPath == "cookiecutter.json" {
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
				info, err := d.Info()
				if err != nil {
					return fmt.Errorf("failed to get directory info %s: %w", destPath, err)
				}
				if err := os.MkdirAll(destPath, info.Mode()); err != nil {
					return fmt.Errorf("failed to create directory %s: %w", destPath, err)
				}
			}
			return nil
		}

		// Process file
		result.FilesProcessed++

		// Read source content
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", relPath, err)
		}

		// Analyze content for incompatibilities
		findings := c.analyzer.Analyze(relPath, content)
		result.Incompatibilities = append(result.Incompatibilities, findings...)

		// Write to destination
		if !dryRun {
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			// Copy file
			if err := fileutil.CopyFile(path, destPath); err != nil {
				return fmt.Errorf("failed to copy %s: %w", relPath, err)
			}
		}

		return nil
	})
}

// ConvertInPlace converts a Cookiecutter template in-place by writing
// tag.template.json to the same directory as cookiecutter.json.
// Unlike Convert(), this does not copy files to a new directory.
func (c *Converter) ConvertInPlace(ctx context.Context, templateDir string) error {
	// Verify it's a Cookiecutter template
	cookiecutterPath := filepath.Join(templateDir, "cookiecutter.json")
	if _, err := os.Stat(cookiecutterPath); os.IsNotExist(err) {
		return ErrNoCookiecutterConfig
	}

	// Read and convert cookiecutter.json
	configData, err := os.ReadFile(cookiecutterPath)
	if err != nil {
		return fmt.Errorf("failed to read cookiecutter.json: %w", err)
	}

	tagConfig, _, _, err := ConvertCookiecutterConfig(configData)
	if err != nil {
		return err
	}

	// Process hooks (detect only, don't copy - they're already in place)
	hooksProcessor := NewHooksProcessor(templateDir, templateDir, false)
	hookFindings, err := hooksProcessor.DetectHooks()
	if err != nil {
		return fmt.Errorf("failed to detect hooks: %w", err)
	}

	// Add shell hooks to config
	preHooks, postHooks := SuggestTagHooksConfig(hookFindings)
	if len(preHooks) > 0 || len(postHooks) > 0 {
		tagConfig.Hooks = &scaffold.HooksConfig{
			PreScaffold:  preHooks,
			PostScaffold: postHooks,
		}
	}

	// Write tag.template.json
	tagJSON, err := GenerateTagTemplateJSON(tagConfig, "", "Converted from Cookiecutter template")
	if err != nil {
		return fmt.Errorf("failed to generate tag.template.json: %w", err)
	}

	tagConfigPath := filepath.Join(templateDir, "tag.template.json")
	if err := os.WriteFile(tagConfigPath, tagJSON, 0o644); err != nil {
		return fmt.Errorf("failed to write tag.template.json: %w", err)
	}

	return nil
}
