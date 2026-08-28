package lockfile_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/lockfile"
)

// ---- Hash tests ----

func TestUT_HashTemplateDirDeterministic(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("world"), 0o644))

	h1, err := lockfile.HashTemplateDir(dir)
	require.NoError(t, err)
	h2, err := lockfile.HashTemplateDir(dir)
	require.NoError(t, err)

	assert.Equal(t, h1, h2, "same directory hashed twice should produce identical result")
}

func TestUT_HashTemplateDirOrder(t *testing.T) {
	// Create two dirs with the same files in different creation order.
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	files := map[string]string{
		"z_last.go":   "package z",
		"a_first.go":  "package a",
		"m_middle.go": "package m",
	}
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir1, name), []byte(content), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir2, name), []byte(content), 0o644))
	}

	h1, err := lockfile.HashTemplateDir(dir1)
	require.NoError(t, err)
	h2, err := lockfile.HashTemplateDir(dir2)
	require.NoError(t, err)

	assert.Equal(t, h1, h2, "hash should be stable regardless of file creation order")
}

func TestUT_HashTemplateDirChangesOnContentChange(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "template.txt")
	require.NoError(t, os.WriteFile(file, []byte("v1"), 0o644))

	h1, err := lockfile.HashTemplateDir(dir)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(file, []byte("v2"), 0o644))

	h2, err := lockfile.HashTemplateDir(dir)
	require.NoError(t, err)

	assert.NotEqual(t, h1, h2, "hash should change when file content changes")
}

func TestUT_HashTemplateDirSkipsLockFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.go"), []byte("package x"), 0o644))

	h1, err := lockfile.HashTemplateDir(dir)
	require.NoError(t, err)

	// Adding a lock.json should not change the hash.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "lock.json"), []byte(`{"version":1}`), 0o644))

	h2, err := lockfile.HashTemplateDir(dir)
	require.NoError(t, err)

	assert.Equal(t, h1, h2, "lock.json should be excluded from the hash")
}

func TestUT_HashTemplateDirSkipsDSStore(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.go"), []byte("package x"), 0o644))
	h1, err := lockfile.HashTemplateDir(dir)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte("noise"), 0o644))
	h2, err := lockfile.HashTemplateDir(dir)
	require.NoError(t, err)

	assert.Equal(t, h1, h2, ".DS_Store should be ignored in the hash")
}

// ---- Load/Save/Verify tests ----

func TestUT_LockFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, ".tag")
	require.NoError(t, os.MkdirAll(tagDir, 0o750))

	lf := &lockfile.File{
		Version: 1,
		Templates: map[string]*lockfile.Entry{
			"gh:org/repo": {
				Ref:        "gh:org/repo",
				SHA256:     "deadbeef",
				ResolvedAt: time.Now().UTC().Truncate(time.Second),
			},
		},
	}

	require.NoError(t, lockfile.Save(dir, lf))

	loaded, err := lockfile.Load(dir)
	require.NoError(t, err)
	require.Contains(t, loaded.Templates, "gh:org/repo")
	assert.Equal(t, "deadbeef", loaded.Templates["gh:org/repo"].SHA256)
}

func TestUT_LockFileLoadMissing(t *testing.T) {
	dir := t.TempDir()
	lf, err := lockfile.Load(dir)
	require.NoError(t, err)
	assert.NotNil(t, lf)
	assert.Empty(t, lf.Templates)
}

func TestUT_LockFileLoadCorrupted(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tag"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tag", "lock.json"), []byte("not json"), 0o640))

	_, err := lockfile.Load(dir)
	assert.ErrorContains(t, err, "parse lockfile")
}

func TestUT_LockFileVerifyMatch(t *testing.T) {
	dir := t.TempDir()
	tmplDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "file.go"), []byte("package x"), 0o644))

	// First call creates the entry.
	err := lockfile.VerifyAndMaybeUpdate(dir, "gh:org/repo", tmplDir, lockfile.VerifyOptions{})
	require.NoError(t, err)

	// Second call should verify and pass (content unchanged).
	err = lockfile.VerifyAndMaybeUpdate(dir, "gh:org/repo", tmplDir, lockfile.VerifyOptions{})
	require.NoError(t, err)
}

func TestUT_LockFileMismatch(t *testing.T) {
	dir := t.TempDir()
	tmplDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "file.go"), []byte("v1"), 0o644))

	// Create initial entry.
	require.NoError(t, lockfile.VerifyAndMaybeUpdate(dir, "gh:org/repo", tmplDir, lockfile.VerifyOptions{}))

	// Change template content.
	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "file.go"), []byte("v2 — tampered"), 0o644))

	// Verification should fail with ErrChecksumMismatch.
	err := lockfile.VerifyAndMaybeUpdate(dir, "gh:org/repo", tmplDir, lockfile.VerifyOptions{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, lockfile.ErrChecksumMismatch),
		"expected ErrChecksumMismatch, got: %v", err)
	assert.Contains(t, err.Error(), "--update-lock")
}

func TestUT_UpdateLockFlag(t *testing.T) {
	dir := t.TempDir()
	tmplDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "file.go"), []byte("v1"), 0o644))
	require.NoError(t, lockfile.VerifyAndMaybeUpdate(dir, "gh:org/repo", tmplDir, lockfile.VerifyOptions{}))

	// Change content then update-lock.
	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "file.go"), []byte("v2"), 0o644))
	require.NoError(t, lockfile.VerifyAndMaybeUpdate(dir, "gh:org/repo", tmplDir, lockfile.VerifyOptions{UpdateLock: true}))

	// Subsequent verify with new content should pass.
	require.NoError(t, lockfile.VerifyAndMaybeUpdate(dir, "gh:org/repo", tmplDir, lockfile.VerifyOptions{}))
}

func TestUT_IgnoreLockFlag(t *testing.T) {
	dir := t.TempDir()
	tmplDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "file.go"), []byte("v1"), 0o644))
	require.NoError(t, lockfile.VerifyAndMaybeUpdate(dir, "gh:org/repo", tmplDir, lockfile.VerifyOptions{}))

	// Change content but use --ignore-lock; should not error.
	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "file.go"), []byte("tampered"), 0o644))
	err := lockfile.VerifyAndMaybeUpdate(dir, "gh:org/repo", tmplDir, lockfile.VerifyOptions{IgnoreLock: true})
	require.NoError(t, err)
}

func TestUT_LockFileNewEntry(t *testing.T) {
	dir := t.TempDir()
	tmplDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "main.go"), []byte("package main"), 0o644))

	require.NoError(t, lockfile.VerifyAndMaybeUpdate(dir, "gh:org/mytemplate", tmplDir, lockfile.VerifyOptions{}))

	lf, err := lockfile.Load(dir)
	require.NoError(t, err)
	entry, ok := lf.Templates["gh:org/mytemplate"]
	require.True(t, ok, "entry should be created")
	assert.NotEmpty(t, entry.SHA256)
	assert.WithinDuration(t, time.Now(), entry.ResolvedAt, 5*time.Second)
}

func TestUT_LockFilePathNormalization(t *testing.T) {
	// Verify that subdirectory files produce stable hashes.
	dir := t.TempDir()
	subdir := filepath.Join(dir, "sub")
	require.NoError(t, os.MkdirAll(subdir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(subdir, "file.go"), []byte("pkg sub"), 0o644))

	h1, err := lockfile.HashTemplateDir(dir)
	require.NoError(t, err)

	h2, err := lockfile.HashTemplateDir(dir)
	require.NoError(t, err)

	assert.Equal(t, h1, h2)
	assert.True(t, strings.HasPrefix(h1, ""), "hash should be non-empty hex string")
	assert.Len(t, h1, 64, "SHA-256 hex is 64 chars")
}

// ---- #442 --dry-run tests ----

func TestUT_VerifyAndMaybeUpdate_DryRunSkipsSave(t *testing.T) {
	for _, tc := range []struct {
		name       string
		dryRun     bool
		wantTagDir bool
	}{
		{name: "dry run creates no .tag dir", dryRun: true, wantTagDir: false},
		{name: "real run creates .tag dir", dryRun: false, wantTagDir: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			tmplDir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "file.go"), []byte("package x"), 0o644))

			err := lockfile.VerifyAndMaybeUpdate(dir, "gh:org/repo", tmplDir, lockfile.VerifyOptions{DryRun: tc.dryRun})
			require.NoError(t, err)

			tagDir := filepath.Join(dir, ".tag")
			if tc.wantTagDir {
				assert.DirExists(t, tagDir)
			} else {
				assert.NoDirExists(t, tagDir)
			}
		})
	}
}

// TestUT_VerifyAndMaybeUpdate_DryRunStillDetectsChecksumMismatch is a
// NO-CHANGE GUARD: it passes on both sides of the #442 fix. It exists to
// catch an over-broad fix that short-circuits the whole function with
// `if opts.DryRun { return nil }` at the top, which would also silently
// swallow a real checksum mismatch under --dry-run.
func TestUT_VerifyAndMaybeUpdate_DryRunStillDetectsChecksumMismatch(t *testing.T) {
	dir := t.TempDir()
	tmplDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "file.go"), []byte("v1"), 0o644))
	require.NoError(t, lockfile.VerifyAndMaybeUpdate(dir, "gh:org/repo", tmplDir, lockfile.VerifyOptions{}))

	lockPath := filepath.Join(dir, ".tag", "lock.json")
	before, err := os.ReadFile(lockPath)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "file.go"), []byte("v2 — tampered"), 0o644))

	err = lockfile.VerifyAndMaybeUpdate(dir, "gh:org/repo", tmplDir, lockfile.VerifyOptions{DryRun: true})
	require.Error(t, err)
	assert.True(t, errors.Is(err, lockfile.ErrChecksumMismatch),
		"a dry run must still detect a checksum mismatch, got: %v", err)

	after, err := os.ReadFile(lockPath)
	require.NoError(t, err)
	assert.Equal(t, before, after, "the lockfile bytes must be unchanged after a dry-run mismatch check")
}

func TestUT_VerifyAndMaybeUpdate_DryRunUpdateLockDoesNotRewrite(t *testing.T) {
	for _, tc := range []struct {
		name        string
		dryRun      bool
		wantRewrite bool
	}{
		{name: "dry run does not rewrite", dryRun: true, wantRewrite: false},
		{name: "real run rewrites", dryRun: false, wantRewrite: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			tmplDir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "file.go"), []byte("v1"), 0o644))

			seed := &lockfile.File{
				Version: 1,
				Templates: map[string]*lockfile.Entry{
					"gh:org/repo": {
						Ref:        "gh:org/repo",
						Version:    "sentinel-v0",
						SHA256:     "not-the-real-hash-so-this-is-a-mismatch",
						ResolvedAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
					},
				},
			}
			require.NoError(t, lockfile.Save(dir, seed))

			lockPath := filepath.Join(dir, ".tag", "lock.json")
			before, err := os.ReadFile(lockPath)
			require.NoError(t, err)

			err = lockfile.VerifyAndMaybeUpdate(dir, "gh:org/repo", tmplDir,
				lockfile.VerifyOptions{UpdateLock: true, DryRun: tc.dryRun})
			require.NoError(t, err)

			after, err := os.ReadFile(lockPath)
			require.NoError(t, err)

			if tc.wantRewrite {
				assert.NotEqual(t, before, after, "a real --update-lock run must rewrite the entry")
			} else {
				assert.Equal(t, before, after, "a dry --update-lock run must leave the bytes byte-identical")
			}
		})
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	origStderr := os.Stderr
	os.Stderr = w

	fn()

	os.Stderr = origStderr
	require.NoError(t, w.Close())
	captured, readErr := io.ReadAll(r)
	require.NoError(t, readErr)
	return string(captured)
}

// The notice is emitted only where a write was actually skipped. The silent
// rows are the reason this is a table rather than a single positive
// assertion: an announcement on an arm that would not have written anything
// tells the user a real run does something it does not.
func TestUT_VerifyAndMaybeUpdate_DryRunNoticeOnlyWhereAWriteWasSkipped(t *testing.T) {
	const ref = "gh:org/repo"

	for _, tc := range []struct {
		name       string
		seedEntry  bool
		opts       lockfile.VerifyOptions
		wantNotice bool
	}{
		{name: "no entry: the skipped create is announced", seedEntry: false, opts: lockfile.VerifyOptions{DryRun: true}, wantNotice: true},
		{name: "matching entry: nothing was skipped, so nothing is said", seedEntry: true, opts: lockfile.VerifyOptions{DryRun: true}, wantNotice: false},
		{name: "matching entry with --update-lock: the skipped refresh is announced", seedEntry: true, opts: lockfile.VerifyOptions{DryRun: true, UpdateLock: true}, wantNotice: true},
		{name: "--ignore-lock returns before the dry-run branch is reached", seedEntry: false, opts: lockfile.VerifyOptions{DryRun: true, IgnoreLock: true}, wantNotice: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			tmplDir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "file.go"), []byte("package x"), 0o644))
			if tc.seedEntry {
				require.NoError(t, lockfile.VerifyAndMaybeUpdate(dir, ref, tmplDir, lockfile.VerifyOptions{}))
			}

			var verifyErr error
			captured := captureStderr(t, func() {
				verifyErr = lockfile.VerifyAndMaybeUpdate(dir, ref, tmplDir, tc.opts)
			})
			require.NoError(t, verifyErr)

			notice := "(dry-run) would pin " + ref + " in .tag/lock.json"
			if tc.wantNotice {
				assert.Contains(t, captured, notice)
			} else {
				assert.NotContains(t, captured, "would pin")
			}
		})
	}
}
