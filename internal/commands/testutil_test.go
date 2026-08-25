package commands

import (
	"encoding/json"
	"flag"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/engine"
	"github.com/kaikenlabs/tag/internal/library"
	"github.com/kaikenlabs/tag/internal/types/flags"
)

// mockGenerator is a test double for engine.Generator.
type mockGenerator struct {
	GenerateFunc  func(data engine.Data) (engine.GenerateResult, error)
	GenerateCalls []engine.Data
}

func (m *mockGenerator) Generate(data engine.Data) (engine.GenerateResult, error) {
	m.GenerateCalls = append(m.GenerateCalls, data)
	if m.GenerateFunc != nil {
		return m.GenerateFunc(data)
	}
	return engine.GenerateResult{}, nil
}

// setupTempDir creates a temporary directory and returns its path.
// The directory is automatically cleaned up when the test completes.
func setupTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "tag-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	return dir
}

// createTestConfig creates a minimal valid config for testing.
func createTestConfig(t *testing.T, basePath string) *config.Config {
	t.Helper()
	return &config.Config{
		Env: config.Env{
			Path:       basePath,
			SharedPath: "_shared",
			BundlePath: "_bundles",
		},
		Hooks: config.Hooks{
			Pre:  [][]string{},
			Post: [][]string{},
		},
	}
}

// createTestConfigWithLib creates a config that references a library template.
func createTestConfigWithLib(t *testing.T, basePath, libName string) *config.Config {
	t.Helper()
	cfg := createTestConfig(t, basePath)
	cfg.Template = &config.TemplateOrigin{
		Name:   libName,
		Source: "gh:test/" + libName,
	}
	return cfg
}

// createTestCLIContext creates a cli.Context for testing with the given args and flags.
func createTestCLIContext(t *testing.T, args []string, flagValues map[string]any) *cli.Context {
	t.Helper()

	app := &cli.App{
		Writer: io.Discard,
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: flags.DryRunFlag},
			&cli.StringFlag{Name: flags.PathFlag, Value: ".tag"},
			&cli.StringFlag{Name: flags.SharedPathFlag, Value: "_shared"},
			&cli.StringFlag{Name: flags.BundlePathFlag, Value: "_bundles"},
			&cli.StringSliceFlag{Name: flags.MetaFlag},
			&cli.StringFlag{Name: "package", Value: "mypackage"},
			&cli.BoolFlag{Name: flags.LibFlag},
			&cli.BoolFlag{Name: flags.SelfContainedFlag},
			&cli.StringFlag{Name: flags.InBundleFlag},
			&cli.StringFlag{Name: flags.OnExistingFlag, Value: ""},
			&cli.BoolFlag{Name: flags.VerboseFlag},
			&cli.BoolFlag{Name: flags.NoHooksFlag},
		},
	}

	set := flag.NewFlagSet("test", flag.ContinueOnError)

	// Register flags
	for _, f := range app.Flags {
		if err := f.Apply(set); err != nil {
			t.Fatalf("failed to apply flag: %v", err)
		}
	}

	// Set flag values
	for name, value := range flagValues {
		switch v := value.(type) {
		case bool:
			if err := set.Set(name, boolToString(v)); err != nil {
				t.Fatalf("failed to set flag %s: %v", name, err)
			}
		case string:
			if err := set.Set(name, v); err != nil {
				t.Fatalf("failed to set flag %s: %v", name, err)
			}
		case []string:
			for _, s := range v {
				if err := set.Set(name, s); err != nil {
					t.Fatalf("failed to set flag %s: %v", name, err)
				}
			}
		}
	}

	// Parse args
	if err := set.Parse(args); err != nil {
		t.Fatalf("failed to parse args: %v", err)
	}

	return cli.NewContext(app, set, nil)
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// createGenerator creates a generator directory with a template file.
func createGenerator(t *testing.T, basePath, generatorName, templateContent string) {
	t.Helper()
	genDir := filepath.Join(basePath, generatorName)
	if err := os.MkdirAll(genDir, 0o750); err != nil {
		t.Fatalf("failed to create generator dir: %v", err)
	}

	templatePath := filepath.Join(genDir, generatorName+".go")
	if err := os.WriteFile(templatePath, []byte(templateContent), 0o644); err != nil {
		t.Fatalf("failed to write template file: %v", err)
	}
}

// createBundle creates a bundle directory with a JSON bundle file.
func createBundle(t *testing.T, basePath, bundleName, jsonContent string) {
	t.Helper()
	bundleDir := filepath.Join(basePath, "_bundles", bundleName)
	if err := os.MkdirAll(bundleDir, 0o750); err != nil {
		t.Fatalf("failed to create bundle dir: %v", err)
	}

	bundlePath := filepath.Join(bundleDir, bundleName+".json")
	if err := os.WriteFile(bundlePath, []byte(jsonContent), 0o644); err != nil {
		t.Fatalf("failed to write bundle file: %v", err)
	}
}

// createSharedDir creates the shared templates directory.
func createSharedDir(t *testing.T, basePath string) {
	t.Helper()
	sharedDir := filepath.Join(basePath, "_shared")
	if err := os.MkdirAll(sharedDir, 0o750); err != nil {
		t.Fatalf("failed to create shared dir: %v", err)
	}
}

// setupFakeLibrary creates a fake library data directory with a registered template
// and substitutes newLocalLibrary to use it. Returns the template directory path.
// Restores the original newLocalLibrary when the test completes.
//
// WARNING: This mutates the package-level newLocalLibrary variable.
// Tests using this helper must NOT call t.Parallel().
func setupFakeLibrary(t *testing.T, templateName string) string {
	t.Helper()

	dataDir := isolateLibrary(t)

	// Create template directory on disk
	templateDir := filepath.Join(dataDir, "templates", templateName)
	if err := os.MkdirAll(templateDir, 0o750); err != nil {
		t.Fatalf("failed to create template dir: %v", err)
	}

	// Write a minimal registry with the template entry
	reg := library.Registry{
		Version: 1,
		Entries: map[string]*library.Entry{
			templateName: {
				Name:      templateName,
				Source:    "gh:test/" + templateName,
				AddedAt:   time.Now(),
				UpdatedAt: time.Now(),
			},
		},
	}
	regData, err := json.Marshal(reg)
	if err != nil {
		t.Fatalf("failed to marshal registry: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "library.json"), regData, 0o644); err != nil {
		t.Fatalf("failed to write registry: %v", err)
	}

	return templateDir
}

// isolateLibrary points newLocalLibrary at a fresh, empty library dir and
// returns it.
//
// HOME is isolated as well as the library dir: a scaffold driven through
// scaffoldAction ends in replay.Save, whose getReplayDir resolves
// os.UserHomeDir() directly (internal/replay/save.go) — with the real HOME
// every such test dropped a file into the developer's ~/.tag/replay, which is
// how hundreds accumulated there.
//
// Callers must not use t.Parallel: this substitutes package-level state.
func isolateLibrary(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	dataDir := t.TempDir()
	orig := newLocalLibrary
	newLocalLibrary = func() (*library.Library, error) {
		return library.NewLocal(dataDir), nil
	}
	t.Cleanup(func() { newLocalLibrary = orig })
	return dataDir
}

// listTreeEntries returns a sorted, root-relative listing of every entry
// (files AND directories) under root. It observes entry TYPE and PATH only:
// it detects additions, removals and file/dir kind flips, but NOT a same-path
// file whose bytes changed. Tests that need to prove content survived must
// compare the bytes themselves.
func listTreeEntries(t *testing.T, root string) []string {
	t.Helper()
	var entries []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		kind := "file"
		if d.IsDir() {
			kind = "dir"
		}
		entries = append(entries, kind+":"+filepath.ToSlash(rel))
		return nil
	})
	require.NoError(t, err)
	sort.Strings(entries)
	return entries
}
