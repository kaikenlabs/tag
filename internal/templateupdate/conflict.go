package templateupdate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kaikenlabs/tag/internal/types"
)

// ConflictReport summarises the outcome of a tree merge for display and
// downstream processing.
type ConflictReport struct {
	// Conflicts lists files that could not be cleanly merged.
	Conflicts []ConflictedFile
	// Clean lists files that were merged or updated without conflicts.
	Clean []MergeResult
	// Prompts lists files that need an explicit user decision.
	Prompts []MergeResult
	// Skipped lists paths excluded by ignore patterns.
	Skipped []string
}

// ConflictedFile provides details about a single conflicted file.
type ConflictedFile struct {
	Path          string
	MarkerCount   int
	BaseContent   []byte
	OursContent   []byte
	TheirsContent []byte
	MergedContent []byte
	Mode          os.FileMode
}

// HasConflicts reports whether any files have unresolved conflicts or prompts.
func (r *ConflictReport) HasConflicts() bool {
	return len(r.Conflicts) > 0 || len(r.Prompts) > 0
}

// NewConflictReport builds a ConflictReport from merge results and skipped paths.
func NewConflictReport(results []MergeResult, skipped []string) *ConflictReport {
	report := &ConflictReport{
		Skipped: skipped,
	}

	for _, mr := range results {
		switch {
		case mr.Op == MergeConflict || mr.Conflicted:
			report.Conflicts = append(report.Conflicts, ConflictedFile{
				Path:          mr.Path,
				MarkerCount:   countConflictMarkers(mr.Content),
				BaseContent:   mr.BaseContent,
				OursContent:   mr.OursContent,
				TheirsContent: mr.TheirsContent,
				MergedContent: mr.Content,
				Mode:          mr.Mode,
			})
		case mr.Op == MergePrompt:
			report.Prompts = append(report.Prompts, mr)
		default:
			report.Clean = append(report.Clean, mr)
		}
	}

	return report
}

// countConflictMarkers counts the number of conflict regions by looking for
// the opening marker pattern.
func countConflictMarkers(content []byte) int {
	return bytes.Count(content, []byte("<<<<<<< "))
}

// ResolveMode specifies how to auto-resolve conflicts.
type ResolveMode int

const (
	// ResolveNone does not auto-resolve; conflict markers remain.
	ResolveNone ResolveMode = iota
	// ResolveOurs replaces conflicted content with the user's version.
	ResolveOurs
	// ResolveTheirs replaces conflicted content with the template's version.
	ResolveTheirs
)

// ResolveConflicts applies the given resolution mode to all conflicts in the
// report and returns a new set of clean MergeResults. Only conflicted files
// are affected; clean and prompt results are unchanged.
func ResolveConflicts(report *ConflictReport, mode ResolveMode) []MergeResult {
	if mode == ResolveNone {
		return nil
	}

	resolved := make([]MergeResult, 0, len(report.Conflicts))
	for _, cf := range report.Conflicts {
		var content []byte
		switch mode {
		case ResolveOurs:
			content = cf.OursContent
		case ResolveTheirs:
			content = cf.TheirsContent
		}

		resolved = append(resolved, MergeResult{
			Path:    cf.Path,
			Op:      MergeUpdate,
			Content: content,
			Mode:    cf.Mode,
		})
	}

	return resolved
}

// ConflictStatus is persisted to .tag/conflicts.json when an update leaves
// unresolved conflicts. It allows resumption with --continue or --abort.
type ConflictStatus struct {
	SchemaVersion   int       `json:"schema_version"`
	UpdateCommit    string    `json:"update_commit"`
	ConflictedFiles []string  `json:"conflicted_files"`
	PromptFiles     []string  `json:"prompt_files,omitempty"`
	ResolvedFiles   []string  `json:"resolved_files"`
	StartedAt       time.Time `json:"started_at"`
}

// conflictStatusVersion is the schema version for conflicts.json.
const conflictStatusVersion = 1

// NewConflictStatus creates a ConflictStatus from a ConflictReport.
func NewConflictStatus(report *ConflictReport, updateCommit string) *ConflictStatus {
	conflicted := make([]string, 0, len(report.Conflicts))
	for _, c := range report.Conflicts {
		conflicted = append(conflicted, c.Path)
	}

	prompts := make([]string, 0, len(report.Prompts))
	for _, p := range report.Prompts {
		prompts = append(prompts, p.Path)
	}

	return &ConflictStatus{
		SchemaVersion:   conflictStatusVersion,
		UpdateCommit:    updateCommit,
		ConflictedFiles: conflicted,
		PromptFiles:     prompts,
		ResolvedFiles:   []string{},
		StartedAt:       time.Now(),
	}
}

// WriteConflictStatus atomically writes the conflict status file to
// <projectRoot>/.tag/conflicts.json.
func WriteConflictStatus(projectRoot string, status *ConflictStatus) error {
	tagDir := filepath.Join(projectRoot, types.TemplatesDir)
	if err := os.MkdirAll(tagDir, 0o755); err != nil {
		return fmt.Errorf("create %s directory: %w", types.TemplatesDir, err)
	}

	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal conflict status: %w", err)
	}
	data = append(data, '\n')

	target := filepath.Join(tagDir, "conflicts.json")
	tmp := target + ".tmp"

	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	if _, err := f.Write(data); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := f.Sync(); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("sync temp file: %w", err)
	}
	f.Close()

	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// ReadConflictStatus reads the conflict status from
// <projectRoot>/.tag/conflicts.json. Returns nil if the file does not exist.
func ReadConflictStatus(projectRoot string) (*ConflictStatus, error) {
	data, err := os.ReadFile(filepath.Join(projectRoot, types.TemplatesDir, "conflicts.json"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil //nolint:nilnil // nil,nil is the documented API for "not found"
		}
		return nil, fmt.Errorf("read conflict status: %w", err)
	}

	var status ConflictStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("parse conflict status: %w", err)
	}

	return &status, nil
}

// ClearConflictStatus removes the conflict status file.
func ClearConflictStatus(projectRoot string) error {
	path := filepath.Join(projectRoot, types.TemplatesDir, "conflicts.json")
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove conflict status: %w", err)
	}
	return nil
}

// FormatConflictSummary writes a human-readable conflict summary to the given
// writer. This is intended for stderr output.
func FormatConflictSummary(w io.Writer, report *ConflictReport) {
	total := len(report.Conflicts) + len(report.Prompts)
	if total == 0 {
		return
	}

	fmt.Fprintf(w, "\n⚠ %d conflict(s) found during template update:\n\n", total)

	for _, c := range report.Conflicts {
		fmt.Fprintf(w, "  CONFLICT  %s (%d region(s))\n", c.Path, c.MarkerCount)
	}
	for _, p := range report.Prompts {
		fmt.Fprintf(w, "  PROMPT    %s (%s)\n", p.Path, p.PromptReason)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Resolve conflicts manually, then run: tag update --continue")
	fmt.Fprintln(w, "Or abort with: tag update --abort")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "To auto-resolve: tag update --accept-ours | --accept-theirs")
}

// FormatCleanSummary writes a success summary to the given writer.
func FormatCleanSummary(w io.Writer, report *ConflictReport) {
	var added, updated, deleted int
	for _, r := range report.Clean {
		switch r.Op {
		case MergeAdd:
			added++
		case MergeUpdate:
			updated++
		case MergeDelete:
			deleted++
		}
	}

	var parts []string
	if added > 0 {
		parts = append(parts, fmt.Sprintf("%d added", added))
	}
	if updated > 0 {
		parts = append(parts, fmt.Sprintf("%d updated", updated))
	}
	if deleted > 0 {
		parts = append(parts, fmt.Sprintf("%d deleted", deleted))
	}
	if len(report.Skipped) > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", len(report.Skipped)))
	}

	if len(parts) == 0 {
		fmt.Fprintln(w, "Template is already up to date.")
		return
	}

	fmt.Fprintf(w, "Template updated successfully: %s.\n", strings.Join(parts, ", "))
}
