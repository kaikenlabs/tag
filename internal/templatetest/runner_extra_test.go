package templatetest

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_ValidateFixture_MissingName(t *testing.T) {
	t.Parallel()

	f := &Fixture{Mode: ModeScaffold, Template: "tpl"}
	err := validateFixture(f)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestUT_ValidateFixture_InvalidMode(t *testing.T) {
	t.Parallel()

	f := &Fixture{Name: "test", Mode: "invalid", Template: "tpl"}
	err := validateFixture(f)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mode")
}

func TestUT_ValidateFixture_MissingTemplate(t *testing.T) {
	t.Parallel()

	f := &Fixture{Name: "test", Mode: ModeScaffold}
	err := validateFixture(f)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "template")
}

func TestUT_ValidateFixture_GenerateMissingTarget(t *testing.T) {
	t.Parallel()

	f := &Fixture{Name: "test", Mode: ModeGenerate, Template: "tpl"}
	err := validateFixture(f)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target")
}

func TestUT_ValidateFixture_Valid(t *testing.T) {
	t.Parallel()

	f := &Fixture{
		Name:     "test",
		Mode:     ModeScaffold,
		Template: "tpl",
		Assertions: []Assertion{
			{Type: AssertFileExists, Path: "a.go"},
		},
	}
	err := validateFixture(f)
	assert.NoError(t, err)
}

func TestUT_ValidateFixture_ContentContainsMissingValue(t *testing.T) {
	t.Parallel()

	f := &Fixture{
		Name: "test", Mode: ModeScaffold, Template: "tpl",
		Assertions: []Assertion{
			{Type: AssertContentContains, Path: "a.go"},
		},
	}
	err := validateFixture(f)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "value")
}

func TestUT_ValidateFixture_ContentMatchesMissingPattern(t *testing.T) {
	t.Parallel()

	f := &Fixture{
		Name: "test", Mode: ModeScaffold, Template: "tpl",
		Assertions: []Assertion{
			{Type: AssertContentMatches, Path: "a.go"},
		},
	}
	err := validateFixture(f)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pattern")
}

func TestUT_ValidateFixture_UnknownAssertionType(t *testing.T) {
	t.Parallel()

	f := &Fixture{
		Name: "test", Mode: ModeScaffold, Template: "tpl",
		Assertions: []Assertion{
			{Type: "bogus", Path: "a.go"},
		},
	}
	err := validateFixture(f)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown type")
}

func TestUT_ValidateFixture_AssertionMissingPath(t *testing.T) {
	t.Parallel()

	f := &Fixture{
		Name: "test", Mode: ModeScaffold, Template: "tpl",
		Assertions: []Assertion{
			{Type: AssertFileExists},
		},
	}
	err := validateFixture(f)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path")
}

func TestUT_RunAssertion_FileExists_Pass(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "exists.txt"), []byte("hi"), 0o644))

	result := runAssertion(Assertion{Type: AssertFileExists, Path: "exists.txt"}, dir)
	assert.True(t, result.Passed)
}

func TestUT_RunAssertion_FileExists_Fail(t *testing.T) {
	t.Parallel()

	result := runAssertion(Assertion{Type: AssertFileExists, Path: "nope.txt"}, t.TempDir())
	assert.False(t, result.Passed)
	assert.Contains(t, result.Detail, "not found")
}

func TestUT_RunAssertion_FileNotExists_Pass(t *testing.T) {
	t.Parallel()

	result := runAssertion(Assertion{Type: AssertFileNotExists, Path: "nope.txt"}, t.TempDir())
	assert.True(t, result.Passed)
}

func TestUT_RunAssertion_FileNotExists_Fail(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "exists.txt"), []byte("hi"), 0o644))

	result := runAssertion(Assertion{Type: AssertFileNotExists, Path: "exists.txt"}, dir)
	assert.False(t, result.Passed)
}

func TestUT_RunAssertion_ContentContains_Pass(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("hello world"), 0o644))

	result := runAssertion(Assertion{Type: AssertContentContains, Path: "f.txt", Value: "world"}, dir)
	assert.True(t, result.Passed)
}

func TestUT_RunAssertion_ContentContains_Fail(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("hello"), 0o644))

	result := runAssertion(Assertion{Type: AssertContentContains, Path: "f.txt", Value: "world"}, dir)
	assert.False(t, result.Passed)
}

func TestUT_RunAssertion_ContentExcludes_Pass(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("hello"), 0o644))

	result := runAssertion(Assertion{Type: AssertContentExcludes, Path: "f.txt", Value: "world"}, dir)
	assert.True(t, result.Passed)
}

func TestUT_RunAssertion_ContentExcludes_Fail(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("hello world"), 0o644))

	result := runAssertion(Assertion{Type: AssertContentExcludes, Path: "f.txt", Value: "world"}, dir)
	assert.False(t, result.Passed)
}

func TestUT_RunAssertion_ContentMatches_Pass(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("version 1.2.3"), 0o644))

	result := runAssertion(Assertion{Type: AssertContentMatches, Path: "f.txt", Pattern: `\d+\.\d+\.\d+`}, dir)
	assert.True(t, result.Passed)
}

func TestUT_RunAssertion_ContentMatches_Fail(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("no version"), 0o644))

	result := runAssertion(Assertion{Type: AssertContentMatches, Path: "f.txt", Pattern: `\d+\.\d+\.\d+`}, dir)
	assert.False(t, result.Passed)
}

func TestUT_RunAssertion_ContentMatches_InvalidPattern(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0o644))

	result := runAssertion(Assertion{Type: AssertContentMatches, Path: "f.txt", Pattern: `[invalid`}, dir)
	assert.False(t, result.Passed)
	assert.Contains(t, result.Detail, "invalid pattern")
}

func TestUT_RunAssertion_ContentContains_FileMissing(t *testing.T) {
	t.Parallel()

	result := runAssertion(Assertion{Type: AssertContentContains, Path: "nope.txt", Value: "x"}, t.TempDir())
	assert.False(t, result.Passed)
	assert.Contains(t, result.Detail, "read file")
}

func TestUT_RunFixture_SetupFileWriteFails(t *testing.T) {
	t.Parallel()

	// "." resolves the setup-file destination to the fixture's own temp
	// directory: MkdirAll on its parent succeeds, then WriteFile fails with
	// EISDIR.
	result := runFixture(context.Background(), Fixture{
		Name:       "bad-setup",
		SetupFiles: map[string]string{".": "content"},
	}, t.TempDir())

	assert.Equal(t, CaseErrored, result.Status)
	assert.Contains(t, result.Error, "write setup file")
}
