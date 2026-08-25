package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedGeneratorWithConfig builds a project whose "endpoint" generator ships its
// own tag.template.json, the shape that made #335 abort. The config declares a
// requires gate so the same fixture can prove the file is still read AS CONFIG
// while being skipped as a template.
func seedGeneratorWithConfig(t *testing.T, variablesJSON string) string {
	t.Helper()

	// See TestIT_GenerateDryRunJSON_TerminatesWithStdinClosed for why this
	// resolves symlinks up front.
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tagconfig.json"), []byte(
		`{"env":{"TAG_PATH":".tag","TAG_SHARED_PATH":"_shared","TAG_BUNDLE_PATH":"_bundles"},`+
			`"variables":`+variablesJSON+`}`,
	), 0o600))

	genDir := filepath.Join(dir, ".tag", "endpoint")
	require.NoError(t, os.MkdirAll(genDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "tag.template.json"), []byte(
		`{"requires":["use_http"],"vars":{"port":{"type":"string","default":"8080"}}}`,
	), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "handler.txt"), []byte(
		"---\nto: {{ name }}.go\n---\npackage {{ name }}\n",
	), 0o600))

	return dir
}

// TestIT_GenerateGeneratorWithConfig drives the real binary, which is the only
// way to reach NewGeneratorWithRecorder (the constructor the CLI actually uses)
// and the only place the two halves of #335 can be asserted together: the
// engine must skip tag.template.json as a template while internal/commands
// still reads it as config.
func TestIT_GenerateGeneratorWithConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		variablesJSON string
		wantErr       bool
		wantFile      bool
	}{
		{
			// Fails on the unfixed binary with "missing required 'to' field".
			name:          "requires met: generates instead of aborting on the config file",
			variablesJSON: `{"use_http":true}`,
			wantErr:       false,
			wantFile:      true,
		},
		{
			// Passes on the unfixed binary too: checkRequirements runs before the
			// engine is built. It is here so the pair proves the file is skipped as
			// a template WITHOUT being ignored as config — neither row alone does.
			name:          "requires unmet: config is still honoured and blocks the run",
			variablesJSON: `{}`,
			wantErr:       true,
			wantFile:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := seedGeneratorWithConfig(t, tt.variablesJSON)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			_, stderr, err := runTagSubprocess(t, ctx, dir, "generate", "endpoint", "widget")

			assert.NotContains(t, string(stderr), "missing required 'to' field")
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, string(stderr), "requires the following variables to be enabled")
				assert.Contains(t, string(stderr), "use_http")
			} else {
				require.NoError(t, err, "stderr: %s", stderr)
			}

			if tt.wantFile {
				content, readErr := os.ReadFile(filepath.Join(dir, "widget.go"))
				require.NoError(t, readErr)
				assert.Equal(t, "package widget\n", string(content))
			} else {
				assert.NoFileExists(t, filepath.Join(dir, "widget.go"))
			}

			// The config file must never be rendered into the project as output.
			assert.NoFileExists(t, filepath.Join(dir, "tag.template.json"))
		})
	}
}

// TestIT_ShippedExamples_ScaffoldThenGenerate pins the onboarding path named in
// #335: both examples ship _generators/endpoint/tag.template.json, so both were
// broken out of the box. Hooks are skipped (--no-input without --accept-hooks),
// so this stays offline and deterministic. The oracle is deliberately loose —
// file existence plus a substring — so editing an example does not redden CI.
func TestIT_ShippedExamples_ScaffoldThenGenerate(t *testing.T) {
	t.Parallel()

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)

	tests := []struct {
		name       string
		example    string
		wantFile   string
		injectedIn string
	}{
		{name: "go", example: "weather-api-go", wantFile: "forecast.go", injectedIn: "main.go"},
		{name: "python", example: "weather-api-python", wantFile: "forecast.py", injectedIn: "app.py"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			base, absErr := filepath.EvalSymlinks(t.TempDir())
			require.NoError(t, absErr)
			// --output must not exist yet; t.TempDir() does.
			out := filepath.Join(base, "scaffolded")

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			_, stderr, scaffoldErr := runTagSubprocess(t, ctx, repoRoot,
				"scaffold", filepath.Join(repoRoot, "examples", tt.example), "-o", out, "--no-input")
			require.NoError(t, scaffoldErr, "stderr: %s", stderr)

			proj := filepath.Join(out, "weather-api")
			// The config genuinely lands in the generated project — this is WHY the bug shipped.
			require.FileExists(t, filepath.Join(proj, ".tag", "endpoint", "tag.template.json"))

			_, stderr, genErr := runTagSubprocess(t, ctx, proj, "generate", "endpoint", "forecast")
			assert.NotContains(t, string(stderr), "missing required 'to' field")
			require.NoError(t, genErr, "stderr: %s", stderr)

			assert.FileExists(t, filepath.Join(proj, tt.wantFile))
			injected, readErr := os.ReadFile(filepath.Join(proj, tt.injectedIn))
			require.NoError(t, readErr)
			assert.True(t, strings.Contains(string(injected), "forecast"),
				"expected %s to reference the generated endpoint", tt.injectedIn)
		})
	}
}
