package templateupdate

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUT_FormatDiff_EmptyResults(t *testing.T) {
	var buf bytes.Buffer
	FormatDiff(nil, "gh:acme/tmpl", "abc1234", "def5678", FormatOptions{
		Writer: &buf,
	})
	assert.Contains(t, buf.String(), "abc1234")
	assert.Contains(t, buf.String(), "def5678")
}

func TestUT_FormatDiff_Stat(t *testing.T) {
	results := []MergeResult{
		{Path: "new.txt", Op: MergeAdd, Content: []byte("hello")},
		{Path: "changed.txt", Op: MergeUpdate, Content: []byte("new"), OursContent: []byte("old")},
		{Path: "gone.txt", Op: MergeDelete},
	}

	var buf bytes.Buffer
	FormatDiff(results, "gh:acme/tmpl", "aaa", "bbb", FormatOptions{
		Writer: &buf,
		Stat:   true,
	})

	output := buf.String()
	assert.Contains(t, output, "new.txt")
	assert.Contains(t, output, "changed.txt")
	assert.Contains(t, output, "gone.txt")
	assert.Contains(t, output, "1 file(s) added")
	assert.Contains(t, output, "1 modified")
	assert.Contains(t, output, "1 deleted")
}

func TestUT_FormatDiff_Unified_AddedFile(t *testing.T) {
	results := []MergeResult{
		{Path: "new.txt", Op: MergeAdd, Content: []byte("line1\nline2")},
	}

	var buf bytes.Buffer
	FormatDiff(results, "src", "aaa", "bbb", FormatOptions{
		Writer: &buf,
	})

	output := buf.String()
	assert.Contains(t, output, "+++ b/new.txt")
	assert.Contains(t, output, "+line1")
}

func TestUT_FormatDiff_Unified_DeletedFile(t *testing.T) {
	results := []MergeResult{
		{Path: "old.txt", Op: MergeDelete, BaseContent: []byte("removed")},
	}

	var buf bytes.Buffer
	FormatDiff(results, "src", "aaa", "bbb", FormatOptions{
		Writer: &buf,
	})

	output := buf.String()
	assert.Contains(t, output, "--- a/old.txt")
	assert.Contains(t, output, "-removed")
}

func TestUT_FormatDiff_Conflict(t *testing.T) {
	results := []MergeResult{
		{Path: "conflict.txt", Op: MergeConflict},
	}

	var buf bytes.Buffer
	FormatDiff(results, "src", "aaa", "bbb", FormatOptions{
		Writer: &buf,
	})

	assert.Contains(t, buf.String(), "conflict.txt")
	assert.Contains(t, buf.String(), "conflict")
}

func TestUT_FormatDiff_NoColor(t *testing.T) {
	results := []MergeResult{
		{Path: "new.txt", Op: MergeAdd, Content: []byte("data")},
	}

	var buf bytes.Buffer
	FormatDiff(results, "src", "aaa", "bbb", FormatOptions{
		Writer: &buf,
		Color:  false,
	})

	output := buf.String()
	// Should not contain ANSI escape codes.
	assert.NotContains(t, output, "\033[")
}

func TestUT_FormatDiff_SkipsUnchanged(t *testing.T) {
	results := []MergeResult{
		{Path: "keep.txt", Op: MergeKeep},
		{Path: "user.txt", Op: MergeUserAdded},
	}

	var buf bytes.Buffer
	FormatDiff(results, "src", "aaa", "bbb", FormatOptions{
		Writer: &buf,
		Stat:   true,
	})

	output := buf.String()
	assert.Contains(t, output, "0 file(s) added, 0 modified, 0 deleted")
}

func TestUT_FormatDiff_NilWriter(t *testing.T) {
	// Should not panic.
	FormatDiff(nil, "src", "aaa", "bbb", FormatOptions{Writer: nil})
}
