package commands

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/types/flags"
	"github.com/kaikenlabs/tag/pkg/app"
)

// listTreeEntries returns a sorted, root-relative listing of every entry
// (files AND directories) under root, so a before/after comparison catches
// both unexpected additions and unexpected content changes at the directory
// level, not just missing files.
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

func TestUT_NewAction_MissingGeneratorName(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{}, map[string]any{
		flags.PathFlag: tmpDir,
	})

	err := newAction(ctx, cfg)

	require.Error(t, err)
	var cmdErr *app.CommandError
	require.ErrorAs(t, err, &cmdErr)
	assert.Contains(t, err.Error(), "please provide the generator name")
}

func TestUT_NewAction_NoConfig(t *testing.T) {
	ctx := createTestCLIContext(t, []string{"myGenerator"}, nil)

	err := newAction(ctx, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "init")
}

func TestUT_NewAction_ValidGeneratorCreation(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"myGenerator"}, map[string]any{
		flags.PathFlag: tmpDir,
		"package":      "mypackage",
	})

	err := newAction(ctx, cfg)

	require.NoError(t, err)

	// Verify generator file was created
	generatorPath := filepath.Join(tmpDir, "myGenerator", "myGenerator.go")
	require.FileExists(t, generatorPath)

	// Verify content contains expected template structure
	data, err := os.ReadFile(generatorPath)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "---")
	assert.Contains(t, content, "to:")
	assert.Contains(t, content, "mypackage")
}

func TestUT_NewAction_CustomPackage(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"myGenerator"}, map[string]any{
		flags.PathFlag: tmpDir,
		"package":      "custompackage",
	})

	err := newAction(ctx, cfg)

	require.NoError(t, err)

	generatorPath := filepath.Join(tmpDir, "myGenerator", "myGenerator.go")
	data, err := os.ReadFile(generatorPath)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "custompackage")
}

func TestUT_NewAction_CreatesDirectory(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"newgenerator"}, map[string]any{
		flags.PathFlag: tmpDir,
	})

	// Ensure generator directory doesn't exist yet
	genDir := filepath.Join(tmpDir, "newgenerator")
	_, err := os.Stat(genDir)
	require.True(t, os.IsNotExist(err))

	err = newAction(ctx, cfg)
	require.NoError(t, err)

	// Verify directory was created
	info, err := os.Stat(genDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestUT_NewAction_TemplateContent(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"myGenerator"}, map[string]any{
		flags.PathFlag: tmpDir,
		"package":      "testpkg",
	})

	err := newAction(ctx, cfg)

	require.NoError(t, err)

	generatorPath := filepath.Join(tmpDir, "myGenerator", "myGenerator.go")
	data, err := os.ReadFile(generatorPath)
	require.NoError(t, err)

	content := string(data)

	// Verify template has metadata block
	assert.True(t, strings.HasPrefix(content, "---"), "should start with metadata delimiter")
	assert.Contains(t, content, "to:")

	// Verify template has package declaration
	assert.Contains(t, content, "package testpkg")

	// Verify template uses Gonja name variable with snake filter
	assert.Contains(t, content, "{{ name | snake }}")
}

func TestUT_NewAction_LibFlag_ValidCreation(t *testing.T) {
	templateDir := setupFakeLibrary(t, "my-template")
	tmpDir := setupTempDir(t)
	cfg := createTestConfigWithLib(t, tmpDir, "my-template")

	ctx := createTestCLIContext(t, []string{"myGenerator"}, map[string]any{
		flags.LibFlag: true,
		"package":     "mypackage",
	})

	err := newAction(ctx, cfg)

	require.NoError(t, err)

	// Verify generator was created inside the library template's .tag directory
	generatorPath := filepath.Join(templateDir, ".tag", "myGenerator", "myGenerator.go")
	require.FileExists(t, generatorPath)

	data, readErr := os.ReadFile(generatorPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "mypackage")
}

func TestUT_NewAction_LibFlag_NonExistentTemplate(t *testing.T) {
	setupFakeLibrary(t, "existing-template")
	tmpDir := setupTempDir(t)
	cfg := createTestConfigWithLib(t, tmpDir, "nonexistent")

	ctx := createTestCLIContext(t, []string{"myGenerator"}, map[string]any{
		flags.LibFlag: true,
		"package":     "mypackage",
	})

	err := newAction(ctx, cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUT_NewAction_LibFlag_NoTemplateOrigin(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"myGenerator"}, map[string]any{
		flags.LibFlag: true,
		"package":     "mypackage",
	})

	err := newAction(ctx, cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no library template configured")
}

func TestUT_NewAction_BundleFlag(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	// Create the bundle directory first (simulates tag new-bundle was run)
	bundleDir := filepath.Join(tmpDir, "_bundles", "mybundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0o750))

	ctx := createTestCLIContext(t, []string{"mygen"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.BundlePathFlag: "_bundles",
		flags.InBundleFlag:   "mybundle",
		"package":            "mypackage",
	})

	err := newAction(ctx, cfg)

	require.NoError(t, err)

	// Verify generator was created inside the bundle directory
	generatorPath := filepath.Join(bundleDir, "mygen", "mygen.go")
	require.FileExists(t, generatorPath)

	data, err := os.ReadFile(generatorPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "mypackage")
}

func TestUT_NewAction_BundleFlag_BundleNotFound(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"mygen"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.BundlePathFlag: "_bundles",
		flags.InBundleFlag:   "nonexistent",
		"package":            "mypackage",
	})

	err := newAction(ctx, cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
	assert.Contains(t, err.Error(), "tag template new bundle nonexistent")
}

func TestUT_NewCommand_ReturnsValidCommand(t *testing.T) {
	cfg := createTestConfig(t, ".tag")
	cmd := templateNewGeneratorCommand(cfg)

	require.NotNil(t, cmd)
	assert.Equal(t, "generator", cmd.Name)
	assert.NotEmpty(t, cmd.Usage)
	assert.NotNil(t, cmd.Action)
	assert.True(t, cmd.Args)
	assert.Equal(t, "<generator-name>", cmd.ArgsUsage)

	// Verify description is present
	assert.NotEmpty(t, cmd.Description)

	// Verify package flag exists with correct alias
	var hasPackageFlag bool
	for _, f := range cmd.Flags {
		sf, ok := f.(*cli.StringFlag)
		if !ok || sf.Name != "package" {
			continue
		}
		hasPackageFlag = true
		assert.Equal(t, "mypackage", sf.Value)
		assert.Contains(t, sf.Aliases, "p", "package flag should have -p alias")
		assert.NotContains(t, sf.Aliases, "k", "package flag should not have old -k alias")
		break
	}
	assert.True(t, hasPackageFlag, "should have package flag")

	// Verify lib flag exists
	var hasLibFlag bool
	for _, f := range cmd.Flags {
		if bf, ok := f.(*cli.BoolFlag); ok && bf.Name == flags.LibFlag {
			hasLibFlag = true
			break
		}
	}
	assert.True(t, hasLibFlag, "should have lib flag")

	// Verify bundle flag exists
	var hasBundleFlag bool
	for _, f := range cmd.Flags {
		if sf, ok := f.(*cli.StringFlag); ok && sf.Name == flags.InBundleFlag {
			hasBundleFlag = true
			break
		}
	}
	assert.True(t, hasBundleFlag, "should have bundle flag")
}

func TestUT_NewAction_BundleDirEscapingBaseIsRejected(t *testing.T) {
	type escapeCase struct {
		name               string
		skipWindows        bool
		wantNoDoesNotExist bool
		setup              func(t *testing.T, base, outside string) (bundleSubPath, bundleName, generatorName, canaryPath, wantGenFile, wantGenDirCheck string)
	}

	cases := []escapeCase{
		{
			name: "relative traversal via --bundle-path",
			setup: func(t *testing.T, base, outside string) (string, string, string, string, string, string) {
				t.Helper()
				bundleDir := filepath.Join(outside, "evilbundle")
				require.NoError(t, os.MkdirAll(bundleDir, 0o750))
				canary := filepath.Join(bundleDir, "canary.txt")
				require.NoError(t, os.WriteFile(canary, []byte("canary"), 0o644))
				genFile := filepath.Join(bundleDir, "mygen", "mygen.go")
				return "../outside", "evilbundle", "mygen", canary, genFile, filepath.Dir(genFile)
			},
		},
		{
			name:               "relative traversal to a nonexistent bundle dir",
			wantNoDoesNotExist: true,
			setup: func(t *testing.T, base, outside string) (string, string, string, string, string, string) {
				t.Helper()
				bundleDir := filepath.Join(outside, "nosuchbundle")
				genFile := filepath.Join(bundleDir, "mygen", "mygen.go")
				return "../outside", "nosuchbundle", "mygen", "", genFile, filepath.Dir(genFile)
			},
		},
		{
			// The ONLY assertion here that would fail a cheaper, WRONG fix such as
			// a lexical `strings.Contains(bundleSubPath, "..")` guard: bundleSubPath
			// is the unremarkable default "_bundles", the escape is entirely via a
			// symlink that os.Stat (and MkdirAll) transparently follow.
			name:        "symlinked bundle directory",
			skipWindows: true,
			setup: func(t *testing.T, base, outside string) (string, string, string, string, string, string) {
				t.Helper()
				require.NoError(t, os.MkdirAll(filepath.Join(base, "_bundles"), 0o750))
				target := filepath.Join(outside, "linkedbundle")
				require.NoError(t, os.MkdirAll(target, 0o750))
				canary := filepath.Join(target, "canary.txt")
				require.NoError(t, os.WriteFile(canary, []byte("canary"), 0o644))
				require.NoError(t, os.Symlink(target, filepath.Join(base, "_bundles", "linked")))
				genFile := filepath.Join(target, "mygen", "mygen.go")
				return "", "linked", "mygen", canary, genFile, filepath.Dir(genFile)
			},
		},
		{
			// Exercises the SECOND, surviving ValidatePathContainment(basePath, dirPath)
			// call: bundleDir itself (base/_bundles/realbundle) is genuinely inside
			// base, so the new pre-Stat check passes here. The escape is a symlinked
			// generator subdirectory found only by the existing check further down.
			// This is what stops a future refactor from deleting that second check
			// as "now redundant".
			name:        "generator subdir symlinks out of bundle",
			skipWindows: true,
			setup: func(t *testing.T, base, outside string) (string, string, string, string, string, string) {
				t.Helper()
				realBundle := filepath.Join(base, "_bundles", "realbundle")
				require.NoError(t, os.MkdirAll(realBundle, 0o750))
				target := filepath.Join(outside, "genescape")
				require.NoError(t, os.MkdirAll(target, 0o750))
				canary := filepath.Join(target, "canary.txt")
				require.NoError(t, os.WriteFile(canary, []byte("canary"), 0o644))
				require.NoError(t, os.Symlink(target, filepath.Join(realBundle, "mygen")))
				genFile := filepath.Join(target, "mygen.go")
				return "", "realbundle", "mygen", canary, genFile, ""
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skipWindows && runtime.GOOS == "windows" {
				t.Skip("symlinks require elevated privileges on windows")
			}

			parent := setupTempDir(t)
			base := filepath.Join(parent, "base")
			outside := filepath.Join(parent, "outside")
			require.NoError(t, os.MkdirAll(base, 0o750))
			require.NoError(t, os.MkdirAll(outside, 0o750))

			bundleSubPath, bundleName, generatorName, canaryPath, wantGenFile, wantGenDirCheck := tc.setup(t, base, outside)

			var canaryBefore []byte
			if canaryPath != "" {
				var readErr error
				canaryBefore, readErr = os.ReadFile(canaryPath)
				require.NoError(t, readErr)
			}
			outsideBefore := listTreeEntries(t, outside)

			cfg := createTestConfig(t, base)
			flagValues := map[string]any{
				flags.InBundleFlag: bundleName,
				"package":          "mypackage",
			}
			if bundleSubPath != "" {
				flagValues[flags.BundlePathFlag] = bundleSubPath
			}
			ctx := createTestCLIContext(t, []string{generatorName}, flagValues)

			err := newAction(ctx, cfg)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "path safety check failed")
			assert.Contains(t, err.Error(), "escapes base directory")
			if tc.wantNoDoesNotExist {
				assert.NotContains(t, err.Error(), "does not exist")
			}

			require.NoFileExists(t, wantGenFile)
			if wantGenDirCheck != "" {
				require.NoDirExists(t, wantGenDirCheck)
			}

			if canaryPath != "" {
				canaryAfter, readErr := os.ReadFile(canaryPath)
				require.NoError(t, readErr)
				assert.Equal(t, canaryBefore, canaryAfter)
			}

			outsideAfter := listTreeEntries(t, outside)
			assert.Equal(t, outsideBefore, outsideAfter)
		})
	}
}
