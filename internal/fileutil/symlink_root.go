package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolveSymlinkedRoot resolves root when root itself is a symlink.
// filepath.WalkDir does not descend into a symlinked root: it yields the
// root as a symlink entry, which a caller's own anti-exfiltration guard
// then skips, leaving zero files. Only a symlinked FINAL component breaks
// the walk, and filepath.EvalSymlinks also Cleans, so resolving
// unconditionally would change the spelling of every non-symlinked root in
// relPath, outRel and any "skipping symlink" warning text.
// A stat failure leaves root untouched: only whether resolution is required
// is being decided here, and an unreadable root is reported by the walk.
func ResolveSymlinkedRoot(root string) (string, error) {
	if fi, statErr := os.Lstat(root); statErr == nil && fi.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(root)
		if err != nil {
			return "", fmt.Errorf("failed to resolve template root %q: %w", root, err)
		}
		return resolved, nil
	}
	return root, nil
}
