package history

import "time"

// Manifest is the root of .tag/history.json.
type Manifest struct {
	Generations []Generation `json:"generations"`
}

// Generation is a single generation record representing one `tag generate`
// or `tag scaffold` invocation.
type Generation struct {
	ID        string      `json:"id"`
	Timestamp time.Time   `json:"timestamp"`
	Template  string      `json:"template,omitempty"`
	Command   string      `json:"command"` // "generate" or "scaffold"
	Files     []FileEntry `json:"files"`
}

// FileEntry records a single file operation within a generation.
type FileEntry struct {
	Path       string  `json:"path"`
	Action     string  `json:"action"` // "create", "inject", "append"
	HashBefore *string `json:"hash_before"`
	HashAfter  string  `json:"hash_after"`
}

// Action constants for FileEntry.
const (
	ActionCreate       = "create"
	ActionInject       = "inject"
	ActionAppend       = "append"
	ActionOverwrite    = "overwrite"
	ActionOpenAPIMerge = "openapi-merge"
)
