package library

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kaikenlabs/tag/internal/types"
)

const registryFile = "library.json"

// Store manages the registry file on disk.
// It is responsible solely for loading and saving the Registry —
// all business logic lives in Library.
type Store struct {
	dataDir string
}

func newStore(dataDir string) *Store {
	return &Store{dataDir: dataDir}
}

// load reads the registry from disk.
// Returns an empty registry if the file does not exist.
//
// NOTE: The load-modify-save cycle is NOT safe for concurrent access
// across processes. This is acceptable for a CLI tool where concurrent
// library operations are uncommon. If concurrent safety is needed,
// add advisory file locking (e.g., flock) around the entire cycle.
func (s *Store) load() (*Registry, error) {
	path := filepath.Join(s.dataDir, registryFile)

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Registry{Entries: make(map[string]*Entry)}, nil
		}
		return nil, fmt.Errorf("read registry: %w", err)
	}

	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parse registry: %w", err)
	}

	if reg.Entries == nil {
		reg.Entries = make(map[string]*Entry)
	}

	// Reject registries from a newer version that we don't understand
	if reg.Version > registryVersion {
		return nil, fmt.Errorf("registry version %d is newer than supported version %d; upgrade tag to read this library", reg.Version, registryVersion)
	}
	reg.Version = registryVersion

	return &reg, nil
}

// save writes the registry to disk atomically (temp file + rename).
// See load for concurrency safety notes.
func (s *Store) save(reg *Registry) error {
	reg.Version = registryVersion
	if err := os.MkdirAll(s.dataDir, types.DirModePrivate); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}

	path := filepath.Join(s.dataDir, registryFile)
	tempPath := path + ".tmp"

	if err := os.WriteFile(tempPath, data, types.FileModePrivate); err != nil {
		return fmt.Errorf("write registry: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("finalize registry: %w", err)
	}

	return nil
}
