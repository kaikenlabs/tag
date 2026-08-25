package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIT_Scaffold_SymlinkedTemplateRoot_ProducesTheSameProject drives the
// shipped binary against a template referenced through a symlinked root,
// pinning #414: filepath.WalkDir does not descend into a symlinked root, so
// before the fix `tag scaffold ./link ...` exited 0 having written nothing.
//
// The second row (a dangling symlink root) is a NO-CHANGE GUARD: main
// already fails there, with template resolution rejecting the reference
// before scaffold.Write is ever reached. Its purpose is to prove the #414
// fix did not move that failure later into a silent empty success.
func TestIT_Scaffold_SymlinkedTemplateRoot_ProducesTheSameProject(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	t.Run("symlinked root scaffolds the template's files", func(t *testing.T) {
		dir, err := filepath.EvalSymlinks(t.TempDir())
		require.NoError(t, err)

		writeScaffoldTemplate(t, dir, "tmpl", false)

		link := filepath.Join(dir, "link")
		require.NoError(t, os.Symlink(filepath.Join(dir, "tmpl"), link))

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		stdout, stderr, runErr := runTagSubprocess(t, ctx, dir,
			"scaffold", "./link", "my-proj", "-m", "project_name=my-proj", "--format", "json")
		require.NoError(t, ctx.Err(), "subprocess did not terminate before the deadline")
		require.NoError(t, runErr, "stderr: %s", stderr)

		dec := json.NewDecoder(bytes.NewReader(stdout))
		var doc scaffoldJSONDoc
		require.NoError(t, dec.Decode(&doc))
		_, tokErr := dec.Token()
		require.ErrorIs(t, tokErr, io.EOF, "stdout carried more than one JSON document")

		var docFiles []string
		for _, f := range doc.Files {
			docFiles = append(docFiles, f.Path)
		}
		assert.Equal(t, []string{"README.md"}, docFiles)

		var gotTree []string
		walkErr := filepath.Walk(doc.ProjectRoot, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.IsDir() {
				return nil
			}
			rel, relErr := filepath.Rel(doc.ProjectRoot, path)
			if relErr != nil {
				return relErr
			}
			gotTree = append(gotTree, filepath.ToSlash(rel))
			return nil
		})
		require.NoError(t, walkErr)
		sort.Strings(gotTree)
		assert.Equal(t, []string{".tag/history.json", ".tagconfig.json", "README.md"}, gotTree)

		readme, readErr := os.ReadFile(filepath.Join(doc.ProjectRoot, "README.md"))
		require.NoError(t, readErr)
		assert.Equal(t, "hello my-proj\n", string(readme))

		assert.NotContains(t, string(stderr), "skipping symlink")
	})

	t.Run("dangling symlink root still fails template resolution (NO-CHANGE GUARD)", func(t *testing.T) {
		dir, err := filepath.EvalSymlinks(t.TempDir())
		require.NoError(t, err)

		dangling := filepath.Join(dir, "dang")
		require.NoError(t, os.Symlink(filepath.Join(dir, "does-not-exist"), dangling))

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		stdout, _, runErr := runTagSubprocess(t, ctx, dir,
			"scaffold", "./dang", "my-proj", "--format", "json")
		require.NoError(t, ctx.Err(), "subprocess did not terminate before the deadline")
		require.Error(t, runErr)

		doc := decodeOneJSONErrorDoc(t, stdout)
		assert.Equal(t, "invalid_reference", doc.Error.Code)

		assert.NoDirExists(t, filepath.Join(dir, "my-proj"))
	})
}
