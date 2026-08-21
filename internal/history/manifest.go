package history

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/kaikenlabs/tag/internal/types"
)

// Load reads the manifest from tagDir/.tag/history.json.
// If the file does not exist, it returns an empty manifest (not an error).
func Load(tagDir string) (Manifest, error) {
	path := manifestPath(tagDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Manifest{}, nil
		}
		return Manifest{}, fmt.Errorf("read history manifest: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse history manifest: %w", err)
	}
	return m, nil
}

// Save writes the manifest to tagDir/.tag/history.json atomically
// (write to a temp file in the same directory, then rename).
func Save(tagDir string, m Manifest) error {
	if err := os.MkdirAll(tagDir, types.DirMode); err != nil {
		return fmt.Errorf("create tag dir: %w", err)
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal history manifest: %w", err)
	}

	target := manifestPath(tagDir)

	tmpFile, err := os.CreateTemp(tagDir, filepath.Base(target)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp manifest: %w", err)
	}
	tmp := tmpFile.Name()

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		_ = os.Remove(tmp) //nolint:gosec // G703: tmp is from os.CreateTemp, not user-controlled
		return fmt.Errorf("write temp manifest: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmp) //nolint:gosec // G703: tmp is from os.CreateTemp, not user-controlled
		return fmt.Errorf("write temp manifest: %w", err)
	}

	// os.CreateTemp creates 0600, but the manifest has always been
	// world-readable (types.FileMode, 0644) - restore it before publishing.
	if err := os.Chmod(tmp, types.FileMode); err != nil { //nolint:gosec // G703: tmp is from os.CreateTemp, not user-controlled
		_ = os.Remove(tmp) //nolint:gosec // G703: tmp is from os.CreateTemp, not user-controlled
		return fmt.Errorf("chmod temp manifest: %w", err)
	}

	if err := os.Rename(tmp, target); err != nil { //nolint:gosec // G703: tmp is from os.CreateTemp, not user-controlled
		_ = os.Remove(tmp) //nolint:gosec // G703: tmp is from os.CreateTemp, not user-controlled
		return fmt.Errorf("rename manifest: %w", err)
	}
	return nil
}

// Append loads the manifest, appends g, and saves it atomically.
func Append(tagDir string, g Generation) error {
	m, err := Load(tagDir)
	if err != nil {
		return err
	}
	m.Generations = append(m.Generations, g)
	return Save(tagDir, m)
}

// Remove loads the manifest, removes the generation with id, and saves it.
// Returns ErrNotFound if no such generation exists.
func Remove(tagDir, id string) error {
	m, err := Load(tagDir)
	if err != nil {
		return err
	}
	idx := indexByID(m, id)
	if idx < 0 {
		return ErrNotFound
	}
	m.Generations = append(m.Generations[:idx], m.Generations[idx+1:]...)
	return Save(tagDir, m)
}

func manifestPath(tagDir string) string {
	return filepath.Join(tagDir, types.HistoryFile)
}

func indexByID(m Manifest, id string) int {
	for i, g := range m.Generations {
		if g.ID == id {
			return i
		}
	}
	return -1
}
