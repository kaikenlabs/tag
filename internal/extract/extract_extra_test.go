package extract

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_Run_BinaryFile_RejectsWithError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "icon.png")
	binaryContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0x00, 0x00}
	require.NoError(t, os.WriteFile(src, binaryContent, 0o644))

	opts := Options{
		Name:   "test",
		As:     "gen",
		TagDir: filepath.Join(dir, ".tag"),
		Writer: &bytes.Buffer{},
	}

	_, err := Run(opts, src)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "binary")
}

func TestUT_Run_NonexistentFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	opts := Options{
		Name:   "test",
		As:     "gen",
		TagDir: filepath.Join(dir, ".tag"),
		Writer: &bytes.Buffer{},
	}

	_, err := Run(opts, filepath.Join(dir, "nonexistent.go"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot read source file")
}

func TestUT_Run_NoReplacements(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	require.NoError(t, os.WriteFile(src, []byte("package main\nfunc main() {}"), 0o644))

	tagDir := filepath.Join(dir, ".tag")
	opts := Options{
		Name:   "nonexistent",
		As:     "gen",
		TagDir: tagDir,
		Writer: &bytes.Buffer{},
	}

	result, err := Run(opts, src)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Replacements)
	// File should still be written even with 0 replacements
	assert.Contains(t, result.Content, "---\nto:")
}

func TestUT_Run_DryRun_NoWriter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "handler.go")
	require.NoError(t, os.WriteFile(src, []byte("type User struct{}"), 0o644))

	opts := Options{
		Name:   "user",
		As:     "handler",
		DryRun: true,
		TagDir: filepath.Join(dir, ".tag"),
		Writer: nil, // nil writer
	}

	result, err := Run(opts, src)
	require.NoError(t, err)
	assert.Greater(t, result.Replacements, 0)

	// File should NOT be written in dry-run mode
	_, err = os.Stat(result.TemplatePath)
	assert.True(t, os.IsNotExist(err))
}

func TestUT_Run_InteractiveAcceptAll(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "handler.go")
	require.NoError(t, os.WriteFile(src, []byte("User and user and Users"), 0o644))

	mock := &extraMockConfirmer{decisions: []Decision{DecisionAll}}
	opts := Options{
		Name:        "user",
		As:          "handler",
		Interactive: true,
		TagDir:      filepath.Join(dir, ".tag"),
		Writer:      &bytes.Buffer{},
		Prompter:    mock,
	}

	result, err := Run(opts, src)
	require.NoError(t, err)
	assert.Equal(t, 3, result.Replacements, "DecisionAll should accept all remaining")
}

func TestUT_Run_InteractiveQuit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "handler.go")
	require.NoError(t, os.WriteFile(src, []byte("User and user and Users"), 0o644))

	mock := &extraMockConfirmer{decisions: []Decision{DecisionQuit}}
	opts := Options{
		Name:        "user",
		As:          "handler",
		Interactive: true,
		TagDir:      filepath.Join(dir, ".tag"),
		Writer:      &bytes.Buffer{},
		Prompter:    mock,
	}

	result, err := Run(opts, src)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Replacements, "DecisionQuit should accept none")
}

func TestUT_FilterInteractive_ErrorFromPrompter(t *testing.T) {
	t.Parallel()
	occs := []Occurrence{
		{Start: 0, End: 4, Rule: Rule{Needle: "user", Expr: "{{ name }}"}},
	}

	errMock := &extraErrConfirmer{}
	_, err := filterInteractive(occs, errMock)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "interactive prompt failed")
}

func TestUT_BuildToPath_PascalDir(t *testing.T) {
	t.Parallel()
	path := "internal/User/UserService.go"
	rules := BuildRules("user")

	result := BuildToPath(path, rules)
	assert.Contains(t, result, "{{ name | pascal }}")
}

// extraErrConfirmer always returns an error.
type extraErrConfirmer struct{}

func (e *extraErrConfirmer) Confirm(_ Occurrence) (Decision, error) {
	return DecisionNo, assert.AnError
}

// extraMockConfirmer returns pre-configured decisions in order.
type extraMockConfirmer struct {
	decisions []Decision
	idx       int
}

func (m *extraMockConfirmer) Confirm(_ Occurrence) (Decision, error) {
	if m.idx >= len(m.decisions) {
		return DecisionQuit, nil
	}
	d := m.decisions[m.idx]
	m.idx++
	return d, nil
}
