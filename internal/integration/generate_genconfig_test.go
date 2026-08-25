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

// configAsDir marks a fixture whose tag.template.json is a DIRECTORY. That is a
// root-safe stand-in for "unreadable": os.ReadFile returns EISDIR even for a
// process that can bypass permission bits, so unlike chmod 000 it still fails
// when CI runs the suite as root.
const configAsDir = "\x00dir"

// seedGeneratorWithConfig builds a project whose "endpoint" generator ships its
// own tag.template.json, the shape that made #335 abort. The config declares a
// requires gate so the same fixture can prove the file is still read AS CONFIG
// while being skipped as a template.
func seedGeneratorWithConfig(t *testing.T, variablesJSON, configJSON string) string {
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
	configPath := filepath.Join(genDir, "tag.template.json")
	if configJSON == configAsDir {
		require.NoError(t, os.MkdirAll(configPath, 0o750))
	} else {
		require.NoError(t, os.WriteFile(configPath, []byte(configJSON), 0o600))
	}
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

	const goodConfig = `{"requires":["use_http"],"vars":{"port":{"type":"string","default":"8080"}}}`

	tests := []struct {
		name          string
		variablesJSON string
		configJSON    string
		wantErr       bool
		wantFile      bool
		wantStderr    string
	}{
		{
			// Fails on the unfixed binary with "missing required 'to' field".
			name:          "requires met: generates instead of aborting on the config file",
			variablesJSON: `{"use_http":true}`,
			configJSON:    goodConfig,
			wantErr:       false,
			wantFile:      true,
		},
		{
			// Passes on the unfixed binary too: checkRequirements runs before the
			// engine is built. It is here so the pair proves the file is skipped as
			// a template WITHOUT being ignored as config — neither row alone does.
			name:          "requires unmet: config is still honoured and blocks the run",
			variablesJSON: `{}`,
			configJSON:    goodConfig,
			wantErr:       true,
			wantFile:      false,
			wantStderr:    "requires the following variables to be enabled",
		},
		{
			// Skipping the file as a template must not let a config TAG cannot
			// understand silently disable the gate that config declares.
			name:          "malformed config is fatal, not a silently bypassed gate",
			variablesJSON: `{}`,
			configJSON:    `{"requires":["use_http"`,
			wantErr:       true,
			wantFile:      false,
			wantStderr:    "cannot parse tag.template.json",
		},
		{
			name:          "unreadable config is fatal, not a silently bypassed gate",
			variablesJSON: `{}`,
			configJSON:    configAsDir,
			wantErr:       true,
			wantFile:      false,
			wantStderr:    "cannot read tag.template.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := seedGeneratorWithConfig(t, tt.variablesJSON, tt.configJSON)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			_, stderr, err := runTagSubprocess(t, ctx, dir, "generate", "endpoint", "widget")

			assert.NotContains(t, string(stderr), "missing required 'to' field")
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, string(stderr), tt.wantStderr)
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

// TestIT_GenerateBundle_GeneratorRequiresStillGates pins the second half of the
// gate. Before #335 a bundled generator that shipped a tag.template.json could
// not run at all — the loader aborted on the config file — so the bundle path's
// missing generator-level requires check was invisible. Making the generator
// runnable makes that hole reachable, so the bundle must apply the same gate a
// direct `tag generate` applies.
func TestIT_GenerateBundle_GeneratorRequiresStillGates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		variablesJSON string
		wantErr       bool
	}{
		{name: "generator requires unmet: bundle is blocked", variablesJSON: `{}`, wantErr: true},
		{name: "generator requires met: bundle runs", variablesJSON: `{"use_http":true}`, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := seedGeneratorWithConfig(t, tt.variablesJSON, `{"requires":["use_http"]}`)
			bundleDir := filepath.Join(dir, ".tag", "_bundles", "full")
			require.NoError(t, os.MkdirAll(bundleDir, 0o750))
			require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "full.json"), []byte(
				`{"name":"full","description":"d","generators":[{"name":"endpoint"}]}`,
			), 0o600))

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			_, stderr, err := runTagSubprocess(t, ctx, dir, "generate", "full", "widget")

			if tt.wantErr {
				require.Error(t, err, "bundle ran a generator whose own requires is unmet")
				assert.Contains(t, string(stderr), "requires the following variables to be enabled")
				assert.Contains(t, string(stderr), "use_http")
				assert.NoFileExists(t, filepath.Join(dir, "widget.go"))
			} else {
				require.NoError(t, err, "stderr: %s", stderr)
				assert.FileExists(t, filepath.Join(dir, "widget.go"))
			}
		})
	}
}
