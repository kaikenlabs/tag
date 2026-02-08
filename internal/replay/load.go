package replay

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Load loads replay data for the given template source.
// Returns ErrReplayNotFound if no replay file exists.
// Returns ErrReplayCorrupt if the replay file exists but cannot be parsed.
func Load(templateSource string) (*ReplayData, error) {
	templateSource = strings.TrimSpace(templateSource)
	if templateSource == "" {
		return nil, ErrEmptyTemplateSource
	}

	templateID := GenerateTemplateID(templateSource)
	if templateID == "" {
		return nil, ErrEmptyTemplateSource
	}

	// Get replay file path
	filePath, err := getReplayFilePath(templateID)
	if err != nil {
		return nil, NewReplayError(templateID, "load", err)
	}

	// Read the file
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrReplayNotFound
		}
		if os.IsPermission(err) {
			return nil, NewReplayError(templateID, "load", fmt.Errorf("permission denied: %w", err))
		}
		return nil, NewReplayError(templateID, "load", fmt.Errorf("failed to read file: %w", err))
	}

	// Parse the JSON
	var replay ReplayData
	if err := json.Unmarshal(data, &replay); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrReplayCorrupt, err)
	}

	// Validate basic structure
	if replay.Values == nil {
		replay.Values = make(map[string]any)
	}

	return &replay, nil
}

// Exists checks if a replay file exists for the given template source.
func Exists(templateSource string) bool {
	templateSource = strings.TrimSpace(templateSource)
	if templateSource == "" {
		return false
	}

	templateID := GenerateTemplateID(templateSource)
	if templateID == "" {
		return false
	}

	filePath, err := getReplayFilePath(templateID)
	if err != nil {
		return false
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return false
	}

	return !info.IsDir()
}

// GetReplayFilePath returns the full path to the replay file for the given template source.
// This is useful for displaying to users or for cleanup operations.
func GetReplayFilePath(templateSource string) (string, error) {
	templateSource = strings.TrimSpace(templateSource)
	if templateSource == "" {
		return "", ErrEmptyTemplateSource
	}

	templateID := GenerateTemplateID(templateSource)
	if templateID == "" {
		return "", ErrEmptyTemplateSource
	}

	return getReplayFilePath(templateID)
}

// getReplayFilePath returns the path to the replay file for a template ID.
func getReplayFilePath(templateID string) (string, error) {
	replayDir, err := getReplayDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(replayDir, templateID+".json"), nil
}

// Delete removes the replay file for the given template source.
// Returns nil if the file doesn't exist.
func Delete(templateSource string) error {
	templateSource = strings.TrimSpace(templateSource)
	if templateSource == "" {
		return ErrEmptyTemplateSource
	}

	templateID := GenerateTemplateID(templateSource)
	if templateID == "" {
		return ErrEmptyTemplateSource
	}

	filePath, err := getReplayFilePath(templateID)
	if err != nil {
		return NewReplayError(templateID, "delete", err)
	}

	err = os.Remove(filePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return NewReplayError(templateID, "delete", err)
	}

	return nil
}
