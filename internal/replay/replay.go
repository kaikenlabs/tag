// Package replay provides functionality for saving and loading scaffold input values.
// This enables users to re-run scaffolds with previously used values using the --replay flag.
package replay

import (
	"time"
)

// ReplayData represents saved scaffold inputs for replay functionality.
type ReplayData struct {
	// Template is the original template reference (e.g., "gh:user/repo", "./local-template").
	Template string `json:"template"`

	// Version is the template version from tag.template.json's "version" field.
	// This helps users identify which template version was used for the replay.
	// Empty if the template doesn't specify a version.
	Version string `json:"version,omitempty"`

	// Timestamp is when the replay data was saved.
	Timestamp time.Time `json:"timestamp"`

	// Values contains all variable values that were used during scaffolding.
	// Secret variables are excluded from this map.
	Values map[string]any `json:"values"`
}

// DefaultReplayDir is the default directory name within ~/.tag for replay files.
const DefaultReplayDir = "replay"

// EnvReplayDir overrides the replay directory when set to a non-empty
// absolute path. It is consulted before os.UserHomeDir, so it works in
// sandboxes/containers where HOME is unset.
const EnvReplayDir = "TAG_REPLAY_DIR"
