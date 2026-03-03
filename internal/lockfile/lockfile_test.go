package lockfile_test

import (
	"errors"
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
