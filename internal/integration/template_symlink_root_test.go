package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/graph"
)

// writeLintFixture writes a minimal template with one declared var
// (project_name) and one undeclared reference (undefined_var) in README.md,
// matching the payload measured against a `main` build for #419: direct
// root reports "undefined variable \"undefined_var\"" at README.md:1, exit
// 1; a symlinked root reported "Template is valid. No issues found.", exit
// 0, before the fix.
func writeLintFixture(t *testing.T, root string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(root, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "tag.template.json"), []byte(
		`{"name":"t","description":"d","vars":{"project_name":"x"}}`,
	), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte(
		"{{ vars.undefined_var }}\n",
	), 0o600))
}

// TestIT_TemplateLint_SymlinkedRoot_MatchesDirectRoot drives the shipped
// binary through `tag template lint`, pinning #419 for the lint command.
// Comparing stdout only: stderr carries a [HH:MM:SS.mmm] timestamp prefix
// and is never byte-comparable across runs.
func TestIT_TemplateLint_SymlinkedRoot_MatchesDirectRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	writeLintFixture(t, filepath.Join(dir, "tmpl"))

	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(filepath.Join(dir, "tmpl"), link))
	resolvedLink, evalErr := filepath.EvalSymlinks(link)
	require.NoError(t, evalErr)
	require.NotEqual(t, link, resolvedLink, "fixture must actually be a symlink")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	directStdout, _, directErr := runTagSubprocess(t, ctx, dir, "template", "lint", "./tmpl")
	require.Error(t, directErr, "positive control: the direct root must report the undefined-variable error (exit 1)")
	require.Contains(t, string(directStdout), `undefined variable "undefined_var"`, "positive control")

	linkStdout, _, linkErr := runTagSubprocess(t, ctx, dir, "template", "lint", "./link")

	assert.Equal(t, directErr != nil, linkErr != nil, "exit success/failure must match")
	assert.Equal(t, string(directStdout), string(linkStdout))
	assert.Contains(t, string(linkStdout), `undefined variable "undefined_var"`)
}

// TestIT_TemplateVariables_SymlinkedRoot_MatchesDirectRoot drives
// `tag template variables`. The fixture declares a REFERENCED var
// (project_name) and an UNREFERENCED one (actually_unused_var), and the
// template also carries one genuinely undeclared reference. This asserts
// the specific lie #419 produced: through the broken symlinked root,
// project_name — which IS referenced — showed up as
// "declared but not referenced in any template", and the real undeclared
// finding vanished entirely.
func TestIT_TemplateVariables_SymlinkedRoot_MatchesDirectRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	root := filepath.Join(dir, "tmpl")
	require.NoError(t, os.MkdirAll(root, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "tag.template.json"), []byte(
		`{"name":"t","description":"d","vars":{"project_name":"x","actually_unused_var":"y"}}`,
	), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte(
		"{{ vars.project_name }}\n{{ vars.undefined_var }}\n",
	), 0o600))

	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(root, link))
	resolvedLink, evalErr := filepath.EvalSymlinks(link)
	require.NoError(t, evalErr)
	require.NotEqual(t, link, resolvedLink, "fixture must actually be a symlink")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	directStdout, directStderr, directErr := runTagSubprocess(t, ctx, dir, "template", "variables", "./tmpl")
	require.NoError(t, directErr, "stderr: %s", directStderr)
	require.Contains(t, string(directStdout), "vars.undefined_var", "positive control: the real undeclared finding")
	require.Contains(t, string(directStdout), "actually_unused_var — declared but not referenced in any template", "positive control: the genuinely unused var")
	require.NotContains(t, string(directStdout), "project_name — declared but not referenced", "positive control: project_name IS referenced")

	linkStdout, linkStderr, linkErr := runTagSubprocess(t, ctx, dir, "template", "variables", "./link")
	require.NoError(t, linkErr, "stderr: %s", linkStderr)

	assert.Equal(t, string(directStdout), string(linkStdout))
	assert.Contains(t, string(linkStdout), "vars.undefined_var")
	assert.NotContains(t, string(linkStdout), "project_name — declared but not referenced",
		"the referenced var must never be misreported as unused")
}

// TestIT_TemplateGraph_SymlinkedRoot_MatchesDirectRoot drives
// `tag template graph --format json`, decoding exactly one JSON document
// (json.Unmarshal accepts trailing content, so this repo's convention is to
// decode once and then require the next token to be io.EOF) and comparing
// the decoded markers, which is the field #419 actually corrupts.
func TestIT_TemplateGraph_SymlinkedRoot_MatchesDirectRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	root := filepath.Join(dir, "tmpl")
	genDir := filepath.Join(root, ".tag", "route")
	require.NoError(t, os.MkdirAll(genDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "route.go"), []byte(
		"---\nto: internal/router.go\ninject: true\nafter: // TAG:routes\n---\nrouter.Handle()\n",
	), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "router.go"), []byte(
		"package internal\n\n// TAG:routes\n",
	), 0o600))

	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(root, link))
	resolvedLink, evalErr := filepath.EvalSymlinks(link)
	require.NoError(t, evalErr)
	require.NotEqual(t, link, resolvedLink, "fixture must actually be a symlink")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	directStdout, directStderr, directErr := runTagSubprocess(t, ctx, dir, "template", "graph", "./tmpl", "--format", "json")
	require.NoError(t, directErr, "stderr: %s", directStderr)

	directReport := decodeOneGraphReport(t, directStdout)
	require.Len(t, directReport.Markers, 1, "positive control: direct root must find the marker")
	assert.Equal(t, "internal/router.go", directReport.Markers[0].File)

	linkStdout, linkStderr, linkErr := runTagSubprocess(t, ctx, dir, "template", "graph", "./link", "--format", "json")
	require.NoError(t, linkErr, "stderr: %s", linkStderr)

	linkReport := decodeOneGraphReport(t, linkStdout)
	assert.Equal(t, directReport.Markers, linkReport.Markers)
}

func decodeOneGraphReport(t *testing.T, stdout []byte) graph.GraphReport {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(stdout))
	var report graph.GraphReport
	require.NoError(t, dec.Decode(&report))
	_, tokErr := dec.Token()
	require.ErrorIs(t, tokErr, io.EOF, "stdout carried more than one JSON document")
	return report
}

// TestIT_TemplateRenameVar_SymlinkedRoot_RewritesRealFiles drives
// `tag template rename-var`, pinning #419: through a symlinked root the
// walk found nothing and printed "No references found ... — nothing to
// rename." at exit 0, leaving zero bytes changed.
func TestIT_TemplateRenameVar_SymlinkedRoot_RewritesRealFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	root := filepath.Join(dir, "tmpl")
	require.NoError(t, os.MkdirAll(root, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "tag.template.json"), []byte(
		`{"name":"t","vars":{"old_name":{"type":"string","prompt":"Old"}}}`,
	), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte(
		"# {{ vars.old_name }}\nHello {{ vars.old_name }}\n",
	), 0o600))

	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(root, link))
	resolvedLink, evalErr := filepath.EvalSymlinks(link)
	require.NoError(t, evalErr)
	require.NotEqual(t, link, resolvedLink, "fixture must actually be a symlink")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stdout, stderr, runErr := runTagSubprocess(t, ctx, dir, "template", "rename-var", "old_name", "new_name", "./link")
	require.NoError(t, runErr, "stderr: %s", stderr)

	require.NotContains(t, string(stdout), "No references found", "the rename must find README.md through the symlinked root")
	require.Contains(t, string(stdout), "2 files, 3 replacements total")

	readme, readErr := os.ReadFile(filepath.Join(root, "README.md"))
	require.NoError(t, readErr)
	assert.Equal(t, "# {{ vars.new_name }}\nHello {{ vars.new_name }}\n", string(readme))

	config, readErr := os.ReadFile(filepath.Join(root, "tag.template.json"))
	require.NoError(t, readErr)
	assert.Contains(t, string(config), `"new_name"`)
	assert.NotContains(t, string(config), `"old_name"`)
}

// TestIT_TemplateLint_SymlinkedCWD_MatchesDirectRoot pins the no-path-
// argument form of #419: `tag template lint` (no positional path) defaults
// root to ".", and I measured that a symlinked CWD reproduces the same bug
// — "Template is valid. No issues found." instead of the real error. The
// trap: setting only cmd.Dir does NOT reproduce it, because the child's own
// getcwd() resolves the symlink away before filepath.Abs(".") ever runs;
// $PWD must be set explicitly in the child's environment, since Go's
// os.Getwd() returns $PWD verbatim once it confirms (via Stat + SameFile)
// that it names the same directory as the real cwd.
func TestIT_TemplateLint_SymlinkedCWD_MatchesDirectRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	writeLintFixture(t, filepath.Join(dir, "tmpl"))

	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(filepath.Join(dir, "tmpl"), link))
	resolvedLink, evalErr := filepath.EvalSymlinks(link)
	require.NoError(t, evalErr)
	require.NotEqual(t, link, resolvedLink, "fixture must actually be a symlink")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	directStdout, _, directErr := runTagSubprocess(t, ctx, filepath.Join(dir, "tmpl"), "template", "lint")
	require.Error(t, directErr, "positive control: the direct cwd must report the undefined-variable error (exit 1)")
	require.Contains(t, string(directStdout), `undefined variable "undefined_var"`, "positive control")

	linkedStdout, _, linkedErr := runTagSubprocessEnv(t, ctx, link, []string{"PWD=" + link}, "template", "lint")

	assert.Equal(t, directErr != nil, linkedErr != nil, "exit success/failure must match")
	assert.Equal(t, string(directStdout), string(linkedStdout))
	assert.Contains(t, string(linkedStdout), `undefined variable "undefined_var"`)
}
