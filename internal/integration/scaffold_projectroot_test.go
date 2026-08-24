package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type scaffoldJSONDoc struct {
	OutputDir   string `json:"output_dir"`
	ProjectRoot string `json:"project_root"`
	Files       []struct {
		Path string `json:"path"`
	} `json:"files"`
}

// writeScaffoldTemplate writes a minimal template under dir/name. When wrapped,
// the single content file sits inside a "{{ vars.project_name }}" directory,
// which is what makes findProjectWrapper treat it as a project wrapper.
func writeScaffoldTemplate(t *testing.T, dir, name string, wrapped bool) string {
	t.Helper()

	root := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(root, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "tag.template.json"), []byte(
		`{"name":"`+name+`","description":"d","vars":{"project_name":"my_project"}}`,
	), 0o600))

	contentDir := root
	if wrapped {
		contentDir = filepath.Join(root, "{{ vars.project_name }}")
		require.NoError(t, os.MkdirAll(contentDir, 0o750))
	}
	require.NoError(t, os.WriteFile(filepath.Join(contentDir, "README.md"), []byte(
		"hello {{ vars.project_name }}\n",
	), 0o600))

	return root
}

// TestIT_ScaffoldJSON_ProjectRootNamesTheGeneratedTree pins the #390 contract
// through the shipped binary: project_root names the directory that actually
// holds the generated files, while output_dir keeps its pre-#390 value.
//
// The wrapper + --output row is the one that motivated the ticket — there the
// two paths genuinely differ, and a consumer handed output_dir would publish
// a directory containing nothing but the project directory.
func TestIT_ScaffoldJSON_ProjectRootNamesTheGeneratedTree(t *testing.T) {
	tests := []struct {
		name          string
		wrapped       bool
		useOutputFlag bool
		wantSeparate  bool
		wantOutputDir string
		wantFirstFile string
	}{
		{
			name:          "plain template with --output",
			wrapped:       false,
			useOutputFlag: true,
			wantSeparate:  false,
			wantOutputDir: "out/proj",
			wantFirstFile: "README.md",
		},
		{
			name:          "wrapper template with --output does not unwrap",
			wrapped:       true,
			useOutputFlag: true,
			wantSeparate:  true,
			wantOutputDir: "out/proj",
			wantFirstFile: "my-proj/README.md",
		},
		{
			name:          "wrapper template without --output unwraps",
			wrapped:       true,
			useOutputFlag: false,
			wantSeparate:  false,
			wantOutputDir: "my-proj",
			wantFirstFile: "README.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, err := filepath.EvalSymlinks(t.TempDir())
			require.NoError(t, err)
			writeScaffoldTemplate(t, dir, "tmpl", tt.wrapped)

			args := []string{"scaffold", "./tmpl", "-m", "project_name=my-proj", "--format", "json"}
			if tt.useOutputFlag {
				args = append(args, "-o", "./out/proj")
			} else {
				args = append(args, "my-proj")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			stdout, stderr, runErr := runTagSubprocess(t, ctx, dir, args...)
			require.NoError(t, ctx.Err(), "subprocess did not terminate before the deadline")
			require.NoError(t, runErr, "stderr: %s", stderr)

			// Exactly one JSON document: json.Unmarshal would accept trailing
			// content, so decode once then demand EOF.
			dec := json.NewDecoder(bytes.NewReader(stdout))
			var doc scaffoldJSONDoc
			require.NoError(t, dec.Decode(&doc))
			_, err = dec.Token()
			require.ErrorIs(t, err, io.EOF, "stdout carried more than one JSON document")

			assert.Equal(t, filepath.Join(dir, tt.wantOutputDir), doc.OutputDir)
			assert.True(t, filepath.IsAbs(doc.ProjectRoot), "project_root must be absolute")

			require.Len(t, doc.Files, 1)
			assert.Equal(t, tt.wantFirstFile, doc.Files[0].Path)

			// files[].path is relative to output_dir in BOTH shapes, so this is
			// where the generated file really is.
			onDisk := filepath.Join(doc.OutputDir, doc.Files[0].Path)
			require.FileExists(t, onDisk)

			// The contract project_root buys: the generated tree lives under it.
			rel, relErr := filepath.Rel(doc.ProjectRoot, onDisk)
			require.NoError(t, relErr)
			assert.NotContains(t, rel, "..", "generated file must live under project_root")

			if tt.wantSeparate {
				assert.NotEqual(t, doc.OutputDir, doc.ProjectRoot)
				// The double-prefix trap: files[].path is NOT relative to project_root.
				assert.NoFileExists(t, filepath.Join(doc.ProjectRoot, doc.Files[0].Path))
			} else {
				assert.Equal(t, doc.OutputDir, doc.ProjectRoot)
			}
		})
	}
}
