package integration

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/library"
	"github.com/kaikenlabs/tag/internal/remote"
)

// TestIT_Scaffold_DryRun_LeavesLibraryUntouched is the acceptance criterion
// for ticket #432: `tag scaffold <remote-ref> --dry-run` wrote a real entry
// into the shared library on a build of main, in direct contradiction of
// docs/commands/scaffold.md:188 ("a dry run writes nothing to disk"). This
// drives the real binary against the real XDG layout with a pinned remote
// ref and snapshots the full library tree before and after.
//
// Neither subtest passes --add-to-lib, deliberately: for a remote ref
// scaffoldFromRef sets addToLib without any flag, so this is the ticket's
// default path rather than a flagged variant of it. It is the free-slot
// case specifically — prepareLibrarySlot clears addToLib again for
// slotTakenByOther and slotUnavailable, which #429 covers and this does not.
// (The local-template path, where --add-to-lib is what makes addToLib true,
// is covered by TestUT_ScaffoldFromRef_DryRun_DoesNotAddToLibrary.)
//
// This test deliberately does NOT snapshot sandbox.workDir: a remote
// scaffold's verifyTemplateLock writes .tag/lock.json even under --dry-run
// (see scaffold_nolibrary_test.go's B4 subtest, which documents this as a
// known, pre-existing, OUT-OF-SCOPE side effect independent of --no-library).
// That is a real but separate defect being filed as a follow-up ticket;
// widening this test's snapshot to workDir would fail it on a defect this
// change does not fix.
func TestIT_Scaffold_DryRun_LeavesLibraryUntouched(t *testing.T) {
	t.Run("D1 --dry-run leaves the library tree unchanged", func(t *testing.T) {
		sandbox := newNoLibrarySandbox(t)
		templateSrc := noLibraryFixtureTemplate(t)
		seedRemoteCache(t, sandbox.cacheDir, noLibraryFixtureRef, templateSrc)

		libBefore := snapshotTree(t, sandbox.libraryDataDir())

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		outDir := filepath.Join(sandbox.workDir, "project")
		stdout, stderr, err := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
			"scaffold", noLibraryFixtureRef, "generated",
			"--output", outDir, "--no-input", "--dry-run")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		assert.Equal(t, libBefore, snapshotTree(t, sandbox.libraryDataDir()),
			"a dry run must leave the library tree byte-for-byte unchanged")

		// snapshotTree maps both "root missing" and "root present but empty"
		// to an empty map, so tree equality alone cannot see a bare directory
		// being created. Nothing on the dry-run path creates it: newLocalLibrary
		// -> library.NewLocal and lib.Get are pure reads, and only Store.save
		// calls MkdirAll.
		require.NoDirExists(t, sandbox.libraryDataDir())

		assert.Contains(t, string(stdout), "(dry-run) would add template to library as")
	})

	t.Run("D2 positive control: the same run without --dry-run adds the entry", func(t *testing.T) {
		sandbox := newNoLibrarySandbox(t)
		templateSrc := noLibraryFixtureTemplate(t)
		seedRemoteCache(t, sandbox.cacheDir, noLibraryFixtureRef, templateSrc)

		libBefore := snapshotTree(t, sandbox.libraryDataDir())

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		outDir := filepath.Join(sandbox.workDir, "project")
		stdout, stderr, err := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
			"scaffold", noLibraryFixtureRef, "generated",
			"--output", outDir, "--no-input")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		assert.NotEqual(t, libBefore, snapshotTree(t, sandbox.libraryDataDir()),
			"a real run must change the library tree")

		lib := library.NewLocal(sandbox.libraryDataDir())
		_, getErr := lib.Get(remote.LibraryName(noLibraryFixtureRef))
		assert.NoError(t, getErr, "without --dry-run, the template must be added to the library")
	})
}
