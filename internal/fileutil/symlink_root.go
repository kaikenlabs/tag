package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolveSymlinkedRoot resolves root when root itself is a symlink.
// filepath.WalkDir does not descend into a symlinked root: it yields the root
// as a single symlink entry and stops, so a caller walking a user-supplied
// directory sees nothing beneath it. Only a symlinked FINAL component breaks
// the walk; an intermediate one is followed already.
// The symlink test is load-bearing: filepath.EvalSymlinks also Cleans, so
// resolving unconditionally would change the spelling of every non-symlinked
// root in the paths a caller reports back to the user.
// A stat failure leaves root untouched: only whether resolution is required
// is being decided here, and an unreadable root is reported by the caller.
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
