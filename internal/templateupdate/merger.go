package templateupdate

import (
	"bytes"
	"context"
	"fmt"
	"sort"
)

// MergeEngine reconciles three file trees: base (old template render), ours
// (user's current project), and theirs (new template render).
type MergeEngine struct {
	textMerger TextMerger
	ignoreFn   func(path string) bool
}

// NewMergeEngine creates a MergeEngine with the given text merger and optional
// ignore function. If ignoreFn is nil, no files are skipped.
func NewMergeEngine(tm TextMerger, ignoreFn func(path string) bool) *MergeEngine {
	return &MergeEngine{
		textMerger: tm,
		ignoreFn:   ignoreFn,
	}
}

// MergeTrees merges three file trees and returns a sorted list of merge results
// plus a list of skipped paths.
func (m *MergeEngine) MergeTrees(
	ctx context.Context,
	base, ours, theirs map[string]*RenderedFile,
) ([]MergeResult, []string, error) {
	// Collect all unique paths.
	paths := make(map[string]struct{})
	for p := range base {
		paths[p] = struct{}{}
	}
	for p := range ours {
		paths[p] = struct{}{}
	}
	for p := range theirs {
		paths[p] = struct{}{}
	}

	// Sort for deterministic output.
	sorted := make([]string, 0, len(paths))
	for p := range paths {
		sorted = append(sorted, p)
	}
	sort.Strings(sorted)

	var results []MergeResult
	var skipped []string

	for _, path := range sorted {
		if m.ignoreFn != nil && m.ignoreFn(path) {
			skipped = append(skipped, path)
			continue
		}

		result, err := m.MergeFile(ctx, path, base[path], ours[path], theirs[path])
		if err != nil {
			return nil, nil, fmt.Errorf("merge %s: %w", path, err)
		}
		results = append(results, result)
	}

	return results, skipped, nil
}

// presenceKey encodes which of the three file versions exist as a 3-bit key.
type presenceKey uint8

const (
	pBase   presenceKey = 0b100
	pOurs   presenceKey = 0b010
	pTheirs presenceKey = 0b001
)

// MergeFile merges a single file path across the three trees using the decision
// matrix defined in the design document.
func (m *MergeEngine) MergeFile(
	ctx context.Context, path string, base, ours, theirs *RenderedFile,
) (MergeResult, error) {
	key := presenceKey(0)
	if base != nil {
		key |= pBase
	}
	if ours != nil {
		key |= pOurs
	}
	if theirs != nil {
		key |= pTheirs
	}

	return m.dispatchMerge(ctx, key, path, base, ours, theirs)
}

func (m *MergeEngine) dispatchMerge(
	ctx context.Context, key presenceKey, path string, base, ours, theirs *RenderedFile,
) (MergeResult, error) {
	switch key {
	case 0: // none
		return MergeResult{Path: path, Op: MergeKeep}, nil
	case pTheirs: // new file from template only
		return mergeNewTemplateFile(path, theirs), nil
	case pOurs: // user-added file, not in template
		return MergeResult{Path: path, Op: MergeUserAdded, Mode: ours.Mode}, nil
	case pOurs | pTheirs: // both sides added (no base)
		return m.mergeBothAdded(ctx, path, ours, theirs)
	case pBase: // both deleted
		return MergeResult{Path: path, Op: MergeDelete}, nil
	case pBase | pTheirs: // user deleted, template may have changed
		return mergeUserDeletedTemplateChanged(path, base, theirs), nil
	case pBase | pOurs: // template deleted, user may have modified
		return mergeTemplateDeletedUserHas(path, base, ours), nil
	case pBase | pOurs | pTheirs: // all three exist
		return m.mergeAllThreeExist(ctx, path, base, ours, theirs)
	default:
		return MergeResult{Path: path, Op: MergeKeep}, nil
	}
}

func mergeNewTemplateFile(path string, theirs *RenderedFile) MergeResult {
	return MergeResult{
		Path:    path,
		Op:      MergeAdd,
		Content: theirs.Content,
		Mode:    theirs.Mode,
	}
}

// mergeBothAdded handles the case where base is nil but both ours and theirs exist.
func (m *MergeEngine) mergeBothAdded(
	ctx context.Context, path string, ours, theirs *RenderedFile,
) (MergeResult, error) {
	if bytes.Equal(ours.Content, theirs.Content) {
		return MergeResult{
			Path: path,
			Op:   MergeKeep,
			Mode: ours.Mode,
		}, nil
	}

	if ours.IsBinary || theirs.IsBinary {
		return binaryPrompt(path, nil, ours, theirs,
			"both sides added a binary file with different content"), nil
	}

	return m.textMerge(ctx, path, nil, ours, theirs)
}

// mergeUserDeletedTemplateChanged handles: base exists, user deleted, template changed.
func mergeUserDeletedTemplateChanged(path string, base, theirs *RenderedFile) MergeResult {
	if bytes.Equal(base.Content, theirs.Content) {
		return MergeResult{Path: path, Op: MergeDelete}
	}

	return MergeResult{
		Path:          path,
		Op:            MergePrompt,
		PromptReason:  "you deleted this file but the template updated it",
		BaseContent:   base.Content,
		TheirsContent: theirs.Content,
		Mode:          theirs.Mode,
	}
}

// mergeTemplateDeletedUserHas handles: base exists, template deleted, user still has it.
func mergeTemplateDeletedUserHas(path string, base, ours *RenderedFile) MergeResult {
	if bytes.Equal(base.Content, ours.Content) {
		return MergeResult{Path: path, Op: MergeDelete}
	}

	return MergeResult{
		Path:         path,
		Op:           MergePrompt,
		PromptReason: "the template removed this file but you modified it",
		BaseContent:  base.Content,
		OursContent:  ours.Content,
		Mode:         ours.Mode,
	}
}

// mergeAllThreeExist handles the case where all three versions are present.
func (m *MergeEngine) mergeAllThreeExist(
	ctx context.Context, path string, base, ours, theirs *RenderedFile,
) (MergeResult, error) {
	oursEqBase := bytes.Equal(ours.Content, base.Content)
	theirsEqBase := bytes.Equal(theirs.Content, base.Content)

	switch {
	case theirsEqBase:
		return MergeResult{Path: path, Op: MergeKeep, Mode: ours.Mode}, nil
	case oursEqBase:
		return MergeResult{
			Path: path, Op: MergeUpdate,
			Content: theirs.Content, Mode: ours.Mode,
		}, nil
	case bytes.Equal(ours.Content, theirs.Content):
		return MergeResult{
			Path: path, Op: MergeKeep,
			Mode: ours.Mode,
		}, nil
	default:
		return m.mergeModifiedByBoth(ctx, path, base, ours, theirs)
	}
}

// mergeModifiedByBoth handles the case where both user and template modified
// the file differently from base.
func (m *MergeEngine) mergeModifiedByBoth(
	ctx context.Context, path string, base, ours, theirs *RenderedFile,
) (MergeResult, error) {
	if base.IsBinary || ours.IsBinary || theirs.IsBinary {
		return binaryPrompt(path, base, ours, theirs,
			"binary file modified by both sides"), nil
	}

	return m.textMerge(ctx, path, base, ours, theirs)
}

// textMerge delegates to the TextMerger and classifies the result.
func (m *MergeEngine) textMerge(
	ctx context.Context, path string, base, ours, theirs *RenderedFile,
) (MergeResult, error) {
	var baseContent []byte
	if base != nil {
		baseContent = base.Content
	}

	merged, conflicted, err := m.textMerger.Merge3(ctx, path, baseContent, ours.Content, theirs.Content)
	if err != nil {
		return MergeResult{}, err
	}

	op := MergeUpdate
	if conflicted {
		op = MergeConflict
	}

	return MergeResult{
		Path:          path,
		Op:            op,
		Content:       merged,
		Conflicted:    conflicted,
		Mode:          ours.Mode,
		BaseContent:   baseContent,
		OursContent:   ours.Content,
		TheirsContent: theirs.Content,
	}, nil
}

// binaryPrompt creates a MergePrompt result for binary file conflicts.
func binaryPrompt(
	path string, base, ours, theirs *RenderedFile, reason string,
) MergeResult {
	var baseContent []byte
	if base != nil {
		baseContent = base.Content
	}

	return MergeResult{
		Path:          path,
		Op:            MergePrompt,
		PromptReason:  reason,
		BaseContent:   baseContent,
		OursContent:   ours.Content,
		TheirsContent: theirs.Content,
		Mode:          ours.Mode,
	}
}
