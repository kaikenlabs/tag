package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/lockfile"
)

// TestIT_Scaffold_DryRun_LeavesWorkDirUntouched is the acceptance criterion
// for ticket #442: `tag scaffold <remote-ref> --dry-run` used to write a real
// entry into <cwd>/.tag/lock.json (see scaffold_dryrun_library_test.go's
// header, which documented this as a known, pre-existing, out-of-scope
// defect). This drives the real binary against a pinned remote ref with
// --output kept INSIDE workDir specifically so one snapshot also covers
// output-directory creation.
func TestIT_Scaffold_DryRun_LeavesWorkDirUntouched(t *testing.T) {
	t.Run("L1 --dry-run creates nothing under the working directory", func(t *testing.T) {
		sandbox := newNoLibrarySandbox(t)
		templateSrc := noLibraryFixtureTemplate(t)
		seedRemoteCache(t, sandbox.cacheDir, noLibraryFixtureRef, templateSrc)

		// Do not narrow this snapshot root to a subdirectory later: the whole
		// point of covering workDir (rather than only the library dir, which
		// scaffold_dryrun_library_test.go already does) is to catch the
		// lockfile write AND the output-directory creation in one pass.
		before := snapshotTree(t, sandbox.workDir)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		outDir := filepath.Join(sandbox.workDir, "project")
		stdout, stderr, err := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
			"scaffold", noLibraryFixtureRef, "generated", "--output", outDir, "--no-input", "--dry-run")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		// snapshotTree maps both "root missing" and "root present but empty"
		// to an empty map, so tree equality alone cannot see a bare directory
		// being created — the explicit NoDirExists below is what catches that.
		assert.Equal(t, before, snapshotTree(t, sandbox.workDir),
			"a dry run must leave workDir byte-for-byte unchanged")
		require.NoDirExists(t, filepath.Join(sandbox.workDir, ".tag"))
	})

	t.Run("L2 positive control: the real run writes the lockfile entry", func(t *testing.T) {
		sandbox := newNoLibrarySandbox(t)
		templateSrc := noLibraryFixtureTemplate(t)
		seedRemoteCache(t, sandbox.cacheDir, noLibraryFixtureRef, templateSrc)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		outDir := filepath.Join(sandbox.workDir, "project")
		stdout, stderr, err := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
			"scaffold", noLibraryFixtureRef, "generated", "--output", outDir, "--no-input")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		assert.Equal(t, []string{"lock.json"}, dirEntryNames(t, filepath.Join(sandbox.workDir, ".tag")))

		lf, loadErr := lockfile.Load(sandbox.workDir)
		require.NoError(t, loadErr)
		entry, ok := lf.Templates[noLibraryFixtureRef]
		require.True(t, ok, "lock.json must carry an entry keyed on the ref")
		assert.NotEmpty(t, entry.SHA256)
	})

	t.Run("L3 --dry-run still fails loudly on a checksum mismatch", func(t *testing.T) {
		// NO-CHANGE GUARD: this subtest passes on both sides of the #442 fix.
		// It exists to catch an over-broad fix that silences verification
		// itself under --dry-run, not merely the write.
		sandbox := newNoLibrarySandbox(t)
		templateSrc := noLibraryFixtureTemplate(t)
		seedRemoteCache(t, sandbox.cacheDir, noLibraryFixtureRef, templateSrc)

		seeded := &lockfile.File{
			Version: 1,
			Templates: map[string]*lockfile.Entry{
				noLibraryFixtureRef: {
					Ref:        noLibraryFixtureRef,
					SHA256:     "0000000000000000000000000000000000000000000000000000000000000",
					ResolvedAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
				},
			},
		}
		require.NoError(t, lockfile.Save(sandbox.workDir, seeded))
		lockPath := filepath.Join(sandbox.workDir, ".tag", "lock.json")
		before, readErr := os.ReadFile(lockPath)
		require.NoError(t, readErr)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		outDir := filepath.Join(sandbox.workDir, "project")
		_, stderr, err := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
			"scaffold", noLibraryFixtureRef, "generated", "--output", outDir, "--no-input", "--dry-run")
		require.Error(t, err, "a checksum mismatch must fail the run even under --dry-run")
		assert.Contains(t, string(stderr), "template checksum mismatch")
		assert.Contains(t, string(stderr), "--update-lock")

		after, readErr := os.ReadFile(lockPath)
		require.NoError(t, readErr)
		assert.Equal(t, before, after, "the seeded lock.json bytes must be unchanged")
	})

	t.Run("L4 --update-lock --dry-run leaves the existing entry byte-identical", func(t *testing.T) {
		sandbox := newNoLibrarySandbox(t)
		templateSrc := noLibraryFixtureTemplate(t)
		seedRemoteCache(t, sandbox.cacheDir, noLibraryFixtureRef, templateSrc)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		outDir := filepath.Join(sandbox.workDir, "project")
		stdout, stderr, err := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
			"scaffold", noLibraryFixtureRef, "generated", "--output", outDir, "--no-input")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		// TRAP: do not derive the expected hash with lockfile.HashTemplateDir
		// against templateSrc. HashTemplateDir does not skip _meta.json, and
		// FSCache.Set writes _meta.json into the resolved cache entry root —
		// so the source-tree hash is NOT the resolved-dir hash the real run
		// above just pinned. Seeding from that source hash would silently
		// land the run below in the mismatch branch instead of the refresh
		// branch, satisfying the byte-identical assertion for the wrong
		// reason. Read back what production actually wrote instead.
		lf, loadErr := lockfile.Load(sandbox.workDir)
		require.NoError(t, loadErr)
		entry, ok := lf.Templates[noLibraryFixtureRef]
		require.True(t, ok)

		entry.Version = "sentinel-v0"
		entry.ResolvedAt = time.Date(2019, 6, 15, 12, 0, 0, 0, time.UTC)
		require.NoError(t, lockfile.Save(sandbox.workDir, lf))

		lockPath := filepath.Join(sandbox.workDir, ".tag", "lock.json")
		before, readErr := os.ReadFile(lockPath)
		require.NoError(t, readErr)

		stdout, stderr, err = runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
			"scaffold", noLibraryFixtureRef, "generated", "--output", outDir, "--no-input", "--update-lock", "--dry-run", "--force")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		// Without this the subtest false-greens if scaffoldFromRef ever stops
		// forwarding --update-lock: the matching-checksum no-op arm also
		// leaves the bytes and both sentinels untouched. The notice is only
		// emitted from the refresh arm, so it is what proves that arm ran.
		assert.Contains(t, string(stderr), "would pin",
			"the refresh arm must have been reached, not the matching-checksum no-op arm")

		after, readErr := os.ReadFile(lockPath)
		require.NoError(t, readErr)
		assert.Equal(t, before, after, "--update-lock --dry-run must not rewrite the lockfile")

		lfAfter, loadErr := lockfile.Load(sandbox.workDir)
		require.NoError(t, loadErr)
		entryAfter, ok := lfAfter.Templates[noLibraryFixtureRef]
		require.True(t, ok)
		assert.Equal(t, "sentinel-v0", entryAfter.Version, "the sentinel Version must survive an --update-lock --dry-run")
		assert.True(t, entryAfter.ResolvedAt.Equal(time.Date(2019, 6, 15, 12, 0, 0, 0, time.UTC)),
			"the sentinel ResolvedAt must survive an --update-lock --dry-run")
	})

	t.Run("L5 the skipped-write notice does not pollute the JSON document", func(t *testing.T) {
		sandbox := newNoLibrarySandbox(t)
		templateSrc := noLibraryFixtureTemplate(t)
		seedRemoteCache(t, sandbox.cacheDir, noLibraryFixtureRef, templateSrc)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		outDir := filepath.Join(sandbox.workDir, "project")
		stdout, stderr, err := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
			"scaffold", noLibraryFixtureRef, "generated", "--output", outDir, "--no-input", "--dry-run", "--format", "json")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		assert.Contains(t, string(stderr), "would pin")

		dec := json.NewDecoder(bytes.NewReader(stdout))
		var doc map[string]any
		require.NoError(t, dec.Decode(&doc), "stdout must decode as JSON: %s", stdout)
		require.ErrorIs(t, dec.Decode(&doc), io.EOF, "stdout must contain exactly one JSON document")
	})
}
