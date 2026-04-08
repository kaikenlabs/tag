package writer

import (
	"fmt"
	"log/slog"

	"github.com/kaikenlabs/tag/internal/fileutil"
	"github.com/kaikenlabs/tag/internal/openapi"
	"github.com/kaikenlabs/tag/internal/types"
)

// MergeOpenAPIFile reads the target spec file, merges the fragment into it
// using the openapi.Editor, and writes the result back. The merge is idempotent:
// identical content is skipped, conflicts return an error.
func (w *Write) MergeOpenAPIFile(name string, fragment []byte, opts OpenAPIMergeOptions) (OpenAPIMergeResult, error) {
	w.mx.Lock()
	defer w.mx.Unlock()

	if err := fileutil.ValidatePathContainment(w.cwd, name); err != nil {
		return OpenAPIMergeResult{}, fmt.Errorf("path safety check failed: %w", err)
	}

	specData, err := w.fs.ReadFile(name)
	if err != nil {
		slog.Error("cannot read spec file for merge", "file", name, "error", err)
		return OpenAPIMergeResult{}, fmt.Errorf("cannot read spec file %s: %w", name, err)
	}

	editor := openapi.NewEditor()
	mergeOpts := openapi.MergeOptions{
		ValidateResult: opts.ValidateResult,
		ConflictPolicy: openapi.ConflictError,
	}

	merged, mergeResult, err := editor.Merge(specData, fragment, mergeOpts)
	if err != nil {
		return OpenAPIMergeResult{}, fmt.Errorf("openapi merge failed for %s: %w", name, err)
	}

	result := OpenAPIMergeResult{
		Changed:        mergeResult.Changed,
		AddedPaths:     mergeResult.AddedPaths,
		AddedSchemas:   mergeResult.AddedSchemas,
		SkippedPaths:   mergeResult.SkippedPaths,
		SkippedSchemas: mergeResult.SkippedSchemas,
	}

	if !mergeResult.Changed {
		return result, nil
	}

	if err := w.fs.WriteFile(name, merged, types.FileMode); err != nil {
		slog.Error("cannot write merged spec", "file", name, "error", err)
		return result, fmt.Errorf("cannot write merged spec %s: %w", name, err)
	}

	return result, nil
}
