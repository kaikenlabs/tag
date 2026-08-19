package templateupdate

import "strings"

// DiffSummary is the --format json shape for `tag diff`.
//
// Skipped paths from DiffResult are deliberately NOT included here: they are
// an internal ignore-rule detail, not part of the file-level diff a script
// would consume.
type DiffSummary struct {
	OldSHA string            `json:"old_sha"`
	NewSHA string            `json:"new_sha"`
	Source string            `json:"source"`
	Files  []DiffFileSummary `json:"files"`
}

// DiffFileSummary is one entry of DiffSummary.Files.
type DiffFileSummary struct {
	Path       string `json:"path"`
	Op         string `json:"op"`
	Conflicted bool   `json:"conflicted"`
	IsBinary   bool   `json:"is_binary"`
	Added      int    `json:"added"`
	Deleted    int    `json:"deleted"`
}

// Summarize builds the --format json summary for a DiffResult.
//
// added/deleted are DEFINED as the number of +/- lines the TEXT formatter
// (formatFileAdd/formatFileDelete/formatFileUpdate) emits for that file, via
// the same simpleDiffLines primitive the text path prints from — see
// TestUT_Summarize_CountsMatchTextDiffLines, which is what turns "the diff is
// computed once" into an observable property rather than a claim. Accepted
// consequences of that definition:
//   - content ending in a newline counts a trailing empty line, because
//     splitLines yields it and the text path prints a bare "+"/"-" for it;
//   - a conflict is always 0/0 (the text path prints only the warning line,
//     never the conflict-marked content);
//   - a delete with nil BaseContent is 0 (the text path prints no minus
//     lines in that case);
//   - a prompt is always 0/0 (added/deleted are only computed for
//     add/delete/update below).
//
// Binary files are the ONE exception: IsBinary true always reports 0/0, and
// their content is never split or dumped into the summary, regardless of Op.
//
// files includes ops add/delete/update/conflict/prompt and EXCLUDES
// keep/user-added. The text formatter additionally skips prompt entirely
// (formatUnified has no case for MergePrompt) — that is a pre-existing text
// bug filed separately, not something JSON should inherit.
func Summarize(result *DiffResult) DiffSummary {
	files := make([]DiffFileSummary, 0, len(result.Results))

	for _, r := range result.Results {
		if r.Op == MergeKeep || r.Op == MergeUserAdded {
			continue
		}

		var added, deleted int
		if !r.IsBinary {
			switch r.Op {
			case MergeAdd:
				// Deliberately NOT splitLines: formatFileAdd splits
				// r.Content unguarded, so nil content still prints one
				// bare "+" line, whereas splitLines' nil guard would
				// return zero. formatFileDelete below DOES guard on nil,
				// which is why that branch can use splitLines. Matching
				// each branch to its own text formatter is the whole point
				// of the counts-equal-text-lines contract; see
				// TestUT_Summarize_CountsMatchTextDiffLines.
				added = len(strings.Split(string(r.Content), "\n"))
			case MergeDelete:
				if r.BaseContent != nil {
					deleted = len(splitLines(r.BaseContent))
				}
			case MergeUpdate:
				for _, dl := range simpleDiffLines(splitLines(r.OursContent), splitLines(r.Content)) {
					if dl.Sign == '+' {
						added++
					} else {
						deleted++
					}
				}
			default:
				// MergeConflict and MergePrompt are always 0/0 by definition
				// above; MergeKeep/MergeUserAdded never reach this switch,
				// they are filtered out at the top of the loop.
			}
		}

		files = append(files, DiffFileSummary{
			Path:       r.Path,
			Op:         r.Op.String(),
			Conflicted: r.Conflicted,
			IsBinary:   r.IsBinary,
			Added:      added,
			Deleted:    deleted,
		})
	}

	return DiffSummary{
		OldSHA: result.OldSHA,
		NewSHA: result.NewSHA,
		Source: result.Source,
		Files:  files,
	}
}
