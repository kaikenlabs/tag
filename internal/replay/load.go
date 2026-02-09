package replay

import (
	"encoding/json"
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
		return nil, fmt.Errorf("%w: %w", ErrReplayCorrupt, err)
	}

	// Validate basic structure
	if replay.Values == nil {
		replay.Values = make(map[string]any)
	}

	return &replay, nil
}

// getReplayFilePath returns the path to the replay file for a template ID.
func getReplayFilePath(templateID string) (string, error) {
	replayDir, err := getReplayDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(replayDir, templateID+".json"), nil
}
