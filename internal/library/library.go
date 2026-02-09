package library

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/kaikenlabs/tag/internal/convert"
	"github.com/kaikenlabs/tag/internal/fileutil"
	"github.com/kaikenlabs/tag/internal/remote"
	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/types"
)

const (
	templatesDir = "templates"
	maxNameLen   = 255
)

// Resolver resolves template references to local directories.
type Resolver interface {
	Resolve(ctx context.Context, input string, opts remote.ResolveOptions) (string, error)
}

// Library manages a persistent collection of installed templates.
type Library struct {
	dataDir  string
	resolver Resolver
}

// New creates a Library using the XDG data directory.
func New(dataDir string) (*Library, error) {
	resolver, err := remote.NewResolver()
	if err != nil {
		return nil, fmt.Errorf("create resolver: %w", err)
	}

	return &Library{
		dataDir:  dataDir,
		resolver: resolver,
	}, nil
}

// NewLocal creates a Library without a resolver (for local-only operations like ls, inspect, edit, rm).
// Operations that require resolving remote templates (add, update) will return an error.
func NewLocal(dataDir string) *Library {
	return &Library{dataDir: dataDir}
}

// NewWithDir creates a Library with explicit dependencies (for testing).
func NewWithDir(dataDir string, resolver Resolver) *Library {
	return &Library{
		dataDir:  dataDir,
		resolver: resolver,
	}
}

// Add installs a template into the library.
func (l *Library) Add(ctx context.Context, opts AddOptions) (*AddResult, error) {
	// Derive name
	name := opts.Name
	autoDerived := name == ""
	if autoDerived {
		name = deriveName(opts.Ref)
	}

	if err := validateName(name); err != nil {
		if autoDerived {
			return nil, &LibraryError{
				Name:      name,
				Operation: "add",
				Err:       fmt.Errorf("%w (auto-derived from ref; use --as to specify a name)", err),
			}
		}
		return nil, &LibraryError{Name: name, Operation: "add", Err: err}
	}

	// Load registry
	reg, err := loadRegistry(l.dataDir)
	if err != nil {
		return nil, &LibraryError{Name: name, Operation: "add", Err: err}
	}

	// Check for existing entry
	isUpdate := false
	if _, exists := reg.Entries[name]; exists {
		if !opts.Force {
			return nil, &LibraryError{
				Name:      name,
				Operation: "add",
				Err:       fmt.Errorf("%w: use --force to overwrite", ErrTemplateExists),
			}
		}
		isUpdate = true
	}

	// Resolve template (fetch from remote if needed)
	if l.resolver == nil {
		return nil, &LibraryError{Name: name, Operation: "add", Err: errors.New("resolver not configured")}
	}
	resolvedDir, err := l.resolver.Resolve(ctx, opts.Ref, remote.ResolveOptions{
		ForceUpdate: true,
	})
	if err != nil {
		return nil, &LibraryError{Name: name, Operation: "add", Err: fmt.Errorf("resolve template: %w", err)}
	}

	destPath := filepath.Join(l.dataDir, templatesDir, name)

	// Store template (convert if Cookiecutter, copy if TAG)
	result, err := storeTemplate(ctx, resolvedDir, destPath, name, opts, isUpdate)
	if err != nil {
		return nil, err
	}

	// Read template metadata (non-fatal on error)
	version, description, metaErr := readTemplateMetadata(destPath)
	if metaErr != nil {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("failed to read template metadata: %v", metaErr))
	}

	// Update registry
	now := time.Now().UTC()
	entry := &Entry{
		Name:          name,
		Source:        opts.Ref,
		UpdatedAt:     now,
		Version:       version,
		Description:   description,
		ConvertedFrom: result.ConvertedFrom,
	}
	if isUpdate {
		// Preserve original AddedAt
		if prev, ok := reg.Entries[name]; ok {
			entry.AddedAt = prev.AddedAt
		}
	} else {
		entry.AddedAt = now
	}
	reg.Entries[name] = entry

	if err := saveRegistry(l.dataDir, reg); err != nil {
		return nil, &LibraryError{Name: name, Operation: "add", Err: err}
	}

	return result, nil
}

// storeTemplate copies or converts a template into the library destination.
// Uses atomic swap when updating or when the destination already exists (inconsistent state).
// Uses deferred cleanup for fresh adds to remove partial writes on failure.
func storeTemplate(ctx context.Context, resolvedDir, destPath, name string, opts AddOptions, isUpdate bool) (*AddResult, error) {
	result := &AddResult{
		Name:        name,
		Source:      opts.Ref,
		IsUpdate:    isUpdate,
		TemplateDir: destPath,
	}

	// Use atomic swap for updates, or if dest exists on disk despite not being in registry
	// (inconsistent state — avoid data loss).
	if isUpdate || pathExists(destPath) {
		return storeTemplateAtomic(ctx, resolvedDir, destPath, name, opts, result)
	}

	return storeTemplateFresh(ctx, resolvedDir, destPath, name, opts, result)
}

// pathExists reports whether a path exists on disk (using Lstat to not follow symlinks).
func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// storeTemplateFresh writes to destPath directly with deferred cleanup on failure.
func storeTemplateFresh(ctx context.Context, resolvedDir, destPath, name string, opts AddOptions, result *AddResult) (*AddResult, error) {
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(destPath)
		}
	}()

	if err := storeToDir(ctx, resolvedDir, destPath, opts, result); err != nil {
		return nil, &LibraryError{Name: name, Operation: "add", Err: err}
	}

	success = true
	return result, nil
}

// storeTemplateAtomic writes to a temp dir, then swaps with the existing destPath
// using a three-step pattern: rename old → rename new → remove old backup.
// If the process dies between steps, the .old backup exists for recovery.
func storeTemplateAtomic(ctx context.Context, resolvedDir, destPath, name string, opts AddOptions, result *AddResult) (*AddResult, error) {
	tmpDest := destPath + ".new"
	_ = secureRemoveAll(tmpDest) // clean up any leftover from a previous failed attempt

	success := false
	defer func() {
		if !success {
			_ = secureRemoveAll(tmpDest)
		}
	}()

	if err := storeToDir(ctx, resolvedDir, tmpDest, opts, result); err != nil {
		return nil, &LibraryError{Name: name, Operation: "add", Err: err}
	}

	// Three-step swap: (1) move old to .old, (2) move new into place, (3) remove .old
	backupPath := destPath + ".old"
	_ = secureRemoveAll(backupPath) // clean up any leftover .old from a previous crash

	// Step 1: move existing directory to backup (may not exist for fresh-routed calls)
	if _, err := os.Lstat(destPath); err == nil {
		if err := os.Rename(destPath, backupPath); err != nil {
			return nil, &LibraryError{Name: name, Operation: "add", Err: fmt.Errorf("backup old template: %w", err)}
		}
	}

	// Step 2: move new directory into place
	if err := os.Rename(tmpDest, destPath); err != nil {
		// Try to restore backup
		if _, restoreErr := os.Lstat(backupPath); restoreErr == nil {
			_ = os.Rename(backupPath, destPath)
		}
		return nil, &LibraryError{Name: name, Operation: "add", Err: fmt.Errorf("finalize template: %w", err)}
	}

	// Step 3: clean up backup (non-fatal)
	_ = secureRemoveAll(backupPath)

	success = true
	result.TemplateDir = destPath
	return result, nil
}

// storeToDir detects the template type and copies or converts into targetDir.
func storeToDir(ctx context.Context, resolvedDir, targetDir string, opts AddOptions, result *AddResult) error {
	_, isCookiecutter := scaffold.IsCookiecutterTemplate(resolvedDir)
	if isCookiecutter {
		return storeCookiecutterToDir(ctx, resolvedDir, targetDir, opts, result)
	}

	if err := copyDir(resolvedDir, targetDir); err != nil {
		return fmt.Errorf("copy template: %w", err)
	}
	return nil
}

// storeCookiecutterToDir converts a Cookiecutter template into targetDir.
func storeCookiecutterToDir(ctx context.Context, resolvedDir, targetDir string, opts AddOptions, result *AddResult) error {
	converter, err := convert.NewConverter()
	if err != nil {
		return fmt.Errorf("create converter: %w", err)
	}

	convResult, err := converter.Convert(ctx, convert.Options{
		Source:      resolvedDir,
		Destination: targetDir,
		Force:       opts.Force,
	})
	if err != nil {
		return fmt.Errorf("convert template: %w", err)
	}

	result.ConvertedFrom = "cookiecutter"
	result.Warnings = convResult.Warnings
	for _, inc := range convResult.Incompatibilities {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%s:%d - %s", inc.Path, inc.Line, inc.Kind))
	}

	return nil
}

// Remove deletes a template from the library.
func (l *Library) Remove(name string) error {
	if err := validateName(name); err != nil {
		return &LibraryError{Name: name, Operation: "remove", Err: err}
	}

	reg, err := loadRegistry(l.dataDir)
	if err != nil {
		return &LibraryError{Name: name, Operation: "remove", Err: err}
	}

	if _, exists := reg.Entries[name]; !exists {
		return &LibraryError{Name: name, Operation: "remove", Err: ErrTemplateNotFound}
	}

	// Remove template directory (symlink-safe)
	templatePath := filepath.Join(l.dataDir, templatesDir, name)
	if err := secureRemoveAll(templatePath); err != nil {
		return &LibraryError{Name: name, Operation: "remove", Err: fmt.Errorf("remove directory: %w", err)}
	}

	// Remove registry entry
	delete(reg.Entries, name)

	if err := saveRegistry(l.dataDir, reg); err != nil {
		return &LibraryError{Name: name, Operation: "remove", Err: err}
	}

	return nil
}

// List returns all entries sorted by name.
func (l *Library) List() ([]*Entry, error) {
	reg, err := loadRegistry(l.dataDir)
	if err != nil {
		return nil, &LibraryError{Operation: "list", Err: err}
	}

	entries := make([]*Entry, 0, len(reg.Entries))
	for _, entry := range reg.Entries {
		entries = append(entries, entry)
	}

	slices.SortFunc(entries, func(a, b *Entry) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return entries, nil
}

// Get returns a single entry or ErrTemplateNotFound.
func (l *Library) Get(name string) (*Entry, error) {
	if err := validateName(name); err != nil {
		return nil, &LibraryError{Name: name, Operation: "get", Err: err}
	}

	reg, err := loadRegistry(l.dataDir)
	if err != nil {
		return nil, &LibraryError{Name: name, Operation: "get", Err: err}
	}

	entry, exists := reg.Entries[name]
	if !exists {
		return nil, &LibraryError{Name: name, Operation: "get", Err: ErrTemplateNotFound}
	}

	return entry, nil
}

// Update re-fetches a template from its original source.
func (l *Library) Update(ctx context.Context, name string) (*AddResult, error) {
	entry, err := l.Get(name)
	if err != nil {
		return nil, err
	}

	return l.Add(ctx, AddOptions{
		Ref:   entry.Source,
		Name:  name,
		Force: true,
	})
}

// UpdateAll updates every template in the library.
// It continues on individual failures and returns all results plus any errors.
func (l *Library) UpdateAll(ctx context.Context) ([]*AddResult, error) {
	entries, err := l.List()
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 {
		return nil, &LibraryError{Operation: "update-all", Err: ErrEmptyLibrary}
	}

	var (
		results []*AddResult
		errs    []error
	)
	for _, entry := range entries {
		result, err := l.Update(ctx, entry.Name)
		if err != nil {
			errs = append(errs, fmt.Errorf("update %q: %w", entry.Name, err))
			continue
		}
		results = append(results, result)
	}

	return results, errors.Join(errs...)
}

// TemplatePath returns the absolute path to a stored template directory.
func (l *Library) TemplatePath(name string) (string, error) {
	// Get validates the name and verifies it exists in registry
	if _, err := l.Get(name); err != nil {
		return "", err
	}

	path := filepath.Join(l.dataDir, templatesDir, name)

	// Verify directory exists on disk using Lstat (does not follow symlinks)
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", &LibraryError{
				Name:      name,
				Operation: "path",
				Err:       fmt.Errorf("template directory missing from disk: %w", ErrTemplateNotFound),
			}
		}
		return "", &LibraryError{Name: name, Operation: "path", Err: err}
	}

	// Reject symlinks — the path should point to a real directory
	if info.Mode()&os.ModeSymlink != 0 {
		return "", &LibraryError{
			Name:      name,
			Operation: "path",
			Err:       fmt.Errorf("template path is a symlink: %w", ErrTemplateNotFound),
		}
	}

	return path, nil
}

// deriveName extracts a template name from a reference string.
func deriveName(ref string) string {
	// Try to parse as a remote reference to get the repo name
	parsed, err := remote.Parse(ref)
	if err == nil && parsed.Repo != "" {
		name := parsed.Repo
		// Strip common cookiecutter- prefix
		name = strings.TrimPrefix(name, "cookiecutter-")
		return name
	}

	// Fallback: use base name of the path
	base := filepath.Base(ref)
	base = strings.TrimPrefix(base, "cookiecutter-")
	base = strings.TrimSuffix(base, ".git")
	return base
}

// validateName checks that a template name is valid for use as a directory name.
func validateName(name string) error {
	if name == "" {
		return ErrInvalidName
	}
	if len(name) > maxNameLen {
		return fmt.Errorf("%w: exceeds maximum length of %d", ErrInvalidName, maxNameLen)
	}
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("%w: contains path separator", ErrInvalidName)
	}
	if name == "." || name == ".." {
		return fmt.Errorf("%w: reserved name", ErrInvalidName)
	}
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("%w: name cannot start with a dot", ErrInvalidName)
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7F {
			return fmt.Errorf("%w: contains control character", ErrInvalidName)
		}
	}
	return nil
}

// readTemplateMetadata reads version and description from tag.template.json.
// Returns empty strings if the config file does not exist.
// Returns an error if the file exists but cannot be parsed.
func readTemplateMetadata(templateDir string) (version, description string, err error) {
	configPath := filepath.Join(templateDir, types.TemplateConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", nil
		}
		return "", "", fmt.Errorf("read %s: %w", types.TemplateConfigFile, err)
	}

	config, err := scaffold.ParseTemplateConfig(data)
	if err != nil {
		return "", "", fmt.Errorf("parse %s: %w", types.TemplateConfigFile, err)
	}

	return config.Version, config.Description, nil
}

// secureRemoveAll removes a path safely. If the path is a symlink, it removes
// only the link (not the target). For regular files and directories, it delegates
// to os.RemoveAll. Returns nil if the path does not exist.
func secureRemoveAll(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	// If it's a symlink, remove only the link itself
	if info.Mode()&os.ModeSymlink != 0 {
		return os.Remove(path)
	}

	return os.RemoveAll(path)
}

// copyDir copies a directory tree from src to dst, skipping symlinks.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip symlinks (detected at walk time via Lstat)
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, types.DirModePrivate)
		}

		if mkdirErr := os.MkdirAll(filepath.Dir(destPath), types.DirModePrivate); mkdirErr != nil {
			return mkdirErr
		}

		// TOCTOU mitigation: re-check with Lstat before copy to confirm
		// the file hasn't been replaced with a symlink since WalkDir's Lstat.
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil // skip: became a symlink between walk and copy
		}

		return fileutil.CopyFile(path, destPath)
	})
}
