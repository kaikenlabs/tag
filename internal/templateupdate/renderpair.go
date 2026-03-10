package templateupdate

import (
	"context"
	"fmt"
)

// RenderPair renders the template at two commits (old and new) and returns
// pointer maps suitable for MergeTrees. This provides a clean API for future
// single-clone optimization while currently calling RenderAt twice.
func (r *HistoricalRenderer) RenderPair(
	ctx context.Context,
	templateURL string,
	oldSHA string,
	newSHA string,
	vars map[string]any,
) (base, theirs map[string]*RenderedFile, err error) {
	baseValues, err := r.RenderAt(ctx, templateURL, oldSHA, vars)
	if err != nil {
		return nil, nil, fmt.Errorf("render base (commit %s): %w", shortSHA(oldSHA), err)
	}

	theirsValues, err := r.RenderAt(ctx, templateURL, newSHA, vars)
	if err != nil {
		return nil, nil, fmt.Errorf("render theirs (commit %s): %w", shortSHA(newSHA), err)
	}

	return ToPointerMap(baseValues), ToPointerMap(theirsValues), nil
}

// shortSHA returns the first 7 characters of a SHA for display.
func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
