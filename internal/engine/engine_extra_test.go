package engine

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/types"
)

func TestUT_CheckConflicts_NoConflicts(t *testing.T) {
	t.Parallel()

	c := NewCore(TemplateParser{}, &mockFileWriter{}, io.Discard)

	items := []TemplateData{
		{To: "/nonexistent/file.go", ParseData: ParseData{Action: template.ActionCreate}},
	}

	err := c.checkConflicts(items)
	assert.NoError(t, err)
}

func TestUT_CheckConflicts_ExistingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	existing := filepath.Join(dir, "exists.go")
	require.NoError(t, os.WriteFile(existing, []byte("x"), types.FileMode))

	c := NewCore(TemplateParser{}, &mockFileWriter{}, io.Discard)

	items := []TemplateData{
		{To: existing, ParseData: ParseData{Action: template.ActionCreate}},
	}

	err := c.checkConflicts(items)
	require.Error(t, err)

	var ce *ConflictError
	require.ErrorAs(t, err, &ce)
	assert.Contains(t, ce.Files, existing)
}

func TestUT_CheckConflicts_SkipsAppendAndInject(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	existing := filepath.Join(dir, "exists.go")
	require.NoError(t, os.WriteFile(existing, []byte("x"), types.FileMode))

	c := NewCore(TemplateParser{}, &mockFileWriter{}, io.Discard)

	items := []TemplateData{
		{To: existing, ParseData: ParseData{Action: template.ActionAppend}},
		{To: existing, ParseData: ParseData{Action: template.ActionInject}},
	}

	err := c.checkConflicts(items)
	assert.NoError(t, err)
}

func TestUT_CheckConflicts_MultipleConflicts_Sorted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fileB := filepath.Join(dir, "b.go")
	fileA := filepath.Join(dir, "a.go")
	require.NoError(t, os.WriteFile(fileB, []byte("x"), types.FileMode))
	require.NoError(t, os.WriteFile(fileA, []byte("x"), types.FileMode))

	c := NewCore(TemplateParser{}, &mockFileWriter{}, io.Discard)

	items := []TemplateData{
		{To: fileB, ParseData: ParseData{Action: template.ActionCreate}},
		{To: fileA, ParseData: ParseData{Action: template.ActionCreate}},
	}

	err := c.checkConflicts(items)
	require.Error(t, err)

	var ce *ConflictError
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, fileA, ce.Files[0])
	assert.Equal(t, fileB, ce.Files[1])
}

func TestUT_ApplyCreatePolicy_NewFile_Written(t *testing.T) {
	t.Parallel()

	fw := &mockFileWriter{}
	c := NewCore(TemplateParser{}, fw, io.Discard)

	item := TemplateData{
		To:     "/nonexistent/path/file.go",
		Output: []byte("content"),
	}

	var result GenerateResult
	err := c.applyCreatePolicy(item, OnExistingFail, &result)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Created)
	assert.Len(t, fw.writeCalls, 1)
}

func TestUT_ApplyCreatePolicy_ExistingFile_Skip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	existing := filepath.Join(dir, "exists.go")
	require.NoError(t, os.WriteFile(existing, []byte("x"), types.FileMode))

	fw := &mockFileWriter{}
	c := NewCore(TemplateParser{}, fw, io.Discard)

	item := TemplateData{
		To:     existing,
		Output: []byte("new"),
	}

	var result GenerateResult
	err := c.applyCreatePolicy(item, OnExistingSkip, &result)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Skipped)
	assert.Empty(t, fw.writeCalls)
}

func TestUT_ApplyCreatePolicy_ExistingFile_Overwrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	existing := filepath.Join(dir, "exists.go")
	require.NoError(t, os.WriteFile(existing, []byte("x"), types.FileMode))

	fw := &mockFileWriter{}
	c := NewCore(TemplateParser{}, fw, io.Discard)

	item := TemplateData{
		To:     existing,
		Output: []byte("overwritten"),
	}

	var result GenerateResult
	err := c.applyCreatePolicy(item, OnExistingOverwrite, &result)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Overwritten)
	assert.Len(t, fw.writeCalls, 1)
}

func TestUT_ApplyCreatePolicy_ExistingFile_Fail(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	existing := filepath.Join(dir, "exists.go")
	require.NoError(t, os.WriteFile(existing, []byte("x"), types.FileMode))

	fw := &mockFileWriter{}
	c := NewCore(TemplateParser{}, fw, io.Discard)

	item := TemplateData{
		To: existing,
	}

	var result GenerateResult
	err := c.applyCreatePolicy(item, OnExistingFail, &result)
	require.Error(t, err)

	var ce *ConflictError
	require.ErrorAs(t, err, &ce)
}
