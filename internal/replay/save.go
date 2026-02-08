package replay

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kaikenlabs/tag/internal/types"
)

// Save saves replay data for the given template source.
// It creates the replay directory if it doesn't exist.
// Secret variables (keys in the secrets map set to true) are excluded from the saved values.
//
// The replay file is saved to ~/.tag/replay/<template-id>.json with permissions 0600.
// The replay directory is created with permissions 0700.
func Save(templateSource, version string, values map[string]any, secrets map[string]bool) error {
	templateSource = strings.TrimSpace(templateSource)
	if templateSource == "" {
		return ErrEmptyTemplateSource
	}

	templateID := GenerateTemplateID(templateSource)
	if templateID == "" {
		return ErrEmptyTemplateSource
	}

	// Get replay directory path
	replayDir, err := getReplayDir()
	if err != nil {
		return NewReplayError(templateID, "save", fmt.Errorf("failed to get replay directory: %w", err))
	}

	// Ensure replay directory exists with secure permissions
	if err := os.MkdirAll(replayDir, types.DirModePrivate); err != nil {
		return NewReplayError(templateID, "save", fmt.Errorf("failed to create replay directory: %w", err))
	}

	// Filter out secret values
	filteredValues := filterSecrets(values, secrets)

	// Create replay data
	data := ReplayData{
		Template:  templateSource,
		Version:   version,
		Timestamp: time.Now().UTC(),
		Values:    filteredValues,
	}

	// Marshal to JSON with indentation for readability
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return NewReplayError(templateID, "save", fmt.Errorf("failed to marshal replay data: %w", err))
	}

	// Write to file with secure permissions
	filePath := filepath.Join(replayDir, templateID+".json")

	// Write atomically: write to temp file then rename
	tempPath := filePath + ".tmp"
	if err := os.WriteFile(tempPath, jsonData, types.FileModePrivate); err != nil {
		return NewReplayError(templateID, "save", fmt.Errorf("failed to write replay file: %w", err))
	}

	// Rename temp file to final path (atomic on most systems)
	if err := os.Rename(tempPath, filePath); err != nil {
		// Clean up temp file on error
		_ = os.Remove(tempPath)
		return NewReplayError(templateID, "save", fmt.Errorf("failed to finalize replay file: %w", err))
	}

	return nil
}

// filterSecrets returns a copy of values with secret variables removed.
func filterSecrets(values map[string]any, secrets map[string]bool) map[string]any {
	if len(secrets) == 0 {
		// No secrets to filter, return a copy of values
		result := make(map[string]any, len(values))
		for k, v := range values {
			result[k] = v
		}
		return result
	}

	result := make(map[string]any, len(values))
	for k, v := range values {
		if !secrets[k] {
			result[k] = v
		}
	}
	return result
}

// getReplayDir returns the path to the replay directory.
// Default: ~/.tag/replay
func getReplayDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".tag", DefaultReplayDir), nil
}
