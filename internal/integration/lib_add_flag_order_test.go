package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIT_LibAdd_AsFlagInBothOrders is the ticket's headline criterion for
// issue #375: `lib add --as <name> <ref>` and `lib add <ref> --as <name>` must
// install under the same name. Only the compiled binary can prove this —
// urfave/cli's own top-level flag parser is what silently drops a trailing
// --as, and no in-process test harness with a hand-built flag.FlagSet can see
// that class of bug.
func TestIT_LibAdd_AsFlagInBothOrders(t *testing.T) {
	tests := []struct {
		name string
		argv func(srcDir string) []string
	}{
		{"leading", func(srcDir string) []string { return []string{"lib", "add", "--as", "renamed", srcDir} }},
		{"trailing", func(srcDir string) []string { return []string{"lib", "add", srcDir, "--as", "renamed"} }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home, err := filepath.EvalSymlinks(t.TempDir())
			require.NoError(t, err)
			dataHome, err := filepath.EvalSymlinks(t.TempDir())
			require.NoError(t, err)
			t.Setenv("HOME", home)
			t.Setenv("XDG_DATA_HOME", dataHome)

			srcParent, err := filepath.EvalSymlinks(t.TempDir())
			require.NoError(t, err)
			srcDir := filepath.Join(srcParent, "derived-name")
			require.NoError(t, os.MkdirAll(srcDir, 0o750))

			meta, marshalErr := json.Marshal(map[string]any{
				"name":    "derived-name",
				"version": "1.0.0",
			})
			require.NoError(t, marshalErr)
			require.NoError(t, os.WriteFile(filepath.Join(srcDir, "tag.template.json"), meta, 0o600))

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			dir, err := filepath.EvalSymlinks(t.TempDir())
			require.NoError(t, err)

			stdout, stderr, runErr := runTagSubprocess(t, ctx, dir, tt.argv(srcDir)...)
			require.NoError(t, ctx.Err(), "subprocess did not terminate before the deadline (hang)")
			require.NoError(t, runErr, "stdout: %s\nstderr: %s", stdout, stderr)

			renamedDir := filepath.Join(dataHome, "tag", "templates", "renamed")
			derivedDir := filepath.Join(dataHome, "tag", "templates", "derived-name")
			assert.DirExists(t, renamedDir)
			assert.NoDirExists(t, derivedDir)
		})
	}
}

// TestIT_UnknownTrailingFlag_ExitsNonZero pins that a genuinely unrecognised
// trailing flag surfaces as a non-zero process exit code. main.go's
// ExitErrHandler deliberately swallows the exit code inside the same process
// (handleExitError), so only a subprocess can observe the real code
// os.Exit receives.
func TestIT_UnknownTrailingFlag_ExitsNonZero(t *testing.T) {
	home, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	dataHome, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", dataHome)

	srcDir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, stderr, runErr := runTagSubprocess(t, ctx, dir, "lib", "add", srcDir, "--tag-guard-not-a-flag")
	require.NoError(t, ctx.Err(), "subprocess did not terminate before the deadline (hang)")
	require.Error(t, runErr, "stderr: %s", stderr)

	entries, readErr := os.ReadDir(dataHome)
	require.NoError(t, readErr)
	assert.Empty(t, entries, "an unrecognised trailing flag must produce no side effect")
}
