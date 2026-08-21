package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/history"
)

// runTagSubprocess runs the built binary in dir with stdin explicitly
// CLOSED (not merely unset — exec.Cmd with a nil Stdin already reads from
// /dev/null on most platforms, but setting an already-closed pipe end makes
// the "nobody is there to answer a prompt" condition explicit and
// unambiguous rather than relying on that default). It fails the test if the
// process does not exit within ctx's deadline, which is what actually catches
// a hang — a subprocess blocked on stdin does not get killed by a normal
// require.NoError on a completed Wait, it just never completes.
func runTagSubprocess(t *testing.T, ctx context.Context, dir string, args ...string) (stdout, stderr []byte, err error) {
	t.Helper()
	return runTagSubprocessEnv(t, ctx, dir, nil, args...)
}

// runTagSubprocessEnv is runTagSubprocess with environment overrides. Each
// entry in env is either "KEY=VALUE" (set) or bare "KEY" (force-unset). The
// named keys are filtered out of the inherited os.Environ() first and the
// "KEY=VALUE" entries appended after, rather than just appending overrides
// and relying on os/exec's last-wins duplicate handling — that would leave a
// force-unset key's parent-process value still present in the slice, and
// nothing in exec.Cmd's contract guarantees a later duplicate wins.
func runTagSubprocessEnv(t *testing.T, ctx context.Context, dir string, env []string, args ...string) (stdout, stderr []byte, err error) {
	t.Helper()

	closedR, closedW, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)
	require.NoError(t, closedW.Close())
	t.Cleanup(func() { _ = closedR.Close() })

	names := make(map[string]bool, len(env))
	for _, kv := range env {
		key, _, _ := strings.Cut(kv, "=")
		names[key] = true
	}

	cmdEnv := make([]string, 0, len(os.Environ())+len(env))
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if !names[key] {
			cmdEnv = append(cmdEnv, kv)
		}
	}
	for _, kv := range env {
		if _, _, hasValue := strings.Cut(kv, "="); hasValue {
			cmdEnv = append(cmdEnv, kv)
		}
	}

	cmd := exec.CommandContext(ctx, tagBinary, args...)
	cmd.Dir = dir
	cmd.Env = cmdEnv
	cmd.Stdin = closedR
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	return outBuf.Bytes(), errBuf.Bytes(), err
}

// TestIT_GenerateDryRunJSON_TerminatesWithStdinClosed guards against a HANG:
// `tag generate --dry-run --format json` must terminate promptly even with
// stdin closed. This is NOT evidence for D1 (a captured subprocess's stdout
// is a pipe, never a real terminal, so the isTTY check reads false whether or
// not the D1 fix is present — swapping os.Stdout.Fd() for os.Stdin.Fd() in
// diffPromptEnabled would be just as invisible here). The D1 evidence is
// TestUT_DiffPromptEnabled_OnlyWhenSinkIsRealTerminal in internal/engine.
func TestIT_GenerateDryRunJSON_TerminatesWithStdinClosed(t *testing.T) {
	// EvalSymlinks up front: on macOS, t.TempDir() lives under /var, which is
	// itself a symlink to /private/var. The engine's own path-containment
	// check resolves symlinks on both the cached cwd and the target path
	// independently, so handing it an UNresolved directory here (that the
	// subprocess would then getcwd() as the resolved form) makes the two
	// disagree — unrelated to anything this ticket touches, just a real
	// filesystem quirk this fixture must not trip over.
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tagconfig.json"), []byte(
		`{"env":{"TAG_PATH":".tag","TAG_SHARED_PATH":"_shared","TAG_BUNDLE_PATH":"_bundles"}}`,
	), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tag", "_shared"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tag", "hello"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tag", "hello", "hello.txt"), []byte(
		"---\nto: out.txt\n---\nHello {{ name }}\n",
	), 0o600))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stdout, stderr, err := runTagSubprocess(t, ctx, dir,
		"generate", "hello", "world", "--no-hooks", "--dry-run", "--format", "json")
	require.NoError(t, ctx.Err(), "subprocess did not terminate before the deadline (hang)")
	require.NoError(t, err, "stderr: %s", stderr)

	var doc struct {
		DryRun bool `json:"dry_run"`
	}
	require.NoError(t, json.Unmarshal(stdout, &doc))
	require.True(t, doc.DryRun)

	// dry-run must not have written the file.
	_, statErr := os.Stat(filepath.Join(dir, "out.txt"))
	require.True(t, os.IsNotExist(statErr))
}

// TestIT_UndoJSON_TerminatesWithStdinClosed guards against the same class of
// hang for `tag undo --yes --format json`. It is not evidence for D1 or D2 —
// D2 (JSON mode requires --yes) is exercised directly by
// TestUT_UndoJSON_RequiresYes at the command level; this only proves the
// process actually exits.
func TestIT_UndoJSON_TerminatesWithStdinClosed(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, ".tag")
	require.NoError(t, os.MkdirAll(tagDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "handler.go"), []byte("package main\n"), 0o600))

	require.NoError(t, history.Append(tagDir, history.Generation{
		ID:       "gen_1_aaa",
		Template: "model",
		Command:  "generate",
		Files: []history.FileEntry{
			{Path: "handler.go", Action: history.ActionCreate, HashAfter: history.HashBytes([]byte("package main\n"))},
		},
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stdout, stderr, err := runTagSubprocess(t, ctx, dir, "undo", "--yes", "--format", "json")
	require.NoError(t, ctx.Err(), "subprocess did not terminate before the deadline (hang)")
	require.NoError(t, err, "stderr: %s", stderr)

	var doc struct {
		GenID    string `json:"gen_id"`
		Reverted int    `json:"reverted"`
	}
	require.NoError(t, json.Unmarshal(stdout, &doc))
	require.Equal(t, "gen_1_aaa", doc.GenID)
	require.Equal(t, 1, doc.Reverted)
}
