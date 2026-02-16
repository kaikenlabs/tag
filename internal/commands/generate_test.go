package commands

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/engine"
	"github.com/kaikenlabs/tag/internal/hooks"
	"github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/types/flags"
	"github.com/kaikenlabs/tag/pkg/app"
)

func TestUT_GenerateAction_MissingArguments(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "no arguments",
			args:    []string{},
			wantErr: "please provide the generator/bundle and the name",
		},
		{
			name:    "only generator name",
			args:    []string{"myGenerator"},
			wantErr: "please provide the generator/bundle and the name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := setupTempDir(t)
			cfg := createTestConfig(t, tmpDir)
			ctx := createTestCLIContext(t, tt.args, nil)

			err := generateAction(ctx, cfg)

			require.Error(t, err)
			var cmdErr *app.CommandError
			require.True(t, errors.As(err, &cmdErr))
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestUT_GenerateAction_NoConfig(t *testing.T) {
	ctx := createTestCLIContext(t, []string{"myGenerator", "myName"}, nil)

	err := generateAction(ctx, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "init")
}

func TestUT_GenerateAction_GeneratorNotFound(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"nonexistent", "myName"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
	})

	err := generateAction(ctx, cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "generator \"nonexistent\" not found")
}

func TestUT_GenerateAction_ValidGenerator(t *testing.T) {
	tmpDir := setupTempDir(t)

	// Create a simple template
	templateContent := `---
to: output/{{ .Name }}.txt
---
Hello {{ .Name }}`

	createGenerator(t, tmpDir, "hello", templateContent)
	createSharedDir(t, tmpDir)

	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"hello", "world"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
	})

	// Use a mock to verify the engine is called correctly
	var capturedData engine.Data
	originalNewEngine := newEngine
	newEngine = func(dryRun bool, dirPath string, sharedPath string) (engine.Generator, error) {
		mock := &mockGenerator{
			GenerateFunc: func(data engine.Data) error {
				capturedData = data
				return nil
			},
		}
		return mock, nil
	}
	t.Cleanup(func() { newEngine = originalNewEngine })

	err := generateAction(ctx, cfg)

	require.NoError(t, err)
	assert.Equal(t, "world", capturedData.Name)
}

func TestUT_GenerateAction_WithArgs(t *testing.T) {
	tmpDir := setupTempDir(t)

	templateContent := `---
to: output/{{ .Name }}.txt
---
Hello {{ .Name }} with {{ .Args }}`

	createGenerator(t, tmpDir, "hello", templateContent)
	createSharedDir(t, tmpDir)

	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"hello", "world", "extra-args"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
	})

	var capturedData engine.Data
	originalNewEngine := newEngine
	newEngine = func(dryRun bool, dirPath string, sharedPath string) (engine.Generator, error) {
		mock := &mockGenerator{
			GenerateFunc: func(data engine.Data) error {
				capturedData = data
				return nil
			},
		}
		return mock, nil
	}
	t.Cleanup(func() { newEngine = originalNewEngine })

	err := generateAction(ctx, cfg)

	require.NoError(t, err)
	assert.Equal(t, "world", capturedData.Name)
	assert.Equal(t, "extra-args", capturedData.Args)
}

func TestUT_GenerateAction_WithMeta(t *testing.T) {
	tmpDir := setupTempDir(t)

	templateContent := `---
to: output/{{ .Name }}.txt
---
Hello {{ .Name }}`

	createGenerator(t, tmpDir, "hello", templateContent)
	createSharedDir(t, tmpDir)

	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"hello", "world"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.MetaFlag:       []string{"key1=value1", "key2=value2"},
	})

	var capturedData engine.Data
	originalNewEngine := newEngine
	newEngine = func(dryRun bool, dirPath string, sharedPath string) (engine.Generator, error) {
		mock := &mockGenerator{
			GenerateFunc: func(data engine.Data) error {
				capturedData = data
				return nil
			},
		}
		return mock, nil
	}
	t.Cleanup(func() { newEngine = originalNewEngine })

	err := generateAction(ctx, cfg)

	require.NoError(t, err)
	assert.Equal(t, []string{"key1=value1", "key2=value2"}, capturedData.MetaArgs)
}

func TestUT_GenerateAction_GeneratorError(t *testing.T) {
	tmpDir := setupTempDir(t)

	templateContent := `---
to: output/{{ .Name }}.txt
---
Hello {{ .Name }}`

	createGenerator(t, tmpDir, "hello", templateContent)
	createSharedDir(t, tmpDir)

	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"hello", "world"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
	})

	originalNewEngine := newEngine
	newEngine = func(dryRun bool, dirPath string, sharedPath string) (engine.Generator, error) {
		mock := &mockGenerator{
			GenerateFunc: func(data engine.Data) error {
				return errors.New("template execution failed")
			},
		}
		return mock, nil
	}
	t.Cleanup(func() { newEngine = originalNewEngine })

	err := generateAction(ctx, cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error when generating template")
}

func TestUT_GenerateAction_EngineCreationError(t *testing.T) {
	tmpDir := setupTempDir(t)

	templateContent := `---
to: output/{{ .Name }}.txt
---
Hello {{ .Name }}`

	createGenerator(t, tmpDir, "hello", templateContent)
	createSharedDir(t, tmpDir)

	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"hello", "world"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
	})

	originalNewEngine := newEngine
	newEngine = func(dryRun bool, dirPath string, sharedPath string) (engine.Generator, error) {
		return nil, errors.New("failed to create engine")
	}
	t.Cleanup(func() { newEngine = originalNewEngine })

	err := generateAction(ctx, cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error creating engine")
}

func TestUT_GenerateBundle_BundleNotFound(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"nonexistent", "myName"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.BundlePathFlag: "_bundles",
		"bundle":             true,
	})

	err := generateAction(ctx, cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot open bundle file")
}

func TestUT_GenerateBundle_InvalidJSON(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	// Create invalid JSON bundle
	createBundle(t, tmpDir, "mybundle", "not valid json")

	ctx := createTestCLIContext(t, []string{"mybundle", "myName"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.BundlePathFlag: "_bundles",
		"bundle":             true,
	})

	err := generateAction(ctx, cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot decode bundle file")
}

func TestUT_GenerateBundle_ValidBundle(t *testing.T) {
	tmpDir := setupTempDir(t)

	templateContent := `---
to: output/{{ .Name }}.txt
---
Hello {{ .Name }}`

	createGenerator(t, tmpDir, "hello", templateContent)
	createSharedDir(t, tmpDir)

	// Create valid bundle
	bundleJSON := `{"name":"mybundle","generators":[{"name":"hello"}]}`
	createBundle(t, tmpDir, "mybundle", bundleJSON)

	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"mybundle", "world"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.BundlePathFlag: "_bundles",
		"bundle":             true,
	})

	var generateCalls int
	originalBundleEngine := newBundleEngine
	newBundleEngine = func(tmplEngine *template.Engine, dryRun bool, dirPath string, sharedPath string) (engine.Generator, error) {
		mock := &mockGenerator{
			GenerateFunc: func(data engine.Data) error {
				generateCalls++
				return nil
			},
		}
		return mock, nil
	}
	t.Cleanup(func() { newBundleEngine = originalBundleEngine })

	err := generateAction(ctx, cfg)

	require.NoError(t, err)
	assert.Equal(t, 1, generateCalls, "expected Generate to be called once for the single generator in bundle")
}

func TestUT_GenerateBundle_MultipleGenerators(t *testing.T) {
	tmpDir := setupTempDir(t)

	templateContent := `---
to: output/{{ .Name }}.txt
---
Hello {{ .Name }}`

	createGenerator(t, tmpDir, "gen1", templateContent)
	createGenerator(t, tmpDir, "gen2", templateContent)
	createSharedDir(t, tmpDir)

	bundleJSON := `{"name":"mybundle","generators":[{"name":"gen1"},{"name":"gen2"}]}`
	createBundle(t, tmpDir, "mybundle", bundleJSON)

	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"mybundle", "world"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.BundlePathFlag: "_bundles",
		"bundle":             true,
	})

	var generateCalls int
	originalBundleEngine := newBundleEngine
	newBundleEngine = func(tmplEngine *template.Engine, dryRun bool, dirPath string, sharedPath string) (engine.Generator, error) {
		mock := &mockGenerator{
			GenerateFunc: func(data engine.Data) error {
				generateCalls++
				return nil
			},
		}
		return mock, nil
	}
	t.Cleanup(func() { newBundleEngine = originalBundleEngine })

	err := generateAction(ctx, cfg)

	require.NoError(t, err)
	assert.Equal(t, 2, generateCalls, "expected Generate to be called twice for two generators in bundle")
}

func TestUT_RunHooks_EmptyHooks(t *testing.T) {
	err := runHooks([][]string{}, hooks.HookPhasePreGen)
	require.NoError(t, err)
}

func TestUT_RunHooks_NilHooks(t *testing.T) {
	err := runHooks(nil, hooks.HookPhasePreGen)
	require.NoError(t, err)
}

func TestUT_RunHooks_ValidHook(t *testing.T) {
	tmpDir := setupTempDir(t)

	// Create a simple script that succeeds
	scriptPath := filepath.Join(tmpDir, "test-hook.sh")
	err := os.WriteFile(scriptPath, []byte("#!/bin/bash\nexit 0"), 0o755)
	require.NoError(t, err)

	// Change to temp dir so hook paths resolve
	oldDir, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(oldDir) })

	// Use relative path with ./ prefix to indicate it's a local file
	err = runHooks([][]string{{"./test-hook.sh"}}, hooks.HookPhasePreGen)
	require.NoError(t, err)
}

func TestUT_RunHooks_FailingHook(t *testing.T) {
	tmpDir := setupTempDir(t)

	// Create a script that fails
	scriptPath := filepath.Join(tmpDir, "failing-hook.sh")
	err := os.WriteFile(scriptPath, []byte("#!/bin/bash\nexit 1"), 0o755)
	require.NoError(t, err)

	oldDir, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(oldDir) })

	// Use relative path with ./ prefix to indicate it's a local file
	err = runHooks([][]string{{"./failing-hook.sh"}}, hooks.HookPhasePreGen)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hook failed")
}

func TestUT_RunHooks_StopsOnFailure(t *testing.T) {
	tmpDir := setupTempDir(t)

	// Create first hook that fails
	script1Path := filepath.Join(tmpDir, "hook1.sh")
	err := os.WriteFile(script1Path, []byte("#!/bin/bash\nexit 1"), 0o755)
	require.NoError(t, err)

	// Create second hook that succeeds (but shouldn't run)
	script2Path := filepath.Join(tmpDir, "hook2.sh")
	markerFile := filepath.Join(tmpDir, "hook2-ran")
	err = os.WriteFile(script2Path, []byte("#!/bin/bash\ntouch "+markerFile+"\nexit 0"), 0o755)
	require.NoError(t, err)

	oldDir, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(oldDir) })

	// Use relative paths with ./ prefix to indicate they are local files
	err = runHooks([][]string{{"./hook1.sh"}, {"./hook2.sh"}}, hooks.HookPhasePreGen)
	require.Error(t, err)

	// Verify second hook didn't run
	_, statErr := os.Stat(markerFile)
	assert.True(t, os.IsNotExist(statErr), "second hook should not have run")
}

func TestUT_RunHooks_NonexistentCommand(t *testing.T) {
	tmpDir := setupTempDir(t)

	oldDir, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(oldDir) })

	err = runHooks([][]string{{"nonexistent-command-xyz"}}, hooks.HookPhasePreGen)
	require.Error(t, err)
}

func TestUT_RunHooks_PathCommand(t *testing.T) {
	// Test that PATH commands work (e.g., "echo" should be found in PATH)
	err := runHooks([][]string{{"echo", "hello"}}, hooks.HookPhasePreGen)
	require.NoError(t, err)
}

func TestUT_RunHooks_RelativePathCommand(t *testing.T) {
	tmpDir := setupTempDir(t)

	// Create a script
	scriptPath := filepath.Join(tmpDir, "myscript.sh")
	err := os.WriteFile(scriptPath, []byte("#!/bin/bash\nexit 0"), 0o755)
	require.NoError(t, err)

	oldDir, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(oldDir) })

	// Relative path should resolve from working directory
	err = runHooks([][]string{{"./myscript.sh"}}, hooks.HookPhasePreGen)
	require.NoError(t, err)
}

// --- resolveGeneratorPaths tests ---

func TestUT_ResolveGeneratorPaths_LocalFallback(t *testing.T) {
	tmpDir := setupTempDir(t)

	// Create a local generator
	genDir := filepath.Join(tmpDir, "myGen")
	require.NoError(t, os.MkdirAll(genDir, 0o750))

	cfg := &config.Config{
		Env: config.Env{Path: tmpDir},
	}

	gotGen, gotShared, err := resolveGeneratorPaths(cfg, "myGen")

	require.NoError(t, err)
	assert.Equal(t, genDir, gotGen)
	assert.Equal(t, filepath.Join(tmpDir, "_shared"), gotShared)
}

func TestUT_ResolveGeneratorPaths_NotFound_NoTemplate(t *testing.T) {
	tmpDir := setupTempDir(t)

	cfg := &config.Config{
		Env: config.Env{Path: tmpDir},
	}

	_, _, err := resolveGeneratorPaths(cfg, "nonexistent")

	require.Error(t, err)
	assert.Contains(t, err.Error(), `generator "nonexistent" not found`)
}

func TestUT_ResolveGeneratorPaths_NotFound_WithTemplate(t *testing.T) {
	tmpDir := setupTempDir(t)

	cfg := &config.Config{
		Template: &config.TemplateOrigin{
			Source: "gh:acme/my-template",
			Name:   "my-template",
		},
		Env: config.Env{Path: tmpDir},
	}

	_, _, err := resolveGeneratorPaths(cfg, "nonexistent")

	require.Error(t, err)
	assert.Contains(t, err.Error(), `generator "nonexistent" not found in template "my-template"`)
	assert.Contains(t, err.Error(), "tag lib add gh:acme/my-template")
}

func TestUT_ResolveGeneratorPaths_EmptyPath(t *testing.T) {
	cfg := &config.Config{
		Env: config.Env{Path: ""},
	}

	_, _, err := resolveGeneratorPaths(cfg, "myGen")

	require.Error(t, err)
	assert.Contains(t, err.Error(), `generator "myGen" not found`)
}

func TestUT_GenerateTemplate_PassesScaffoldVars(t *testing.T) {
	tmpDir := setupTempDir(t)

	templateContent := `---
to: output/{{ .Name }}.txt
---
Hello {{ .Name }}`

	createGenerator(t, tmpDir, "hello", templateContent)
	createSharedDir(t, tmpDir)

	cfg := createTestConfig(t, tmpDir)
	cfg.Variables = map[string]any{
		"project_name": "my-project",
		"use_docker":   true,
	}

	ctx := createTestCLIContext(t, []string{"hello", "world"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
	})

	var capturedData engine.Data
	originalNewEngine := newEngine
	newEngine = func(dryRun bool, dirPath string, sharedPath string) (engine.Generator, error) {
		mock := &mockGenerator{
			GenerateFunc: func(data engine.Data) error {
				capturedData = data
				return nil
			},
		}
		return mock, nil
	}
	t.Cleanup(func() { newEngine = originalNewEngine })

	err := generateAction(ctx, cfg)

	require.NoError(t, err)
	assert.Equal(t, "world", capturedData.Name)
	assert.Equal(t, "my-project", capturedData.ScaffoldVars["project_name"])
	assert.Equal(t, true, capturedData.ScaffoldVars["use_docker"])
}

func TestUT_GenerateTemplate_NilVariablesNoScaffoldVars(t *testing.T) {
	tmpDir := setupTempDir(t)

	templateContent := `---
to: output/{{ .Name }}.txt
---
Hello {{ .Name }}`

	createGenerator(t, tmpDir, "hello", templateContent)
	createSharedDir(t, tmpDir)

	cfg := createTestConfig(t, tmpDir)
	// cfg.Variables is nil (no scaffold vars)

	ctx := createTestCLIContext(t, []string{"hello", "world"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
	})

	var capturedData engine.Data
	originalNewEngine := newEngine
	newEngine = func(dryRun bool, dirPath string, sharedPath string) (engine.Generator, error) {
		mock := &mockGenerator{
			GenerateFunc: func(data engine.Data) error {
				capturedData = data
				return nil
			},
		}
		return mock, nil
	}
	t.Cleanup(func() { newEngine = originalNewEngine })

	err := generateAction(ctx, cfg)

	require.NoError(t, err)
	assert.Nil(t, capturedData.ScaffoldVars)
}

// --- resolveBundlePath tests ---

func TestUT_ResolveBundlePath_LocalFallback(t *testing.T) {
	tmpDir := setupTempDir(t)

	// Create a local bundle
	createBundle(t, tmpDir, "mybundle", `{"name":"mybundle","generators":[]}`)

	cfg := &config.Config{
		Env: config.Env{Path: tmpDir},
	}

	got, err := resolveBundlePath(cfg, "mybundle", "_bundles")

	require.NoError(t, err)
	expected := filepath.Join(tmpDir, "_bundles", "mybundle", "mybundle.json")
	assert.Equal(t, expected, got)
}

func TestUT_ResolveBundlePath_NotFound(t *testing.T) {
	tmpDir := setupTempDir(t)

	cfg := &config.Config{
		Env: config.Env{Path: tmpDir},
	}

	_, err := resolveBundlePath(cfg, "nonexistent", "_bundles")

	require.Error(t, err)
	assert.Contains(t, err.Error(), `bundle "nonexistent" not found`)
}

func TestUT_GenerateBundle_PassesScaffoldVars(t *testing.T) {
	tmpDir := setupTempDir(t)

	templateContent := `---
to: output/{{ .Name }}.txt
---
Hello {{ .Name }}`

	createGenerator(t, tmpDir, "hello", templateContent)
	createSharedDir(t, tmpDir)

	bundleJSON := `{"name":"mybundle","generators":[{"name":"hello"}]}`
	createBundle(t, tmpDir, "mybundle", bundleJSON)

	cfg := createTestConfig(t, tmpDir)
	cfg.Variables = map[string]any{
		"project_name": "my-project",
	}

	ctx := createTestCLIContext(t, []string{"mybundle", "world"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.BundlePathFlag: "_bundles",
		"bundle":             true,
	})

	var capturedData engine.Data
	originalBundleEngine := newBundleEngine
	newBundleEngine = func(tmplEngine *template.Engine, dryRun bool, dirPath string, sharedPath string) (engine.Generator, error) {
		mock := &mockGenerator{
			GenerateFunc: func(data engine.Data) error {
				capturedData = data
				return nil
			},
		}
		return mock, nil
	}
	t.Cleanup(func() { newBundleEngine = originalBundleEngine })

	err := generateAction(ctx, cfg)

	require.NoError(t, err)
	assert.Equal(t, "my-project", capturedData.ScaffoldVars["project_name"])
}

// --- generateList tests ---

func TestUT_GenerateList_NoGenerators(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	var buf bytes.Buffer
	err := generateList(cfg, &buf)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No generators found.")
}

func TestUT_GenerateList_LocalGenerators(t *testing.T) {
	tmpDir := setupTempDir(t)

	// Create local generators
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "component"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "page"), 0o750))
	// Create _shared (should be excluded from listing)
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "_shared"), 0o750))

	cfg := createTestConfig(t, tmpDir)

	var buf bytes.Buffer
	err := generateList(cfg, &buf)

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "component")
	assert.Contains(t, output, "page")
	assert.NotContains(t, output, "_shared")
	assert.Contains(t, output, "PROJECT GENERATORS")
}

func TestUT_GenerateList_GeneratorWithDescription(t *testing.T) {
	tmpDir := setupTempDir(t)

	// Create a generator with tag.template.json
	genDir := filepath.Join(tmpDir, "component")
	require.NoError(t, os.MkdirAll(genDir, 0o750))
	configJSON := `{"description": "Create a React component"}`
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "tag.template.json"), []byte(configJSON), 0o644))

	cfg := createTestConfig(t, tmpDir)

	err := generateList(cfg, io.Discard)

	require.NoError(t, err)
}

func TestUT_GenerateList_WithBundles(t *testing.T) {
	tmpDir := setupTempDir(t)

	// Create a generator
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "component"), 0o750))

	// Create a bundle
	createBundle(t, tmpDir, "feature", `{"name":"feature","generators":[{"name":"component"}]}`)

	cfg := createTestConfig(t, tmpDir)

	err := generateList(cfg, io.Discard)

	require.NoError(t, err)
}

func TestUT_GenerateList_NoConfig(t *testing.T) {
	err := generateList(nil, io.Discard)

	require.Error(t, err)
}

func TestUT_ScanGenerators_SkipsReserved(t *testing.T) {
	tmpDir := setupTempDir(t)

	// Create generators and reserved dirs
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "component"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "_shared"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "_bundles"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".hidden"), 0o750))

	result := scanGenerators(tmpDir)

	require.Len(t, result, 1)
	assert.Equal(t, "component", result[0].Name)
}

func TestUT_ScanGenerators_NonexistentDir(t *testing.T) {
	result := scanGenerators("/nonexistent/path")
	assert.Nil(t, result)
}

func TestUT_ScanBundles_SkipsReserved(t *testing.T) {
	tmpDir := setupTempDir(t)

	// Create bundle dirs
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "feature"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "feature", "feature.json"), []byte(`{"name":"feature"}`), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "_internal"), 0o750))

	result := scanBundles(tmpDir)

	require.Len(t, result, 1)
	assert.Equal(t, "feature", result[0].Name)
}

func TestUT_GeneratorNotFoundError_WithTemplate(t *testing.T) {
	err := &GeneratorNotFoundError{
		Generator: "component",
		Template:  "nextjs-starter",
		Source:    "gh:acme/nextjs-starter",
	}

	msg := err.Error()
	assert.Contains(t, msg, `generator "component" not found in template "nextjs-starter"`)
	assert.Contains(t, msg, "tag lib add gh:acme/nextjs-starter")
}

func TestUT_GeneratorNotFoundError_LocalOnly(t *testing.T) {
	err := &GeneratorNotFoundError{
		Generator: "component",
		LocalPath: ".tag",
	}

	msg := err.Error()
	assert.Contains(t, msg, `generator "component" not found in .tag`)
}

// --- Library-first resolution tests ---

func TestUT_ResolveGeneratorPaths_LibraryFirst(t *testing.T) {
	// Generator found in library template — should return library path.
	templateDir := setupFakeLibrary(t, "my-template")

	// Create generator inside the library template's .tag dir
	genDir := filepath.Join(templateDir, ".tag", "component")
	require.NoError(t, os.MkdirAll(genDir, 0o750))

	cfg := createTestConfigWithLib(t, t.TempDir(), "my-template")

	gotGen, gotShared, err := resolveGeneratorPaths(cfg, "component")

	require.NoError(t, err)
	assert.Equal(t, genDir, gotGen)
	assert.Equal(t, filepath.Join(templateDir, ".tag", "_shared"), gotShared)
}

func TestUT_ResolveGeneratorPaths_LibraryFallbackToLocal(t *testing.T) {
	// Generator not in library template — should fall back to local .tag/.
	templateDir := setupFakeLibrary(t, "my-template")

	// Create .tag dir in library but WITHOUT the requested generator
	require.NoError(t, os.MkdirAll(filepath.Join(templateDir, ".tag"), 0o750))

	// Create local generator
	tmpDir := setupTempDir(t)
	localGenDir := filepath.Join(tmpDir, "component")
	require.NoError(t, os.MkdirAll(localGenDir, 0o750))

	cfg := createTestConfigWithLib(t, tmpDir, "my-template")

	gotGen, _, err := resolveGeneratorPaths(cfg, "component")

	require.NoError(t, err)
	assert.Equal(t, localGenDir, gotGen)
}

func TestUT_ResolveBundlePath_LibraryFirst(t *testing.T) {
	// Bundle found in library template — should return library path.
	templateDir := setupFakeLibrary(t, "my-template")

	// Create bundle inside the library template's .tag/_bundles dir
	bundleDir := filepath.Join(templateDir, ".tag", "_bundles", "fullstack")
	require.NoError(t, os.MkdirAll(bundleDir, 0o750))
	bundlePath := filepath.Join(bundleDir, "fullstack.json")
	require.NoError(t, os.WriteFile(bundlePath, []byte(`{"name":"fullstack","generators":[]}`), 0o644))

	cfg := createTestConfigWithLib(t, t.TempDir(), "my-template")

	got, err := resolveBundlePath(cfg, "fullstack", "_bundles")

	require.NoError(t, err)
	assert.Equal(t, bundlePath, got)
}

func TestUT_WarnVersionMismatch_PrintsWarning(t *testing.T) {
	templateDir := setupFakeLibrary(t, "my-template")

	// Write a tag.template.json with a different version in the library
	configJSON := `{"version": "2.0.0", "description": "test template"}`
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "tag.template.json"), []byte(configJSON), 0o644))

	cfg := createTestConfigWithLib(t, t.TempDir(), "my-template")
	cfg.Template.Version = "1.0.0" // scaffolded version differs

	// Capture stderr
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w

	warnVersionMismatch(cfg, templateDir)

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	assert.Contains(t, output, "Warning: template version mismatch")
	assert.Contains(t, output, "1.0.0")
	assert.Contains(t, output, "2.0.0")
}

func TestUT_WarnVersionMismatch_NoWarningWhenMatch(t *testing.T) {
	templateDir := setupFakeLibrary(t, "my-template")

	configJSON := `{"version": "1.0.0"}`
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "tag.template.json"), []byte(configJSON), 0o644))

	cfg := createTestConfigWithLib(t, t.TempDir(), "my-template")
	cfg.Template.Version = "1.0.0"

	// Capture stderr
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w

	warnVersionMismatch(cfg, templateDir)

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	assert.Empty(t, buf.String(), "no warning expected when versions match")
}

func TestUT_WarnVersionMismatch_SkipsWhenNoVersion(t *testing.T) {
	templateDir := setupFakeLibrary(t, "my-template")

	cfg := createTestConfigWithLib(t, t.TempDir(), "my-template")
	cfg.Template.Version = "" // no version recorded at scaffold time

	// Capture stderr
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w

	warnVersionMismatch(cfg, templateDir)

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	assert.Empty(t, buf.String(), "no warning expected when scaffold version is empty")
}
