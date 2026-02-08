package commands

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/engine"
	"github.com/kaikenlabs/tag/internal/types/flags"
	"github.com/urfave/cli/v2"
)

// mockGenerator is a test double for engine.Generator.
type mockGenerator struct {
	GenerateFunc  func(data engine.Data) error
	GenerateCalls []engine.Data
}

func (m *mockGenerator) Generate(data engine.Data) error {
	m.GenerateCalls = append(m.GenerateCalls, data)
	if m.GenerateFunc != nil {
		return m.GenerateFunc(data)
	}
	return nil
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
			Extension:  ".tmpl",
			SharedPath: "_shared",
			BundlePath: "_bundles",
		},
		Hooks: config.Hooks{
			Pre:  [][]string{},
			Post: [][]string{},
		},
	}
}

// createTestCLIContext creates a cli.Context for testing with the given args and flags.
func createTestCLIContext(t *testing.T, args []string, flagValues map[string]interface{}) *cli.Context {
	t.Helper()

	app := &cli.App{
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: flags.DryRunFlag},
			&cli.StringFlag{Name: flags.PathFlag, Value: ".tag.templates"},
			&cli.StringFlag{Name: flags.ExtensionFlag, Value: ".tmpl"},
			&cli.StringFlag{Name: flags.SharedPathFlag, Value: "_shared"},
			&cli.StringFlag{Name: flags.BundlePathFlag, Value: "_bundles"},
			&cli.BoolFlag{Name: "bundle"},
			&cli.StringSliceFlag{Name: flags.MetaFlag},
			&cli.StringFlag{Name: "package", Value: "mypackage"},
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

	templatePath := filepath.Join(genDir, generatorName+".tmpl")
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
