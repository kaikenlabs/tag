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
)

func TestIT_SkipInTestHook_RunsInScaffoldNotInTest(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	tmpl := filepath.Join(dir, "tmpl")
	require.NoError(t, os.MkdirAll(tmpl, 0o750))
	config := `{
  "name": "skip-in-test",
  "description": "d",
  "vars": {
    "project_name": "proj",
    "use_x": {"type": "boolean", "default": false}
  },
  "hooks": {
    "post_scaffold": [
      "sh -c 'echo first >> order.log'",
      {"run": "sh -c 'echo skipped >> order.log'", "skip_in_test": true},
      {"run": "sh -c 'echo last >> order.log'"}
    ]
  }
}`
	require.NoError(t, os.WriteFile(filepath.Join(tmpl, "tag.template.json"), []byte(config), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpl, "README.md"), []byte("hello\n"), 0o600))
	check := "[ \"$(cat order.log)\" = \"$(printf 'first\\nlast')\" ]\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmpl, "check.sh"), []byte(check), 0o600))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	t.Run("scaffold runs every hook in order", func(t *testing.T) {
		out := filepath.Join(dir, "out")
		stdout, stderr, runErr := runTagSubprocess(t, ctx, dir,
			"scaffold", "./tmpl", "-o", out, "--accept-hooks", "--format", "json")
		require.NoError(t, runErr, "stderr: %s", stderr)

		var doc scaffoldJSONDoc
		require.NoError(t, json.Unmarshal(stdout, &doc))
		got, err := os.ReadFile(filepath.Join(doc.ProjectRoot, "order.log"))
		require.NoError(t, err)
		assert.Equal(t, "first\nskipped\nlast\n", string(got))
	})

	t.Run("test drops the marked hook and keeps the rest", func(t *testing.T) {
		stdout, stderr, runErr := runTagSubprocess(t, ctx, dir,
			"test", "./tmpl", "--accept-hooks", "--run", "sh check.sh", "--format", "json")
		require.NoError(t, runErr, "stdout: %s stderr: %s", stdout, stderr)

		dec := json.NewDecoder(bytes.NewReader(stdout))
		var report struct {
			Passed     int `json:"passed"`
			Failed     int `json:"failed"`
			Errored    int `json:"errored"`
			TotalCases int `json:"total_cases"`
			Cases      []struct {
				Status string `json:"status"`
			} `json:"cases"`
		}
		require.NoError(t, dec.Decode(&report))
		_, err := dec.Token()
		require.ErrorIs(t, err, io.EOF, "stdout carried more than one JSON document")

		assert.Equal(t, 2, report.Passed)
		assert.Equal(t, 0, report.Failed)
		assert.Equal(t, 0, report.Errored)
		assert.Equal(t, 2, report.TotalCases)
		require.Len(t, report.Cases, 2)
		for _, c := range report.Cases {
			assert.Equal(t, "passed", c.Status)
		}
	})
}
