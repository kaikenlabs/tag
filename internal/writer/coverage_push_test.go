package writer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// write_logs.go — fileLog WriteFile (line 24-27)
// ===========================================================================

func TestUT_FileLog_WriteFile(t *testing.T) {
	t.Parallel()
	fl := &fileLog{}
	err := fl.WriteFile("test.go", []byte("package main"), 0o644)
	assert.NoError(t, err)
}

// ===========================================================================
// write_logs.go — fileLog ReadFile (line 29-31)
// ===========================================================================

func TestUT_FileLog_ReadFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "test.go")
	require.NoError(t, os.WriteFile(target, []byte("content"), 0o644))

	fl := &fileLog{}
	data, err := fl.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "content", string(data))
}

// ===========================================================================
// write_logs.go — fileLog OpenFile (line 33-36)
// ===========================================================================

func TestUT_FileLog_OpenFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "test.go")
	require.NoError(t, os.WriteFile(target, []byte("data"), 0o644))

	fl := &fileLog{}
	f, err := fl.OpenFile(target, os.O_RDONLY, 0o644)
	require.NoError(t, err)
	require.NoError(t, f.Close())
}

// ===========================================================================
// write_logs.go — fileLog Write (line 38-41)
// ===========================================================================

func TestUT_FileLog_Write(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "test.go")

	f, err := os.Create(target)
	require.NoError(t, err)
	defer f.Close()

	fl := &fileLog{}
	n, err := fl.Write(f, []byte("content"))
	assert.NoError(t, err)
	assert.Equal(t, 7, n)
}

// ===========================================================================
// write_logs.go — fileDiff ReadFile (line 77-79)
// ===========================================================================

func TestUT_FileDiff_ReadFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "test.go")
	require.NoError(t, os.WriteFile(target, []byte("existing"), 0o644))

	fd := &fileDiff{out: os.Stdout}
	data, err := fd.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "existing", string(data))
}

// ===========================================================================
// write_logs.go — fileDiff OpenFile (line 81-84)
// ===========================================================================

func TestUT_FileDiff_OpenFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "test.go")
	require.NoError(t, os.WriteFile(target, []byte("data"), 0o644))

	fd := &fileDiff{out: os.Stdout}
	f, err := fd.OpenFile(target, os.O_RDONLY, 0o644)
	require.NoError(t, err)
	require.NoError(t, f.Close())
}
