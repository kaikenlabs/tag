package history

import (
	"time"

	"github.com/kaikenlabs/tag/internal/fileaction"
)

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

// Action is an alias for fileaction.Action, the shared vocabulary for
// describing what TAG did to a file. It is kept as a type alias (rather
// than a new defined type) so existing call sites that reference
// history.Action keep compiling unchanged.
type Action = fileaction.Action

// FileEntry records a single file operation within a generation.
type FileEntry struct {
	Path       string  `json:"path"`
	Action     Action  `json:"action"` // one of the five persisted Action values below
	HashBefore *string `json:"hash_before"`
	HashAfter  string  `json:"hash_after"`
}

// Action constants for FileEntry. These are the five actions that are ever
// persisted to .tag/history.json (fileaction.ActionSkip is report-only and
// deliberately not re-exported here — it never appears on disk). Unknown
// action values written by other TAG versions are preserved verbatim when
// loading a manifest.
const (
	ActionCreate       = fileaction.ActionCreate
	ActionInject       = fileaction.ActionInject
	ActionAppend       = fileaction.ActionAppend
	ActionOverwrite    = fileaction.ActionOverwrite
	ActionOpenAPIMerge = fileaction.ActionOpenAPIMerge
)
