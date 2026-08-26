package commands

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/urfave/cli/v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/engine"
	"github.com/kaikenlabs/tag/internal/fileaction"
	"github.com/kaikenlabs/tag/internal/history"
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

			err := generateAction(ctx, cfg, defaultGeneratorFactories())

			require.Error(t, err)
			var cmdErr *app.CommandError
			require.True(t, errors.As(err, &cmdErr))
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestUT_GenerateAction_NoConfig(t *testing.T) {
	ctx := createTestCLIContext(t, []string{"myGenerator", "myName"}, nil)

	err := generateAction(ctx, nil, defaultGeneratorFactories())

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

	err := generateAction(ctx, cfg, defaultGeneratorFactories())

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
	fac := defaultGeneratorFactories()
	fac.newEngine = func(dryRun bool, dirPath string, sharedPath string, rec *history.Recorder, _ io.Writer) (engine.Generator, error) {
		mock := &mockGenerator{
			GenerateFunc: func(data engine.Data) (engine.GenerateResult, error) {
				capturedData = data
				return engine.GenerateResult{}, nil
			},
		}
		return mock, nil
	}

	err := generateAction(ctx, cfg, fac)

	require.NoError(t, err)
	assert.Equal(t, "world", capturedData.Name)
}

func TestUT_GenerateAction_WithArgs(t *testing.T) {
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

	var capturedData engine.Data
	fac := defaultGeneratorFactories()
	fac.newEngine = func(dryRun bool, dirPath string, sharedPath string, rec *history.Recorder, _ io.Writer) (engine.Generator, error) {
		mock := &mockGenerator{
			GenerateFunc: func(data engine.Data) (engine.GenerateResult, error) {
				capturedData = data
				return engine.GenerateResult{}, nil
			},
		}
		return mock, nil
	}

	err := generateAction(ctx, cfg, fac)

	require.NoError(t, err)
	assert.Equal(t, "world", capturedData.Name)
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
	fac := defaultGeneratorFactories()
	fac.newEngine = func(dryRun bool, dirPath string, sharedPath string, rec *history.Recorder, _ io.Writer) (engine.Generator, error) {
		mock := &mockGenerator{
			GenerateFunc: func(data engine.Data) (engine.GenerateResult, error) {
				capturedData = data
				return engine.GenerateResult{}, nil
			},
		}
		return mock, nil
	}

	err := generateAction(ctx, cfg, fac)

	require.NoError(t, err)
	assert.Equal(t, []string{"key1=value1", "key2=value2"}, capturedData.RawMeta)
}

func TestUT_GenerateAction_MetaRenderedInOutput(t *testing.T) {
	// Full-flow reproduction test: CLI -m flags must be accessible as {{ vars.* }} in rendered output.
	// Unlike TestUT_GenerateAction_WithMeta (which uses a mock), this uses the real Gonja engine
	// and verifies the actual file content written to disk.
	tmpDir := setupTempDir(t)

	// Use real Gonja template syntax (not the mock {{ .Name }} syntax).
	templateContent := "---\nto: {{ name }}.txt\n---\nname: {{ name }}\nfields: {{ vars.fields }}\ndomain: {{ vars.domain }}\n"

	createGenerator(t, tmpDir, "hello", templateContent)
	createSharedDir(t, tmpDir)

	cfg := createTestConfig(t, tmpDir)
	cfg.Hooks = config.Hooks{Pre: [][]string{}, Post: [][]string{}}

	ctx := createTestCLIContext(t, []string{"hello", "widget"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: filepath.Join(tmpDir, "_shared"),
		flags.MetaFlag:       []string{"fields=name:string", "domain=tenant"},
		flags.NoHooksFlag:    true,
	})

	// Chdir to temp dir so the file writer creates output there.
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { os.Chdir(origDir) })

	// Use the real engine factory — no mocks.
	fac := defaultGeneratorFactories()

	err = generateAction(ctx, cfg, fac)
	require.NoError(t, err)

	// Verify the rendered file was written with correct content.
	output, err := os.ReadFile(filepath.Join(tmpDir, "widget.txt"))
	require.NoError(t, err, "expected widget.txt to be created")

	assert.Contains(t, string(output), "name: widget")
	assert.Contains(t, string(output), "fields: name:string")
	assert.Contains(t, string(output), "domain: tenant")
}

func TestUT_GenerateAction_MetaViaAppRun(t *testing.T) {
	// End-to-end test: exercises real CLI arg parsing via app.Run() with -m flags.
	// This is the only test that covers the actual urfave/cli StringSlice parsing path,
	// which is the last untested layer between the user's shell and the engine.
	//
	// urfave/cli v2 requires flags BEFORE positional args (POSIX compliance) —
	// "tag generate -m key=val gen name" is the native path. A trailing
	// "tag generate gen name -m key=val" used to be silently swallowed as
	// extra positional args; generateAction now calls reparseTrailingFlags
	// (needed so a trailing --format works at all), which fixes that as a
	// side effect for every flag on generateFlags(), --meta included.
	tests := []struct {
		name     string
		args     []string
		wantMeta bool
	}{
		{
			name:     "flags before args",
			args:     []string{"tag", "generate", "-m", "fields=name:string", "-m", "domain=tenant", "--no-hooks", "hello", "widget"},
			wantMeta: true,
		},
		{
			name:     "flags after args are now recognised via reparseTrailingFlags",
			args:     []string{"tag", "generate", "hello", "widget", "-m", "fields=name:string", "-m", "domain=tenant", "--no-hooks"},
			wantMeta: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := setupTempDir(t)

			templateContent := "---\nto: {{ name }}.txt\n---\nname: {{ name }}\nfields: {{ vars.fields }}\ndomain: {{ vars.domain }}\n"

			createGenerator(t, tmpDir, "hello", templateContent)
			createSharedDir(t, tmpDir)

			origDir, err := os.Getwd()
			require.NoError(t, err)
			require.NoError(t, os.Chdir(tmpDir))
			t.Cleanup(func() { os.Chdir(origDir) })

			cfg := createTestConfig(t, tmpDir)
			cfg.Hooks = config.Hooks{Pre: [][]string{}, Post: [][]string{}}

			cliApp := &cli.App{
				Writer: io.Discard,
				Commands: []*cli.Command{
					{
						Name: "generate",
						Flags: []cli.Flag{
							&cli.StringSliceFlag{Name: "meta", Aliases: []string{"m"}},
							&cli.BoolFlag{Name: flags.DryRunFlag},
							&cli.BoolFlag{Name: flags.NoHooksFlag},
							&cli.StringFlag{Name: flags.OnExistingFlag, Value: ""},
							&cli.BoolFlag{Name: flags.VerboseFlag},
						},
						Action: func(c *cli.Context) error {
							return generateAction(c, cfg, defaultGeneratorFactories())
						},
					},
				},
			}

			err = cliApp.Run(tt.args)
			require.NoError(t, err)

			output, err := os.ReadFile(filepath.Join(tmpDir, "widget.txt"))
			require.NoError(t, err, "expected widget.txt to be created")

			content := string(output)
			assert.Contains(t, content, "name: widget")

			if tt.wantMeta {
				assert.Contains(t, content, "fields: name:string")
				assert.Contains(t, content, "domain: tenant")
			} else {
				// Documents the bug: flags after args are swallowed as extra args.
				assert.NotContains(t, content, "fields: name:string")
				assert.NotContains(t, content, "domain: tenant")
			}
		})
	}
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

	fac := defaultGeneratorFactories()
	fac.newEngine = func(dryRun bool, dirPath string, sharedPath string, rec *history.Recorder, _ io.Writer) (engine.Generator, error) {
		mock := &mockGenerator{
			GenerateFunc: func(data engine.Data) (engine.GenerateResult, error) {
				return engine.GenerateResult{}, errors.New("template execution failed")
			},
		}
		return mock, nil
	}

	err := generateAction(ctx, cfg, fac)

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

	fac := defaultGeneratorFactories()
	fac.newEngine = func(dryRun bool, dirPath string, sharedPath string, rec *history.Recorder, _ io.Writer) (engine.Generator, error) {
		return nil, errors.New("failed to create engine")
	}

	err := generateAction(ctx, cfg, fac)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error creating engine")
}

func TestUT_GenerateBundle_BundleNotFound(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"nonexistent", "myName"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.BundlePathFlag: "_bundles",
	})

	err := generateAction(ctx, cfg, defaultGeneratorFactories())

	require.Error(t, err)
	assert.Contains(t, err.Error(), `generator "nonexistent" not found`)
}

func TestUT_GenerateBundle_InvalidJSON(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	// Create invalid JSON bundle
	createBundle(t, tmpDir, "mybundle", "not valid json")

	ctx := createTestCLIContext(t, []string{"mybundle", "myName"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.BundlePathFlag: "_bundles",
	})

	err := generateAction(ctx, cfg, defaultGeneratorFactories())

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
	})

	var generateCalls int
	fac := defaultGeneratorFactories()
	fac.newBundleEngine = func(tmplEngine *template.Engine, dryRun bool, dirPath string, sharedPath string, rec *history.Recorder, _ io.Writer) (engine.Generator, error) {
		mock := &mockGenerator{
			GenerateFunc: func(data engine.Data) (engine.GenerateResult, error) {
				generateCalls++
				return engine.GenerateResult{}, nil
			},
		}
		return mock, nil
	}

	err := generateAction(ctx, cfg, fac)

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
	})

	var generateCalls int
	fac := defaultGeneratorFactories()
	fac.newBundleEngine = func(tmplEngine *template.Engine, dryRun bool, dirPath string, sharedPath string, rec *history.Recorder, _ io.Writer) (engine.Generator, error) {
		mock := &mockGenerator{
			GenerateFunc: func(data engine.Data) (engine.GenerateResult, error) {
				generateCalls++
				return engine.GenerateResult{}, nil
			},
		}
		return mock, nil
	}

	err := generateAction(ctx, cfg, fac)

	require.NoError(t, err)
	assert.Equal(t, 2, generateCalls, "expected Generate to be called twice for two generators in bundle")
}

func TestUT_RunHooks_EmptyHooks(t *testing.T) {
	err := runHooks([][]string{}, hooks.HookPhasePreGen, nil, io.Discard, "testgen", "testtarget")
	require.NoError(t, err)
}

func TestUT_RunHooks_NilHooks(t *testing.T) {
	err := runHooks(nil, hooks.HookPhasePreGen, nil, io.Discard, "testgen", "testtarget")
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
	err = runHooks([][]string{{"./test-hook.sh"}}, hooks.HookPhasePreGen, nil, io.Discard, "testgen", "testtarget")
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
	err = runHooks([][]string{{"./failing-hook.sh"}}, hooks.HookPhasePreGen, nil, io.Discard, "testgen", "testtarget")
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
	err = runHooks([][]string{{"./hook1.sh"}, {"./hook2.sh"}}, hooks.HookPhasePreGen, nil, io.Discard, "testgen", "testtarget")
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

	err = runHooks([][]string{{"nonexistent-command-xyz"}}, hooks.HookPhasePreGen, nil, io.Discard, "testgen", "testtarget")
	require.Error(t, err)
}

func TestUT_RunHooks_PathCommand(t *testing.T) {
	// Test that PATH commands work (e.g., "echo" should be found in PATH)
	err := runHooks([][]string{{"echo", "hello"}}, hooks.HookPhasePreGen, nil, io.Discard, "testgen", "testtarget")
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
	err = runHooks([][]string{{"./myscript.sh"}}, hooks.HookPhasePreGen, nil, io.Discard, "testgen", "testtarget")
	require.NoError(t, err)
}

// --- Self-contained bundle tests ---

func TestUT_GenerateBundle_SelfContained_Valid(t *testing.T) {
	tmpDir := setupTempDir(t)

	templateContent := `---
to: output/{{ .Name }}.txt
---
Hello {{ .Name }}`

	// Create generators inside the bundle directory (self-contained)
	bundleDir := filepath.Join(tmpDir, "_bundles", "mybundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0o750))

	// Create generator inside bundle dir
	genDir := filepath.Join(bundleDir, "hello")
	require.NoError(t, os.MkdirAll(genDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "hello.go"), []byte(templateContent), 0o644))

	// Create bundle JSON with self_contained: true
	bundleJSON := `{"name":"mybundle","self_contained":true,"generators":[{"name":"hello"}]}`
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "mybundle.json"), []byte(bundleJSON), 0o644))

	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"mybundle", "world"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.BundlePathFlag: "_bundles",
	})

	var capturedDirPath, capturedSharedPath string
	fac := defaultGeneratorFactories()
	fac.newBundleEngine = func(tmplEngine *template.Engine, dryRun bool, dirPath string, sharedPath string, rec *history.Recorder, _ io.Writer) (engine.Generator, error) {
		capturedDirPath = dirPath
		capturedSharedPath = sharedPath
		mock := &mockGenerator{
			GenerateFunc: func(data engine.Data) (engine.GenerateResult, error) {
				return engine.GenerateResult{}, nil
			},
		}
		return mock, nil
	}

	err := generateAction(ctx, cfg, fac)

	require.NoError(t, err)
	// Verify generator was resolved from bundle dir, not root .tag/
	assert.Equal(t, filepath.Join(bundleDir, "hello"), capturedDirPath)
	assert.Equal(t, filepath.Join(bundleDir, "_shared"), capturedSharedPath)
}

func TestUT_GenerateBundle_SelfContained_GeneratorNotFound(t *testing.T) {
	tmpDir := setupTempDir(t)

	// Create bundle dir with no generators inside
	bundleDir := filepath.Join(tmpDir, "_bundles", "mybundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0o750))

	// Create bundle JSON referencing a generator that doesn't exist in bundle dir
	bundleJSON := `{"name":"mybundle","self_contained":true,"generators":[{"name":"missing"}]}`
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "mybundle.json"), []byte(bundleJSON), 0o644))

	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"mybundle", "world"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.BundlePathFlag: "_bundles",
	})

	err := generateAction(ctx, cfg, defaultGeneratorFactories())

	require.Error(t, err)
	assert.Contains(t, err.Error(), `generator "missing" not found in self-contained bundle "mybundle"`)
}

func TestUT_GenerateBundle_SelfContained_UsesOwnShared(t *testing.T) {
	tmpDir := setupTempDir(t)

	templateContent := `---
to: output/{{ .Name }}.txt
---
Hello {{ .Name }}`

	// Create bundle with its own _shared directory
	bundleDir := filepath.Join(tmpDir, "_bundles", "mybundle")
	bundleShared := filepath.Join(bundleDir, "_shared")
	require.NoError(t, os.MkdirAll(bundleShared, 0o750))

	// Create generator inside bundle
	genDir := filepath.Join(bundleDir, "hello")
	require.NoError(t, os.MkdirAll(genDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "hello.go"), []byte(templateContent), 0o644))

	// Also create root _shared (should NOT be used)
	rootShared := filepath.Join(tmpDir, "_shared")
	require.NoError(t, os.MkdirAll(rootShared, 0o750))

	bundleJSON := `{"name":"mybundle","self_contained":true,"generators":[{"name":"hello"}]}`
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "mybundle.json"), []byte(bundleJSON), 0o644))

	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"mybundle", "world"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.BundlePathFlag: "_bundles",
	})

	var capturedSharedPath string
	fac := defaultGeneratorFactories()
	fac.newBundleEngine = func(tmplEngine *template.Engine, dryRun bool, dirPath string, sharedPath string, rec *history.Recorder, _ io.Writer) (engine.Generator, error) {
		capturedSharedPath = sharedPath
		mock := &mockGenerator{
			GenerateFunc: func(data engine.Data) (engine.GenerateResult, error) {
				return engine.GenerateResult{}, nil
			},
		}
		return mock, nil
	}

	err := generateAction(ctx, cfg, fac)

	require.NoError(t, err)
	// Should use bundle's own _shared, not root _shared
	assert.Equal(t, bundleShared, capturedSharedPath)
	assert.NotEqual(t, rootShared, capturedSharedPath)
}

func TestUT_GenerateBundle_SelfContained_PathTraversal(t *testing.T) {
	tmpDir := setupTempDir(t)

	// Create bundle with a generator name containing path traversal
	bundleDir := filepath.Join(tmpDir, "_bundles", "evil")
	require.NoError(t, os.MkdirAll(bundleDir, 0o750))

	bundleJSON := `{"name":"evil","self_contained":true,"generators":[{"name":"../../etc"}]}`
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "evil.json"), []byte(bundleJSON), 0o644))

	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"evil", "target"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.BundlePathFlag: "_bundles",
	})

	err := generateAction(ctx, cfg, defaultGeneratorFactories())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid generator name in bundle")
}

// --- resolveGeneratorPaths tests ---

func TestUT_ResolveGeneratorPaths_LocalFallback(t *testing.T) {
	tmpDir := setupTempDir(t)

	// Create a local generator
	genDir := filepath.Join(tmpDir, "myGen")
	require.NoError(t, os.MkdirAll(genDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "gen.tmpl"), []byte("content"), 0o644))

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
	fac := defaultGeneratorFactories()
	fac.newEngine = func(dryRun bool, dirPath string, sharedPath string, rec *history.Recorder, _ io.Writer) (engine.Generator, error) {
		mock := &mockGenerator{
			GenerateFunc: func(data engine.Data) (engine.GenerateResult, error) {
				capturedData = data
				return engine.GenerateResult{}, nil
			},
		}
		return mock, nil
	}

	err := generateAction(ctx, cfg, fac)

	require.NoError(t, err)
	assert.Equal(t, "world", capturedData.Name)
	assert.Equal(t, "my-project", capturedData.ScaffoldVars["project_name"])
	assert.Equal(t, true, capturedData.ScaffoldVars["use_docker"])
}

func TestUT_GenerateAction_OnExisting_InvalidValue(t *testing.T) {
	tmpDir := setupTempDir(t)
	createGenerator(t, tmpDir, "hello", "---\nto: output.txt\n---\ncontent")
	createSharedDir(t, tmpDir)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"hello", "world"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.OnExistingFlag: "badvalue",
	})

	err := generateAction(ctx, cfg, defaultGeneratorFactories())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --on-existing value")
	assert.Contains(t, err.Error(), "badvalue")
}

func TestUT_GenerateAction_OnExisting_PassedToEngine(t *testing.T) {
	tmpDir := setupTempDir(t)
	createGenerator(t, tmpDir, "hello", "---\nto: output.txt\n---\ncontent")
	createSharedDir(t, tmpDir)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"hello", "world"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.OnExistingFlag: "skip",
	})

	var capturedData engine.Data
	fac := defaultGeneratorFactories()
	fac.newEngine = func(dryRun bool, dirPath string, sharedPath string, rec *history.Recorder, _ io.Writer) (engine.Generator, error) {
		mock := &mockGenerator{
			GenerateFunc: func(data engine.Data) (engine.GenerateResult, error) {
				capturedData = data
				return engine.GenerateResult{}, nil
			},
		}
		return mock, nil
	}

	err := generateAction(ctx, cfg, fac)

	require.NoError(t, err)
	assert.Equal(t, engine.OnExistingSkip, capturedData.OnExisting)
}

func TestUT_GenerateAction_ConflictError_ReturnsError(t *testing.T) {
	tmpDir := setupTempDir(t)
	createGenerator(t, tmpDir, "hello", "---\nto: output.txt\n---\ncontent")
	createSharedDir(t, tmpDir)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"hello", "world"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
	})

	fac := defaultGeneratorFactories()
	fac.newEngine = func(dryRun bool, dirPath string, sharedPath string, rec *history.Recorder, _ io.Writer) (engine.Generator, error) {
		mock := &mockGenerator{
			GenerateFunc: func(data engine.Data) (engine.GenerateResult, error) {
				return engine.GenerateResult{}, &engine.ConflictError{
					Files: []string{"output.txt", "other.go"},
				}
			},
		}
		return mock, nil
	}

	err := generateAction(ctx, cfg, fac)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "output.txt")
}

func TestUT_PrintGenerateSummary_NoVerbose(t *testing.T) {
	result := engine.GenerateResult{
		Created:     2,
		Skipped:     1,
		Overwritten: 3,
		Modified:    0,
		Details: []engine.FileOpDetail{
			{Path: "a.go", Action: fileaction.ActionCreate},
			{Path: "b.go", Action: fileaction.ActionCreate},
			{Path: "c.go", Action: fileaction.ActionSkip},
		},
	}

	var buf bytes.Buffer
	printGenerateSummary(&buf, result, false)

	out := buf.String()
	assert.Contains(t, out, "Generated: 2 created, 1 skipped, 3 overwritten, 0 modified")
	assert.NotContains(t, out, "a.go", "per-file details should not be shown without --verbose")
}

func TestUT_PrintGenerateSummary_Verbose(t *testing.T) {
	result := engine.GenerateResult{
		Created:  1,
		Modified: 1,
		Details: []engine.FileOpDetail{
			{Path: "a.go", Action: fileaction.ActionCreate},
			{Path: "b.go", Action: fileaction.ActionAppend},
		},
	}

	var buf bytes.Buffer
	printGenerateSummary(&buf, result, true)

	out := buf.String()
	assert.Contains(t, out, "a.go")
	assert.Contains(t, out, "created")
	assert.Contains(t, out, "b.go")
	assert.Contains(t, out, "modified")
	assert.Contains(t, out, "Generated: 1 created, 0 skipped, 0 overwritten, 1 modified")
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
	fac := defaultGeneratorFactories()
	fac.newEngine = func(dryRun bool, dirPath string, sharedPath string, rec *history.Recorder, _ io.Writer) (engine.Generator, error) {
		mock := &mockGenerator{
			GenerateFunc: func(data engine.Data) (engine.GenerateResult, error) {
				capturedData = data
				return engine.GenerateResult{}, nil
			},
		}
		return mock, nil
	}

	err := generateAction(ctx, cfg, fac)

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
	})

	var capturedData engine.Data
	fac := defaultGeneratorFactories()
	fac.newBundleEngine = func(tmplEngine *template.Engine, dryRun bool, dirPath string, sharedPath string, rec *history.Recorder, _ io.Writer) (engine.Generator, error) {
		mock := &mockGenerator{
			GenerateFunc: func(data engine.Data) (engine.GenerateResult, error) {
				capturedData = data
				return engine.GenerateResult{}, nil
			},
		}
		return mock, nil
	}

	err := generateAction(ctx, cfg, fac)

	require.NoError(t, err)
	assert.Equal(t, "my-project", capturedData.ScaffoldVars["project_name"])
}

// --- generateList tests ---

func TestUT_GenerateList_NoGenerators(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	var buf bytes.Buffer
	err := generateList(cfg, true, &buf, formatText)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No generators found.")
}

func TestUT_GenerateList_LocalGenerators(t *testing.T) {
	tmpDir := setupTempDir(t)

	// Create local generators
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "component"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "component", "gen.tmpl"), []byte("content"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "page"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "page", "gen.tmpl"), []byte("content"), 0o644))
	// Create _shared (should be excluded from listing)
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "_shared"), 0o750))

	cfg := createTestConfig(t, tmpDir)

	var buf bytes.Buffer
	err := generateList(cfg, true, &buf, formatText)

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
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "gen.tmpl"), []byte("content"), 0o644))

	cfg := createTestConfig(t, tmpDir)

	var buf bytes.Buffer
	err := generateList(cfg, true, &buf, formatText)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Create a React component")
}

func TestUT_GenerateList_WithBundles(t *testing.T) {
	tmpDir := setupTempDir(t)

	// Create a generator
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "component"), 0o750))

	// Create a bundle
	createBundle(t, tmpDir, "feature", `{"name":"feature","generators":[{"name":"component"}]}`)

	cfg := createTestConfig(t, tmpDir)

	err := generateList(cfg, true, io.Discard, formatText)

	require.NoError(t, err)
}

func TestUT_GenerateList_BundleDescriptionShown(t *testing.T) {
	tmpDir := setupTempDir(t)

	// Create a generator so the list is non-empty.
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "component"), 0o750))

	// Create a bundle with a description field.
	createBundle(t, tmpDir, "feature", `{"name":"feature","description":"Scaffolds a full feature","generators":[{"name":"component"}]}`)

	cfg := createTestConfig(t, tmpDir)

	var buf bytes.Buffer
	err := generateList(cfg, true, &buf, formatText)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Scaffolds a full feature")
}

func TestUT_GenerateList_NoConfig(t *testing.T) {
	err := generateList(nil, true, io.Discard, formatText)

	require.Error(t, err)
}

func TestUT_ScanGenerators_SkipsReserved(t *testing.T) {
	tmpDir := setupTempDir(t)

	// Create generators and reserved dirs
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "component"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "component", "gen.tmpl"), []byte("content"), 0o644))
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

func TestUT_ScanGenerators_FrontmatterDescFallback(t *testing.T) {
	tmpDir := setupTempDir(t)

	// Create a generator with a template file containing desc in frontmatter
	genDir := filepath.Join(tmpDir, "service")
	require.NoError(t, os.MkdirAll(genDir, 0o750))

	tmplContent := "---\nto: internal/{{ name | snake }}.go\ndesc: Generate a service layer\n---\npackage services\n"
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "service.go"), []byte(tmplContent), 0o644))

	result := scanGenerators(tmpDir)

	require.Len(t, result, 1)
	assert.Equal(t, "service", result[0].Name)
	assert.Equal(t, "Generate a service layer", result[0].Description)
}

func TestUT_ScanGenerators_TagTemplateJSONTakesPrecedence(t *testing.T) {
	tmpDir := setupTempDir(t)

	// Create a generator with both tag.template.json and frontmatter desc
	genDir := filepath.Join(tmpDir, "handler")
	require.NoError(t, os.MkdirAll(genDir, 0o750))

	configJSON := `{"description": "From config"}`
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "tag.template.json"), []byte(configJSON), 0o644))

	tmplContent := "---\nto: internal/{{ name | snake }}.go\ndesc: From frontmatter\n---\npackage handler\n"
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "handler.go"), []byte(tmplContent), 0o644))

	result := scanGenerators(tmpDir)

	require.Len(t, result, 1)
	assert.Equal(t, "handler", result[0].Name)
	assert.Equal(t, "From config", result[0].Description, "tag.template.json should take precedence")
}

func TestUT_ScanGenerators_NoDescriptionAnywhere(t *testing.T) {
	tmpDir := setupTempDir(t)

	// Create a generator with a template file but no desc
	genDir := filepath.Join(tmpDir, "model")
	require.NoError(t, os.MkdirAll(genDir, 0o750))

	tmplContent := "---\nto: internal/{{ name | snake }}.go\n---\npackage models\n"
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "model.go"), []byte(tmplContent), 0o644))

	result := scanGenerators(tmpDir)

	require.Len(t, result, 1)
	assert.Equal(t, "model", result[0].Name)
	assert.Empty(t, result[0].Description)
}

func TestUT_GenerateList_FrontmatterDescShown(t *testing.T) {
	tmpDir := setupTempDir(t)

	genDir := filepath.Join(tmpDir, "service")
	require.NoError(t, os.MkdirAll(genDir, 0o750))

	tmplContent := "---\nto: internal/{{ name | snake }}.go\ndesc: Generate a service layer\n---\npackage services\n"
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "service.go"), []byte(tmplContent), 0o644))

	cfg := createTestConfig(t, tmpDir)

	var buf bytes.Buffer
	err := generateList(cfg, true, &buf, formatText)

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "service")
	assert.Contains(t, output, "Generate a service layer")
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
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "gen.tmpl"), []byte("content"), 0o644))

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
	require.NoError(t, os.WriteFile(filepath.Join(localGenDir, "gen.tmpl"), []byte("content"), 0o644))

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

	// Capture slog output
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	old := slog.Default()
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(old)

	warnVersionMismatch(cfg, templateDir)

	output := buf.String()
	assert.Contains(t, output, "template version mismatch")
	assert.Contains(t, output, "1.0.0")
	assert.Contains(t, output, "2.0.0")
}

func TestUT_WarnVersionMismatch_NoWarningWhenMatch(t *testing.T) {
	templateDir := setupFakeLibrary(t, "my-template")

	configJSON := `{"version": "1.0.0"}`
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "tag.template.json"), []byte(configJSON), 0o644))

	cfg := createTestConfigWithLib(t, t.TempDir(), "my-template")
	cfg.Template.Version = "1.0.0"

	// Capture slog output
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	old := slog.Default()
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(old)

	warnVersionMismatch(cfg, templateDir)

	assert.Empty(t, buf.String(), "no warning expected when versions match")
}

func TestUT_WarnVersionMismatch_SkipsWhenNoVersion(t *testing.T) {
	templateDir := setupFakeLibrary(t, "my-template")

	cfg := createTestConfigWithLib(t, t.TempDir(), "my-template")
	cfg.Template.Version = "" // no version recorded at scaffold time

	// Capture slog output
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	old := slog.Default()
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(old)

	warnVersionMismatch(cfg, templateDir)

	assert.Empty(t, buf.String(), "no warning expected when scaffold version is empty")
}

// --- Output capture tests (validate c.App.Writer injection from M1 refactoring) ---

func TestUT_GenerateTemplate_SummaryGoesToAppWriter(t *testing.T) {
	tmpDir := setupTempDir(t)

	templateContent := `---
to: output/{{ .Name }}.txt
---
Hello {{ .Name }}`

	createGenerator(t, tmpDir, "greet", templateContent)
	createSharedDir(t, tmpDir)
	cfg := createTestConfig(t, tmpDir)

	var buf bytes.Buffer
	cliApp := &cli.App{
		Writer: &buf,
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: flags.DryRunFlag},
			&cli.StringFlag{Name: flags.PathFlag, Value: ".tag"},
			&cli.StringFlag{Name: flags.SharedPathFlag, Value: "_shared"},
			&cli.BoolFlag{Name: flags.NoHooksFlag},
			&cli.BoolFlag{Name: flags.VerboseFlag},
			&cli.StringSliceFlag{Name: flags.MetaFlag},
			&cli.StringFlag{Name: flags.OnExistingFlag, Value: ""},
		},
	}

	set := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range cliApp.Flags {
		require.NoError(t, f.Apply(set))
	}
	require.NoError(t, set.Set(flags.PathFlag, tmpDir))
	require.NoError(t, set.Parse([]string{"greet", "world"}))
	ctx := cli.NewContext(cliApp, set, nil)

	fac := defaultGeneratorFactories()
	fac.newEngine = func(dryRun bool, dirPath string, sharedPath string, rec *history.Recorder, _ io.Writer) (engine.Generator, error) {
		return &mockGenerator{
			GenerateFunc: func(data engine.Data) (engine.GenerateResult, error) {
				return engine.GenerateResult{Created: 2, Skipped: 1}, nil
			},
		}, nil
	}

	err := generateAction(ctx, cfg, fac)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Generated:", "summary should be written to App.Writer")
	assert.Contains(t, out, "2 created", "created count should be in output")
	assert.Contains(t, out, "1 skipped", "skipped count should be in output")
}

func TestUT_PrintGenerateSummary_ZeroResults(t *testing.T) {
	var buf bytes.Buffer
	printGenerateSummary(&buf, engine.GenerateResult{}, false)
	assert.Equal(t, "Generated: 0 created, 0 skipped, 0 overwritten, 0 modified\n", buf.String())
}

func TestUT_PrintGenerateSummary_SummaryLineFormat(t *testing.T) {
	result := engine.GenerateResult{Created: 3, Skipped: 1, Overwritten: 2, Modified: 4}
	var buf bytes.Buffer
	printGenerateSummary(&buf, result, false)
	assert.Equal(t, "Generated: 3 created, 1 skipped, 2 overwritten, 4 modified\n", buf.String())
}

// --- Prerequisites tests ---

func TestUT_GenerateBundle_RequirementsMet(t *testing.T) {
	tmpDir := setupTempDir(t)

	templateContent := `---
to: output/{{ .Name }}.txt
---
Hello {{ .Name }}`

	createGenerator(t, tmpDir, "hello", templateContent)
	createSharedDir(t, tmpDir)

	bundleJSON := `{"name":"mybundle","requires":["use_postgres"],"generators":[{"name":"hello"}]}`
	createBundle(t, tmpDir, "mybundle", bundleJSON)

	cfg := createTestConfig(t, tmpDir)
	cfg.Variables = map[string]any{
		"use_postgres": true,
	}

	ctx := createTestCLIContext(t, []string{"mybundle", "world"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.BundlePathFlag: "_bundles",
	})

	var generateCalls int
	fac := defaultGeneratorFactories()
	fac.newBundleEngine = func(tmplEngine *template.Engine, dryRun bool, dirPath string, sharedPath string, rec *history.Recorder, _ io.Writer) (engine.Generator, error) {
		mock := &mockGenerator{
			GenerateFunc: func(data engine.Data) (engine.GenerateResult, error) {
				generateCalls++
				return engine.GenerateResult{}, nil
			},
		}
		return mock, nil
	}

	err := generateAction(ctx, cfg, fac)

	require.NoError(t, err)
	assert.Equal(t, 1, generateCalls, "generator should run when requirements are met")
}

func TestUT_GenerateBundle_RequirementsUnmet(t *testing.T) {
	tmpDir := setupTempDir(t)

	templateContent := `---
to: output/{{ .Name }}.txt
---
Hello {{ .Name }}`

	createGenerator(t, tmpDir, "hello", templateContent)
	createSharedDir(t, tmpDir)

	bundleJSON := `{"name":"mybundle","requires":["use_postgres"],"generators":[{"name":"hello"}]}`
	createBundle(t, tmpDir, "mybundle", bundleJSON)

	cfg := createTestConfig(t, tmpDir)
	cfg.Variables = map[string]any{
		"use_postgres": false,
	}

	ctx := createTestCLIContext(t, []string{"mybundle", "world"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.BundlePathFlag: "_bundles",
	})

	err := generateAction(ctx, cfg, defaultGeneratorFactories())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "use_postgres")
}

func TestUT_GenerateBundle_RequirementsPartiallyMet(t *testing.T) {
	tmpDir := setupTempDir(t)

	templateContent := `---
to: output/{{ .Name }}.txt
---
Hello {{ .Name }}`

	createGenerator(t, tmpDir, "hello", templateContent)
	createSharedDir(t, tmpDir)

	bundleJSON := `{"name":"mybundle","requires":["use_postgres","use_amqp"],"generators":[{"name":"hello"}]}`
	createBundle(t, tmpDir, "mybundle", bundleJSON)

	cfg := createTestConfig(t, tmpDir)
	cfg.Variables = map[string]any{
		"use_postgres": true,
		"use_amqp":     false,
	}

	ctx := createTestCLIContext(t, []string{"mybundle", "world"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.BundlePathFlag: "_bundles",
	})

	err := generateAction(ctx, cfg, defaultGeneratorFactories())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "use_amqp")
	assert.NotContains(t, err.Error(), "use_postgres")
}

func TestUT_GenerateBundle_NoRequirements_BackwardCompat(t *testing.T) {
	tmpDir := setupTempDir(t)

	templateContent := `---
to: output/{{ .Name }}.txt
---
Hello {{ .Name }}`

	createGenerator(t, tmpDir, "hello", templateContent)
	createSharedDir(t, tmpDir)

	// Bundle without "requires" field — should work unchanged.
	bundleJSON := `{"name":"mybundle","generators":[{"name":"hello"}]}`
	createBundle(t, tmpDir, "mybundle", bundleJSON)

	cfg := createTestConfig(t, tmpDir)
	cfg.Variables = map[string]any{
		"use_postgres": false,
	}

	ctx := createTestCLIContext(t, []string{"mybundle", "world"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.BundlePathFlag: "_bundles",
	})

	var generateCalls int
	fac := defaultGeneratorFactories()
	fac.newBundleEngine = func(tmplEngine *template.Engine, dryRun bool, dirPath string, sharedPath string, rec *history.Recorder, _ io.Writer) (engine.Generator, error) {
		mock := &mockGenerator{
			GenerateFunc: func(data engine.Data) (engine.GenerateResult, error) {
				generateCalls++
				return engine.GenerateResult{}, nil
			},
		}
		return mock, nil
	}

	err := generateAction(ctx, cfg, fac)

	require.NoError(t, err)
	assert.Equal(t, 1, generateCalls, "generator should run when no requirements are specified")
}

func TestUT_GenerateBundle_RequirementsMissing_NoConfig(t *testing.T) {
	tmpDir := setupTempDir(t)

	templateContent := `---
to: output/{{ .Name }}.txt
---
Hello {{ .Name }}`

	createGenerator(t, tmpDir, "hello", templateContent)
	createSharedDir(t, tmpDir)

	bundleJSON := `{"name":"mybundle","requires":["use_postgres"],"generators":[{"name":"hello"}]}`
	createBundle(t, tmpDir, "mybundle", bundleJSON)

	cfg := createTestConfig(t, tmpDir)
	// No variables set at all.

	ctx := createTestCLIContext(t, []string{"mybundle", "world"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.BundlePathFlag: "_bundles",
	})

	err := generateAction(ctx, cfg, defaultGeneratorFactories())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "use_postgres")
	assert.Contains(t, err.Error(), "not set")
}

func TestUT_GenerateList_FiltersUnmetRequirements(t *testing.T) {
	tmpDir := setupTempDir(t)

	createGenerator(t, tmpDir, "domain", `---
to: output/{{ .Name }}.txt
desc: Domain generator
---
Hello`)

	// Create a bundle with requirements
	bundleJSON := `{"name":"crud","description":"CRUD bundle","requires":["use_postgres"],"generators":[{"name":"domain"}]}`
	createBundle(t, tmpDir, "crud", bundleJSON)

	// Create a bundle without requirements
	bundleJSON2 := `{"name":"bdd","description":"BDD bundle","generators":[{"name":"domain"}]}`
	createBundle(t, tmpDir, "bdd", bundleJSON2)

	cfg := createTestConfig(t, tmpDir)
	cfg.Variables = map[string]any{
		"use_postgres": false,
	}

	var buf bytes.Buffer
	err := generateList(cfg, false, &buf, formatText)

	require.NoError(t, err)
	output := buf.String()
	assert.NotContains(t, output, "crud", "bundle with unmet requirements should be filtered out")
	assert.Contains(t, output, "bdd", "bundle without requirements should be shown")
}

func TestUT_GenerateList_AllFlagShowsEverything(t *testing.T) {
	tmpDir := setupTempDir(t)

	createGenerator(t, tmpDir, "domain", `---
to: output/{{ .Name }}.txt
desc: Domain generator
---
Hello`)

	bundleJSON := `{"name":"crud","description":"CRUD bundle","requires":["use_postgres"],"generators":[{"name":"domain"}]}`
	createBundle(t, tmpDir, "crud", bundleJSON)

	cfg := createTestConfig(t, tmpDir)
	cfg.Variables = map[string]any{
		"use_postgres": false,
	}

	var buf bytes.Buffer
	err := generateList(cfg, true, &buf, formatText)

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "crud", "bundle should be shown with --all flag even if requirements unmet")
	assert.Contains(t, output, "[requires: use_postgres]", "requirements should be displayed")
}

func TestUT_GenerateList_RequirementsShownInOutput(t *testing.T) {
	tmpDir := setupTempDir(t)

	createGenerator(t, tmpDir, "domain", `---
to: output/{{ .Name }}.txt
---
Hello`)

	bundleJSON := `{"name":"crud","description":"CRUD bundle","requires":["use_postgres","use_amqp"],"generators":[{"name":"domain"}]}`
	createBundle(t, tmpDir, "crud", bundleJSON)

	cfg := createTestConfig(t, tmpDir)
	cfg.Variables = map[string]any{
		"use_postgres": true,
		"use_amqp":     true,
	}

	var buf bytes.Buffer
	err := generateList(cfg, false, &buf, formatText)

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "[requires: use_postgres, use_amqp]")
}

func TestUT_GenerateTemplate_RequirementsMet(t *testing.T) {
	tmpDir := setupTempDir(t)

	templateContent := `---
to: output/{{ .Name }}.txt
---
Hello {{ .Name }}`

	createGenerator(t, tmpDir, "postgres-adapter", templateContent)
	createSharedDir(t, tmpDir)

	// Write a tag.template.json with requires into the generator directory.
	genConfigPath := filepath.Join(tmpDir, "postgres-adapter", "tag.template.json")
	require.NoError(t, os.WriteFile(genConfigPath, []byte(`{"requires":["use_postgres"]}`), 0o644))

	cfg := createTestConfig(t, tmpDir)
	cfg.Variables = map[string]any{
		"use_postgres": true,
	}

	ctx := createTestCLIContext(t, []string{"postgres-adapter", "orders"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
	})

	var generateCalls int
	fac := defaultGeneratorFactories()
	fac.newEngine = func(dryRun bool, dirPath string, sharedPath string, rec *history.Recorder, _ io.Writer) (engine.Generator, error) {
		mock := &mockGenerator{
			GenerateFunc: func(data engine.Data) (engine.GenerateResult, error) {
				generateCalls++
				return engine.GenerateResult{}, nil
			},
		}
		return mock, nil
	}

	err := generateAction(ctx, cfg, fac)

	require.NoError(t, err)
	assert.Equal(t, 1, generateCalls, "generator should run when requirements are met")
}

func TestUT_GenerateTemplate_RequirementsUnmet(t *testing.T) {
	tmpDir := setupTempDir(t)

	templateContent := `---
to: output/{{ .Name }}.txt
---
Hello {{ .Name }}`

	createGenerator(t, tmpDir, "postgres-adapter", templateContent)
	createSharedDir(t, tmpDir)

	// Write a tag.template.json with requires into the generator directory.
	genConfigPath := filepath.Join(tmpDir, "postgres-adapter", "tag.template.json")
	require.NoError(t, os.WriteFile(genConfigPath, []byte(`{"requires":["use_postgres"]}`), 0o644))

	cfg := createTestConfig(t, tmpDir)
	cfg.Variables = map[string]any{
		"use_postgres": false,
	}

	ctx := createTestCLIContext(t, []string{"postgres-adapter", "orders"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
	})

	err := generateAction(ctx, cfg, defaultGeneratorFactories())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "use_postgres")
	assert.Contains(t, err.Error(), "generator")
}

func TestUT_GenerateTemplate_NoRequirements_BackwardCompat(t *testing.T) {
	tmpDir := setupTempDir(t)

	templateContent := `---
to: output/{{ .Name }}.txt
---
Hello {{ .Name }}`

	createGenerator(t, tmpDir, "hello", templateContent)
	createSharedDir(t, tmpDir)

	// Generator without tag.template.json — should work unchanged.
	cfg := createTestConfig(t, tmpDir)
	cfg.Variables = map[string]any{
		"use_postgres": false,
	}

	ctx := createTestCLIContext(t, []string{"hello", "world"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
	})

	var generateCalls int
	fac := defaultGeneratorFactories()
	fac.newEngine = func(dryRun bool, dirPath string, sharedPath string, rec *history.Recorder, _ io.Writer) (engine.Generator, error) {
		mock := &mockGenerator{
			GenerateFunc: func(data engine.Data) (engine.GenerateResult, error) {
				generateCalls++
				return engine.GenerateResult{}, nil
			},
		}
		return mock, nil
	}

	err := generateAction(ctx, cfg, fac)

	require.NoError(t, err)
	assert.Equal(t, 1, generateCalls, "generator should run when no tag.template.json exists")
}

// --- Enhancement 1: Bundle Variables tests ---

func TestUT_GenerateBundle_BundleVarsMerged(t *testing.T) {
	tmpDir := setupTempDir(t)

	templateContent := "---\nto: output/{{ .Name }}.txt\n---\nHello {{ .Name }}"
	createGenerator(t, tmpDir, "hello", templateContent)
	createSharedDir(t, tmpDir)

	bundleJSON := `{"name":"mybundle","vars":{"domain":"tenant","use_db":true},"generators":[{"name":"hello"}]}`
	createBundle(t, tmpDir, "mybundle", bundleJSON)

	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"mybundle", "world"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.BundlePathFlag: "_bundles",
	})

	var capturedData engine.Data
	fac := defaultGeneratorFactories()
	fac.newBundleEngine = func(_ *template.Engine, _ bool, _ string, _ string, _ *history.Recorder, _ io.Writer) (engine.Generator, error) {
		return &mockGenerator{
			GenerateFunc: func(data engine.Data) (engine.GenerateResult, error) {
				capturedData = data
				return engine.GenerateResult{}, nil
			},
		}, nil
	}

	err := generateAction(ctx, cfg, fac)

	require.NoError(t, err)
	assert.Equal(t, "tenant", capturedData.ScaffoldVars["domain"])
	assert.Equal(t, true, capturedData.ScaffoldVars["use_db"])
}

func TestUT_GenerateBundle_BundleVarsOverrideScaffoldVars(t *testing.T) {
	tmpDir := setupTempDir(t)

	templateContent := "---\nto: output/{{ .Name }}.txt\n---\nHello"
	createGenerator(t, tmpDir, "hello", templateContent)
	createSharedDir(t, tmpDir)

	bundleJSON := `{"name":"mybundle","vars":{"domain":"admin"},"generators":[{"name":"hello"}]}`
	createBundle(t, tmpDir, "mybundle", bundleJSON)

	cfg := createTestConfig(t, tmpDir)
	cfg.Variables = map[string]any{
		"domain":       "default",
		"project_name": "myproj",
	}

	ctx := createTestCLIContext(t, []string{"mybundle", "world"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.BundlePathFlag: "_bundles",
	})

	var capturedData engine.Data
	fac := defaultGeneratorFactories()
	fac.newBundleEngine = func(_ *template.Engine, _ bool, _ string, _ string, _ *history.Recorder, _ io.Writer) (engine.Generator, error) {
		return &mockGenerator{
			GenerateFunc: func(data engine.Data) (engine.GenerateResult, error) {
				capturedData = data
				return engine.GenerateResult{}, nil
			},
		}, nil
	}

	err := generateAction(ctx, cfg, fac)

	require.NoError(t, err)
	assert.Equal(t, "admin", capturedData.ScaffoldVars["domain"], "bundle vars should override scaffold vars")
	assert.Equal(t, "myproj", capturedData.ScaffoldVars["project_name"], "scaffold vars not in bundle should be preserved")
}

func TestUT_GenerateBundle_EmptyBundleVars(t *testing.T) {
	tmpDir := setupTempDir(t)

	templateContent := "---\nto: output/{{ .Name }}.txt\n---\nHello"
	createGenerator(t, tmpDir, "hello", templateContent)
	createSharedDir(t, tmpDir)

	bundleJSON := `{"name":"mybundle","vars":{},"generators":[{"name":"hello"}]}`
	createBundle(t, tmpDir, "mybundle", bundleJSON)

	cfg := createTestConfig(t, tmpDir)
	cfg.Variables = map[string]any{"project_name": "myproj"}

	ctx := createTestCLIContext(t, []string{"mybundle", "world"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.BundlePathFlag: "_bundles",
	})

	var capturedData engine.Data
	fac := defaultGeneratorFactories()
	fac.newBundleEngine = func(_ *template.Engine, _ bool, _ string, _ string, _ *history.Recorder, _ io.Writer) (engine.Generator, error) {
		return &mockGenerator{
			GenerateFunc: func(data engine.Data) (engine.GenerateResult, error) {
				capturedData = data
				return engine.GenerateResult{}, nil
			},
		}, nil
	}

	err := generateAction(ctx, cfg, fac)

	require.NoError(t, err)
	assert.Equal(t, "myproj", capturedData.ScaffoldVars["project_name"], "scaffold vars preserved with empty bundle vars")
}

func TestUT_GenerateBundle_NilBundleVars(t *testing.T) {
	tmpDir := setupTempDir(t)

	templateContent := "---\nto: output/{{ .Name }}.txt\n---\nHello"
	createGenerator(t, tmpDir, "hello", templateContent)
	createSharedDir(t, tmpDir)

	bundleJSON := `{"name":"mybundle","generators":[{"name":"hello"}]}`
	createBundle(t, tmpDir, "mybundle", bundleJSON)

	cfg := createTestConfig(t, tmpDir)
	cfg.Variables = map[string]any{"project_name": "myproj"}

	ctx := createTestCLIContext(t, []string{"mybundle", "world"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.BundlePathFlag: "_bundles",
	})

	var capturedData engine.Data
	fac := defaultGeneratorFactories()
	fac.newBundleEngine = func(_ *template.Engine, _ bool, _ string, _ string, _ *history.Recorder, _ io.Writer) (engine.Generator, error) {
		return &mockGenerator{
			GenerateFunc: func(data engine.Data) (engine.GenerateResult, error) {
				capturedData = data
				return engine.GenerateResult{}, nil
			},
		}, nil
	}

	err := generateAction(ctx, cfg, fac)

	require.NoError(t, err)
	assert.Equal(t, "myproj", capturedData.ScaffoldVars["project_name"], "scaffold vars preserved when bundle has no vars")
}

func TestUT_GenerateBundle_BundleVarsDoNotMutateCfg(t *testing.T) {
	tmpDir := setupTempDir(t)

	templateContent := "---\nto: output/{{ .Name }}.txt\n---\nHello"
	createGenerator(t, tmpDir, "gen1", templateContent)
	createGenerator(t, tmpDir, "gen2", templateContent)
	createSharedDir(t, tmpDir)

	bundleJSON := `{"name":"mybundle","vars":{"domain":"tenant"},"generators":[{"name":"gen1"},{"name":"gen2"}]}`
	createBundle(t, tmpDir, "mybundle", bundleJSON)

	cfg := createTestConfig(t, tmpDir)
	cfg.Variables = map[string]any{"project_name": "myproj"}

	ctx := createTestCLIContext(t, []string{"mybundle", "world"}, map[string]any{
		flags.PathFlag:       tmpDir,
		flags.SharedPathFlag: "_shared",
		flags.BundlePathFlag: "_bundles",
	})

	fac := defaultGeneratorFactories()
	fac.newBundleEngine = func(_ *template.Engine, _ bool, _ string, _ string, _ *history.Recorder, _ io.Writer) (engine.Generator, error) {
		return &mockGenerator{}, nil
	}

	err := generateAction(ctx, cfg, fac)

	require.NoError(t, err)
	_, hasDomain := cfg.Variables["domain"]
	assert.False(t, hasDomain, "cfg.Variables should not be mutated by bundle vars merge")
}

// --- Enhancement 2a: Hook Env Vars tests ---

func TestUT_RunHooks_SetsGeneratorNameEnv(t *testing.T) {
	tmpDir := setupTempDir(t)

	markerFile := filepath.Join(tmpDir, "gen-name.txt")
	scriptContent := "#!/bin/bash\necho \"$TAG_GENERATOR_NAME\" > " + markerFile
	scriptPath := filepath.Join(tmpDir, "check-env.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0o755))

	oldDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(oldDir) })

	err = runHooks([][]string{{"./check-env.sh"}}, hooks.HookPhasePreGen, nil, io.Discard, "my-generator", "my-target")
	require.NoError(t, err)

	content, readErr := os.ReadFile(markerFile)
	require.NoError(t, readErr)
	assert.Equal(t, "my-generator\n", string(content))
}

func TestUT_RunHooks_SetsTargetNameEnv(t *testing.T) {
	tmpDir := setupTempDir(t)

	markerFile := filepath.Join(tmpDir, "target-name.txt")
	scriptContent := "#!/bin/bash\necho \"$TAG_TARGET_NAME\" > " + markerFile
	scriptPath := filepath.Join(tmpDir, "check-env.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0o755))

	oldDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(oldDir) })

	err = runHooks([][]string{{"./check-env.sh"}}, hooks.HookPhasePreGen, nil, io.Discard, "my-generator", "my-target")
	require.NoError(t, err)

	content, readErr := os.ReadFile(markerFile)
	require.NoError(t, readErr)
	assert.Equal(t, "my-target\n", string(content))
}

func TestUT_RunHooks_EnvVarsPresentWithNilVars(t *testing.T) {
	tmpDir := setupTempDir(t)

	markerFile := filepath.Join(tmpDir, "env-check.txt")
	scriptContent := "#!/bin/bash\necho \"GEN=$TAG_GENERATOR_NAME TARGET=$TAG_TARGET_NAME\" > " + markerFile
	scriptPath := filepath.Join(tmpDir, "check-env.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0o755))

	oldDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(oldDir) })

	err = runHooks([][]string{{"./check-env.sh"}}, hooks.HookPhasePreGen, nil, io.Discard, "crud-tenant", "widget")
	require.NoError(t, err)

	content, readErr := os.ReadFile(markerFile)
	require.NoError(t, readErr)
	assert.Equal(t, "GEN=crud-tenant TARGET=widget\n", string(content))
}

// --- Enhancement 2b: Post-Hook Warning tests ---

func TestUT_GenerateWithHooks_PostHookFailureIsWarning(t *testing.T) {
	tmpDir := setupTempDir(t)

	scriptPath := filepath.Join(tmpDir, "post-fail.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/bash\nexit 1"), 0o755))

	cfg := createTestConfig(t, tmpDir)
	cfg.Hooks.Post = [][]string{{scriptPath}}

	var buf bytes.Buffer
	ctx := createTestCLIContext(t, []string{"mygen", "mytarget"}, map[string]any{
		flags.PathFlag: tmpDir,
	})
	ctx.App.Writer = &buf

	oldDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(oldDir) })

	// Suppress slog warning output in test.
	origLogger := slog.Default()
	slog.SetDefault(slog.New(slog.DiscardHandler))
	t.Cleanup(func() { slog.SetDefault(origLogger) })

	_, err = generateWithHooks(ctx, cfg, "mygen", "mytarget", false, func(_ *history.Recorder) (engine.GenerateResult, error) {
		return engine.GenerateResult{Created: 1}, nil
	})

	require.NoError(t, err, "post-hook failure should not return error")
	assert.Contains(t, buf.String(), "warning:")
}

func TestUT_GenerateWithHooks_PreHookFailureIsFatal(t *testing.T) {
	tmpDir := setupTempDir(t)

	scriptPath := filepath.Join(tmpDir, "pre-fail.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/bash\nexit 1"), 0o755))

	cfg := createTestConfig(t, tmpDir)
	cfg.Hooks.Pre = [][]string{{scriptPath}}

	ctx := createTestCLIContext(t, []string{"mygen", "mytarget"}, map[string]any{
		flags.PathFlag: tmpDir,
	})

	oldDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(oldDir) })

	_, err = generateWithHooks(ctx, cfg, "mygen", "mytarget", false, func(_ *history.Recorder) (engine.GenerateResult, error) {
		t.Fatal("generate function should not be called when pre-hook fails")
		return engine.GenerateResult{}, nil
	})

	require.Error(t, err, "pre-hook failure should return error")
	assert.Contains(t, err.Error(), "hook failed")
}
