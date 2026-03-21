package dialect

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// NewRegistry creates an empty dialect registry.
func NewRegistry() *Registry {
	return &Registry{
		dialects: make(map[string]*Dialect),
	}
}

// Load parses a single YAML dialect definition and merges it into the registry.
// If a dialect with the same name already exists, the type mappings are deep-merged
// (individual type keys from the new definition override existing ones).
func (r *Registry) Load(data []byte) error {
	var d Dialect
	if err := yaml.Unmarshal(data, &d); err != nil {
		return fmt.Errorf("failed to parse dialect YAML: %w", err)
	}

	if d.Name == "" {
		return errors.New("dialect YAML is missing required 'name' field")
	}

	// Ensure Types map is initialized to prevent nil map panics.
	if d.Types == nil {
		d.Types = make(map[string]string)
	}

	existing, ok := r.dialects[d.Name]
	if !ok {
		r.dialects[d.Name] = &d
		return nil
	}

	// Deep merge: override individual type mappings.
	if d.Description != "" {
		existing.Description = d.Description
	}
	if existing.Types == nil {
		existing.Types = make(map[string]string)
	}
	maps.Copy(existing.Types, d.Types)

	return nil
}

// LoadDir loads all *.yaml files from a directory on disk into the registry.
// Files are loaded in lexical order for deterministic results.
// Returns nil if the directory does not exist.
func (r *Registry) LoadDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read dialect directory %s: %w", dir, err)
	}

	return r.loadDirEntries(dir, entries)
}

// LoadFS loads all *.yaml files from a directory within an fs.FS into the registry.
// Files are loaded in lexical order for deterministic results.
// Uses forward-slash path joining as required by the fs.FS interface.
func (r *Registry) LoadFS(fsys fs.FS, dir string) error {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return fmt.Errorf("failed to read dialect directory %s in FS: %w", dir, err)
	}

	readFile := func(name string) ([]byte, error) {
		// fs.FS requires forward-slash paths, not filepath.Join.
		return fs.ReadFile(fsys, path.Join(dir, name))
	}

	return r.loadFSEntries(entries, readFile)
}

// loadDirEntries loads dialect YAML files from an OS directory.
func (r *Registry) loadDirEntries(dir string, entries []fs.DirEntry) error {
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return fmt.Errorf("failed to read dialect file %s: %w", entry.Name(), err)
		}

		if err := r.Load(data); err != nil {
			return fmt.Errorf("failed to load dialect from %s: %w", entry.Name(), err)
		}
	}

	return nil
}

// loadFSEntries loads dialect YAML files using a readFile callback.
// Used by LoadFS where path joining is handled by the caller.
func (r *Registry) loadFSEntries(entries []fs.DirEntry, readFile func(string) ([]byte, error)) error {
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		data, err := readFile(entry.Name())
		if err != nil {
			return fmt.Errorf("failed to read dialect file %s: %w", entry.Name(), err)
		}

		if err := r.Load(data); err != nil {
			return fmt.Errorf("failed to load dialect from %s: %w", entry.Name(), err)
		}
	}

	return nil
}

// Resolve looks up the target type for a canonical type name in the given dialect.
// Returns ErrUnknownDialect if the dialect does not exist.
// Returns ErrUnmappedType if the canonical type has no mapping in the dialect.
func (r *Registry) Resolve(canonicalType, dialectName string) (string, error) {
	d, ok := r.dialects[dialectName]
	if !ok {
		return "", fmt.Errorf("%w: %q (available: %s)", ErrUnknownDialect, dialectName, strings.Join(r.Names(), ", "))
	}

	target, ok := d.Types[canonicalType]
	if !ok {
		knownTypes := make([]string, 0, len(d.Types))
		for k := range d.Types {
			knownTypes = append(knownTypes, k)
		}
		sort.Strings(knownTypes)
		return "", fmt.Errorf("%w: %q in dialect %q (available: %s)", ErrUnmappedType, canonicalType, dialectName, strings.Join(knownTypes, ", "))
	}

	return target, nil
}

// Names returns the sorted names of all dialects in the registry.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.dialects))
	for name := range r.dialects {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Get returns the dialect with the given name, or nil if it does not exist.
// The returned Dialect must not be modified by callers.
func (r *Registry) Get(name string) *Dialect {
	return r.dialects[name]
}
