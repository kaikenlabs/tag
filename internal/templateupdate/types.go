package templateupdate

import (
	"context"
	"os"
)

// MergeOp classifies the action to take for a file during a 3-way merge.
type MergeOp int

const (
	// MergeKeep means the file needs no changes (user's version is current).
	MergeKeep MergeOp = iota
	// MergeAdd means a new file from the template should be added.
	MergeAdd
	// MergeDelete means the file was removed by both sides.
	MergeDelete
	// MergeUpdate means a clean merge was applied automatically.
	MergeUpdate
	// MergeConflict means overlapping changes require user resolution.
	MergeConflict
	// MergeUserAdded means the file exists only in the user's project.
	MergeUserAdded
	// MergePrompt means the situation needs an explicit user decision
	// (e.g. user deleted but template changed, or binary conflict).
	MergePrompt
)

// String returns a human-readable label for a MergeOp.
func (op MergeOp) String() string {
	switch op {
	case MergeKeep:
		return "keep"
	case MergeAdd:
		return "add"
	case MergeDelete:
		return "delete"
	case MergeUpdate:
		return "update"
	case MergeConflict:
		return "conflict"
	case MergeUserAdded:
		return "user-added"
	case MergePrompt:
		return "prompt"
	default:
		return "unknown"
	}
}

// MergeResult describes the outcome of merging a single file path.
type MergeResult struct {
	// Path is the relative file path within the project.
	Path string
	// Op is the merge operation classification.
	Op MergeOp
	// Content holds the merged content. For conflicts it contains conflict markers.
	Content []byte
	// Mode is the file permission mode for the merged file.
	Mode os.FileMode
	// Conflicted is true when Content contains unresolved conflict markers.
	Conflicted bool
	// PromptReason describes why a user prompt is needed (non-empty only for MergePrompt).
	PromptReason string
	// BaseContent is the original base version (populated for conflicts/prompts to enable resolution).
	BaseContent []byte
	// OursContent is the user's version (populated for conflicts/prompts).
	OursContent []byte
	// TheirsContent is the template's version (populated for conflicts/prompts).
	TheirsContent []byte
}

// TextMerger performs 3-way text merging of file contents.
type TextMerger interface {
	// Merge3 performs a 3-way merge of base, ours, and theirs content.
	// Returns the merged content, whether conflicts were found, and any error.
	Merge3(ctx context.Context, path string, base, ours, theirs []byte) (merged []byte, conflicted bool, err error)
}
