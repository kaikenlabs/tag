package templateupdate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// GitMerger implements TextMerger by shelling out to git merge-file.
type GitMerger struct{}

// Merge3 writes base, ours, and theirs to temporary files and runs
// git merge-file to produce a 3-way merge. Exit code 0 means clean merge,
// exit code >0 with output means conflicts, and any other error is fatal.
func (g *GitMerger) Merge3(ctx context.Context, path string, base, ours, theirs []byte) ([]byte, bool, error) {
	dir, err := os.MkdirTemp("", "tag-merge-*")
	if err != nil {
		return nil, false, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	basePath := filepath.Join(dir, "base")
	oursPath := filepath.Join(dir, "ours")
	theirsPath := filepath.Join(dir, "theirs")

	if err := os.WriteFile(basePath, base, 0o600); err != nil {
		return nil, false, fmt.Errorf("write base: %w", err)
	}
	if err := os.WriteFile(oursPath, ours, 0o600); err != nil {
		return nil, false, fmt.Errorf("write ours: %w", err)
	}
	if err := os.WriteFile(theirsPath, theirs, 0o600); err != nil {
		return nil, false, fmt.Errorf("write theirs: %w", err)
	}

	// git merge-file writes the merged result to the first file (ours) and
	// prints nothing to stdout. Use -p to write to stdout instead.
	// Labels: LOCAL = user's changes, BASE = original template, TEMPLATE = upstream update.
	cmd := exec.CommandContext(ctx, "git", "merge-file", // #nosec G204
		"-p", "--diff3",
		"-L", "LOCAL (your changes)",
		"-L", "BASE (original template)",
		"-L", "TEMPLATE (upstream update)",
		oursPath, basePath, theirsPath,
	)
	cmd.Env = append(os.Environ(), "LC_ALL=C")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	merged := stdout.Bytes()

	if runErr == nil {
		// Exit code 0: clean merge.
		return merged, false, nil
	}

	// git merge-file exits with a positive number equal to the number of
	// conflict regions. Any negative exit code indicates a fatal error.
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		code := exitErr.ExitCode()
		if code > 0 {
			// Conflicts found but merge completed.
			return merged, true, nil
		}
	}

	return nil, false, fmt.Errorf("git merge-file failed for %s: %w (stderr: %s)",
		path, runErr, stderr.String())
}
