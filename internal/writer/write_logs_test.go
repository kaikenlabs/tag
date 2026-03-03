package writer

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- fileDiff.WriteFile tests ----

func TestUT_FileDiff_WriteFile_NewFile_NonTTY(t *testing.T) {
	var out bytes.Buffer
	fd := &fileDiff{out: &out, in: strings.NewReader(""), isTTY: false}

	err := fd.WriteFile("nonexistent-file-xyz.go", []byte("package main\n\nfunc main() {}\n"), 0o644)
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "(new file)")
	assert.Contains(t, output, "nonexistent-file-xyz.go")
	assert.Contains(t, output, "+package main")
}

func TestUT_FileDiff_WriteFile_ModifiedFile_NonTTY(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	require.NoError(t, os.WriteFile(path, []byte("package main\n"), 0o644))

	var out bytes.Buffer
	fd := &fileDiff{out: &out, in: strings.NewReader(""), isTTY: false}

	err := fd.WriteFile(path, []byte("package main\n\nfunc main() {}\n"), 0o644)
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "--- a/")
	assert.Contains(t, output, "+++ b/")
	assert.Contains(t, output, "+func main() {}")
}

func TestUT_FileDiff_WriteFile_NoChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	content := []byte("package main\n")
	require.NoError(t, os.WriteFile(path, content, 0o644))

	var out bytes.Buffer
	fd := &fileDiff{out: &out, in: strings.NewReader(""), isTTY: false}

	err := fd.WriteFile(path, content, 0o644)
	require.NoError(t, err)

	assert.Contains(t, out.String(), "no changes")
}

func TestUT_FileDiff_WriteFile_TTY_AcceptYes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	require.NoError(t, os.WriteFile(path, []byte("package main\n"), 0o644))

	var out bytes.Buffer
	fd := &fileDiff{out: &out, in: strings.NewReader("y\n"), isTTY: true}

	err := fd.WriteFile(path, []byte("package main\n\nfunc main() {}\n"), 0o644)
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "[y]es/[n]o/[a]ll/[q]uit")
}

func TestUT_FileDiff_WriteFile_TTY_AcceptAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	require.NoError(t, os.WriteFile(path, []byte("package main\n"), 0o644))

	var out bytes.Buffer
	fd := &fileDiff{out: &out, in: strings.NewReader("a\n"), isTTY: true}

	err := fd.WriteFile(path, []byte("package main\n\nfunc main() {}\n"), 0o644)
	require.NoError(t, err)

	// After accepting all, subsequent writes skip the prompt.
	assert.True(t, fd.acceptAll)
}

func TestUT_FileDiff_WriteFile_TTY_Quit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	require.NoError(t, os.WriteFile(path, []byte("package main\n"), 0o644))

	var out bytes.Buffer
	fd := &fileDiff{out: &out, in: strings.NewReader("q\n"), isTTY: true}

	err := fd.WriteFile(path, []byte("package main\n\nfunc main() {}\n"), 0o644)
	assert.ErrorIs(t, err, ErrUserQuit)
}

// ---- fileDiff.Write (append) tests ----

func TestUT_FileDiff_Write_ShowsAppendDiff_NonTTY(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.go")
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	var out bytes.Buffer
	fd := &fileDiff{out: &out, in: strings.NewReader(""), isTTY: false}

	n, err := fd.Write(f, []byte("// new content\n"))
	require.NoError(t, err)
	assert.Equal(t, 15, n)
	assert.Contains(t, out.String(), "(append)")
	assert.Contains(t, out.String(), "+// new content")
}

// ---- NewFileWriter integration ----

func TestUT_NewFileWriter_DryRun_WithDiffOutput(t *testing.T) {
	var out bytes.Buffer
	fw, err := NewFileWriter(true, WithDiffOutput(&out, strings.NewReader(""), false))
	require.NoError(t, err)

	w, ok := fw.(*Write)
	require.True(t, ok)

	_, isDiff := w.fs.(*fileDiff)
	assert.True(t, isDiff, "expected fileDiff writer when WithDiffOutput is supplied")
}

func TestUT_NewFileWriter_DryRun_WithoutDiffOutput_UsesFileLog(t *testing.T) {
	fw, err := NewFileWriter(true)
	require.NoError(t, err)

	w, ok := fw.(*Write)
	require.True(t, ok)

	_, isLog := w.fs.(*fileLog)
	assert.True(t, isLog, "expected fileLog writer when no WithDiffOutput is supplied")
}

func TestUT_NewFileWriter_NotDryRun_UsesFileWrite(t *testing.T) {
	fw, err := NewFileWriter(false)
	require.NoError(t, err)

	w, ok := fw.(*Write)
	require.True(t, ok)

	_, isWrite := w.fs.(*fileWrite)
	assert.True(t, isWrite, "expected fileWrite writer in non-dry-run mode")
}
