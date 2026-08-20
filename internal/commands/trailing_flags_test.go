package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/engine"
)

func writeMinimalTagTemplate(t *testing.T, dir, name string) {
	t.Helper()

	data, err := json.Marshal(map[string]any{
		"name":    name,
		"version": "1.0.0",
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), data, 0o644))
}

func TestUT_LibAdd_AsFlagBothOrders(t *testing.T) {
	tests := []struct {
		name string
		argv func(srcDir string) []string
	}{
		{"leading", func(srcDir string) []string { return []string{"add", "--as", "renamed", srcDir} }},
		{"trailing", func(srcDir string) []string { return []string{"add", srcDir, "--as", "renamed"} }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataHome := t.TempDir()
			t.Setenv("XDG_DATA_HOME", dataHome)

			srcParent := t.TempDir()
			srcDir := filepath.Join(srcParent, "derived-name")
			require.NoError(t, os.MkdirAll(srcDir, 0o750))
			writeMinimalTagTemplate(t, srcDir, "derived-name")

			var buf bytes.Buffer
			err := newTestApp(libAddCommand(), &buf).Run(append([]string{"tag"}, tt.argv(srcDir)...))
			require.NoError(t, err)

			renamedDir := filepath.Join(dataHome, "tag", "templates", "renamed")
			derivedDir := filepath.Join(dataHome, "tag", "templates", "derived-name")
			assert.DirExists(t, renamedDir)
			assert.NoDirExists(t, derivedDir)
		})
	}
}

func TestUT_LibEdit_EditorFlagBothOrders(t *testing.T) {
	tests := []struct {
		name string
		argv func(tplName string) []string
	}{
		{"leading", func(tplName string) []string {
			return []string{"edit", "--editor", "tag-guard-nonexistent-editor", tplName}
		}},
		{"trailing", func(tplName string) []string {
			return []string{"edit", tplName, "--editor", "tag-guard-nonexistent-editor"}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("EDITOR", "")
			t.Setenv("VISUAL", "")
			setupFakeLibrary(t, "edit-target")

			var buf bytes.Buffer
			err := newTestApp(libEditCommand(), &buf).Run(append([]string{"tag"}, tt.argv("edit-target")...))

			require.Error(t, err)
			assert.Contains(t, err.Error(), "tag-guard-nonexistent-editor")
		})
	}
}

func TestUT_TemplateNewBundle_LibFlagBothOrders(t *testing.T) {
	tests := []struct {
		name string
		argv []string
	}{
		{"leading", []string{"bundle", "--lib", "mybundle"}},
		{"trailing", []string{"bundle", "mybundle", "--lib"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			templateDir := setupFakeLibrary(t, "my-template")
			tmpDir := t.TempDir()
			cfg := createTestConfigWithLib(t, tmpDir, "my-template")

			var buf bytes.Buffer
			err := newTestApp(templateNewBundleCommand(cfg), &buf).Run(append([]string{"tag"}, tt.argv...))
			require.NoError(t, err)

			libBundlePath := filepath.Join(templateDir, ".tag", "_bundles", "mybundle", "mybundle.json")
			localBundlePath := filepath.Join(tmpDir, "_bundles", "mybundle", "mybundle.json")
			assert.FileExists(t, libBundlePath)
			assert.NoFileExists(t, localBundlePath)
		})
	}
}

func TestUT_TemplateNewBundle_SelfContainedBothOrders(t *testing.T) {
	tests := []struct {
		name string
		argv []string
	}{
		{"leading", []string{"bundle", "--self-contained", "mybundle"}},
		{"trailing", []string{"bundle", "mybundle", "--self-contained"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			cfg := createTestConfig(t, tmpDir)

			var buf bytes.Buffer
			err := newTestApp(templateNewBundleCommand(cfg), &buf).Run(append([]string{"tag"}, tt.argv...))
			require.NoError(t, err)

			bundlePath := filepath.Join(tmpDir, "_bundles", "mybundle", "mybundle.json")
			data, readErr := os.ReadFile(bundlePath)
			require.NoError(t, readErr)

			var bundle engine.Bundle
			require.NoError(t, json.Unmarshal(data, &bundle))
			assert.True(t, bundle.SelfContained)
		})
	}
}

func TestUT_TemplateNewGenerator_PackageBothOrders(t *testing.T) {
	tests := []struct {
		name string
		argv []string
	}{
		{"leading", []string{"generator", "--package", "custompkg", "mygen"}},
		{"trailing", []string{"generator", "mygen", "--package", "custompkg"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			cfg := createTestConfig(t, tmpDir)

			var buf bytes.Buffer
			err := newTestApp(templateNewGeneratorCommand(cfg), &buf).Run(append([]string{"tag"}, tt.argv...))
			require.NoError(t, err)

			data, readErr := os.ReadFile(filepath.Join(tmpDir, "mygen", "mygen.go"))
			require.NoError(t, readErr)
			content := string(data)
			assert.Contains(t, content, "package custompkg")
			assert.NotContains(t, content, "package mypackage")
		})
	}
}

func TestUT_TemplateNewGenerator_LibFlagBothOrders(t *testing.T) {
	tests := []struct {
		name string
		argv []string
	}{
		{"leading", []string{"generator", "--lib", "mygen"}},
		{"trailing", []string{"generator", "mygen", "--lib"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			templateDir := setupFakeLibrary(t, "my-template")
			tmpDir := t.TempDir()
			cfg := createTestConfigWithLib(t, tmpDir, "my-template")

			var buf bytes.Buffer
			err := newTestApp(templateNewGeneratorCommand(cfg), &buf).Run(append([]string{"tag"}, tt.argv...))
			require.NoError(t, err)

			libGenPath := filepath.Join(templateDir, ".tag", "mygen", "mygen.go")
			localGenPath := filepath.Join(tmpDir, "mygen", "mygen.go")
			assert.FileExists(t, libGenPath)
			assert.NoFileExists(t, localGenPath)
		})
	}
}

func TestUT_TemplateNewGenerator_InBundleBothOrders(t *testing.T) {
	tests := []struct {
		name string
		argv []string
	}{
		{"leading", []string{"generator", "--in-bundle", "mybundle", "mygen"}},
		{"trailing", []string{"generator", "mygen", "--in-bundle", "mybundle"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			cfg := createTestConfig(t, tmpDir)

			bundleDir := filepath.Join(tmpDir, "_bundles", "mybundle")
			require.NoError(t, os.MkdirAll(bundleDir, 0o750))

			var buf bytes.Buffer
			err := newTestApp(templateNewGeneratorCommand(cfg), &buf).Run(append([]string{"tag"}, tt.argv...))
			require.NoError(t, err)

			inBundlePath := filepath.Join(bundleDir, "mygen", "mygen.go")
			outsideBundlePath := filepath.Join(tmpDir, "mygen", "mygen.go")
			assert.FileExists(t, inBundlePath)
			assert.NoFileExists(t, outsideBundlePath)
		})
	}
}

func snapshotTree(t *testing.T, root string) map[string][]byte {
	t.Helper()

	tree := make(map[string][]byte)
	require.NoError(t, filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		tree[rel] = data
		return nil
	}))
	return tree
}

func TestUT_TemplateRenameVar_DryRunBothOrders(t *testing.T) {
	tests := []struct {
		name string
		argv func(root string) []string
	}{
		{"leading", func(root string) []string { return []string{"rename-var", "--dry-run", "old", "new", root} }},
		{"trailing", func(root string) []string { return []string{"rename-var", "old", "new", root, "--dry-run"} }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(root, "tag.template.json"),
				[]byte(`{"vars": {"old": {"type": "string"}}}`), 0o644))
			require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"),
				[]byte("# {{ vars.old }}\n"), 0o644))

			before := snapshotTree(t, root)

			var buf bytes.Buffer
			var runErr error
			stdout := captureStdout(t, func() {
				runErr = newTestApp(templateRenameVarCommand(), &buf).Run(append([]string{"tag"}, tt.argv(root)...))
			})
			require.NoError(t, runErr)
			assert.Contains(t, stdout, "Renaming")

			after := snapshotTree(t, root)
			assert.Equal(t, before, after, "dry-run must not modify any file")
		})
	}
}

func TestUT_GenerateAgentFile_OutputBothOrders(t *testing.T) {
	tests := []struct {
		name string
		argv []string
	}{
		{"leading", []string{"agent-file", "--output", "OUT.md", "claude"}},
		{"trailing", []string{"agent-file", "claude", "--output", "OUT.md"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			t.Chdir(tmpDir)
			cfg := createTestConfig(t, tmpDir)

			var buf bytes.Buffer
			err := newTestApp(generateAgentFileCommand(cfg), &buf).Run(append([]string{"tag"}, tt.argv...))
			require.NoError(t, err)

			assert.FileExists(t, filepath.Join(tmpDir, "OUT.md"))
			assert.NoFileExists(t, filepath.Join(tmpDir, "CLAUDE.md"))
		})
	}
}

func TestUT_TrailingFlags_UnknownFlagIsNamedErrorAndNoSideEffect(t *testing.T) {
	tests := []struct {
		name      string
		cmd       func(t *testing.T) *cli.Command
		argv      []string
		untouched func(t *testing.T) string
	}{
		{
			name: "lib add",
			cmd: func(t *testing.T) *cli.Command {
				t.Helper()
				return libAddCommand()
			},
			argv: []string{"add", "somesrc", "--nope"},
			untouched: func(t *testing.T) string {
				t.Helper()
				dataHome := t.TempDir()
				t.Setenv("XDG_DATA_HOME", dataHome)
				return dataHome
			},
		},
		{
			name: "lib edit",
			cmd: func(t *testing.T) *cli.Command {
				t.Helper()
				return libEditCommand()
			},
			argv: []string{"edit", "sometpl", "--nope"},
			untouched: func(t *testing.T) string {
				t.Helper()
				return setupFakeLibrary(t, "sometpl")
			},
		},
		{
			name: "template new bundle",
			cmd: func(t *testing.T) *cli.Command {
				t.Helper()
				return templateNewBundleCommand(createTestConfig(t, t.TempDir()))
			},
			argv: []string{"bundle", "mybundle", "--nope"},
			untouched: func(t *testing.T) string {
				t.Helper()
				return t.TempDir()
			},
		},
		{
			name: "template new generator",
			cmd: func(t *testing.T) *cli.Command {
				t.Helper()
				return templateNewGeneratorCommand(createTestConfig(t, t.TempDir()))
			},
			argv: []string{"generator", "mygen", "--nope"},
			untouched: func(t *testing.T) string {
				t.Helper()
				return t.TempDir()
			},
		},
		{
			name: "template rename-var",
			cmd: func(t *testing.T) *cli.Command {
				t.Helper()
				return templateRenameVarCommand()
			},
			argv: []string{"rename-var", "old", "new", "--nope"},
			untouched: func(t *testing.T) string {
				t.Helper()
				root := t.TempDir()
				require.NoError(t, os.WriteFile(filepath.Join(root, "tag.template.json"),
					[]byte(`{"vars": {"old": {"type": "string"}}}`), 0o644))
				return root
			},
		},
		{
			name: "generate agent-file",
			cmd: func(t *testing.T) *cli.Command {
				t.Helper()
				return generateAgentFileCommand(createTestConfig(t, t.TempDir()))
			},
			argv: []string{"agent-file", "claude", "--nope"},
			untouched: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				t.Chdir(dir)
				return dir
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.untouched(t)
			before := snapshotTree(t, dir)

			var buf bytes.Buffer
			err := newTestApp(tt.cmd(t), &buf).Run(append([]string{"tag"}, tt.argv...))

			require.Error(t, err)
			assert.Contains(t, err.Error(), "unknown flag -nope")

			after := snapshotTree(t, dir)
			assert.Equal(t, before, after, "an unrecognised trailing flag must produce no side effect")
		})
	}
}

func TestUT_TrailingFlags_EqualsForm(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := createTestConfig(t, tmpDir)

	var buf bytes.Buffer
	err := newTestApp(templateNewGeneratorCommand(cfg), &buf).
		Run([]string{"tag", "generator", "mygen", "--package=custompkg"})
	require.NoError(t, err)

	data, readErr := os.ReadFile(filepath.Join(tmpDir, "mygen", "mygen.go"))
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "package custompkg")
}

func TestUT_TrailingFlags_ValuelessFlagDoesNotSwallowNextToken(t *testing.T) {
	templateDir := setupFakeLibrary(t, "my-template")
	tmpDir := t.TempDir()
	cfg := createTestConfigWithLib(t, tmpDir, "my-template")

	var buf bytes.Buffer
	err := newTestApp(templateNewBundleCommand(cfg), &buf).
		Run([]string{"tag", "bundle", "b", "--self-contained", "--lib"})
	require.NoError(t, err)

	bundlePath := filepath.Join(templateDir, ".tag", "_bundles", "b", "b.json")
	data, readErr := os.ReadFile(bundlePath)
	require.NoError(t, readErr)

	var bundle engine.Bundle
	require.NoError(t, json.Unmarshal(data, &bundle))
	assert.True(t, bundle.SelfContained, "--self-contained must be applied")
}

func TestUT_TrailingFlags_ValueFlagAtEndOfArgvErrors(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)

	var buf bytes.Buffer
	err := newTestApp(libAddCommand(), &buf).Run([]string{"tag", "add", "somesrc", "--as"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "flag -as requires a value")
}

func TestUT_TemplateRenameVar_DoubleDashOnlyEscapesAfterAPositional(t *testing.T) {
	t.Run("trailing double-dash escapes to a positional", func(t *testing.T) {
		var buf bytes.Buffer
		var err error
		captureStdout(t, func() {
			err = newTestApp(templateRenameVarCommand(), &buf).
				Run([]string{"tag", "rename-var", "old", "new", "--", "--weird"})
		})

		require.Error(t, err)
		assert.NotContains(t, err.Error(), "unknown flag")
		assert.NotContains(t, err.Error(), "got 4 argument")
		assert.Contains(t, err.Error(), "--weird")
	})

	t.Run("leading double-dash does not protect the literal token", func(t *testing.T) {
		var buf bytes.Buffer
		var err error
		captureStdout(t, func() {
			err = newTestApp(templateRenameVarCommand(), &buf).
				Run([]string{"tag", "rename-var", "--", "--weird"})
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown flag -weird")
	})
}

func TestUT_LibAdd_ForceFlagBothOrders(t *testing.T) {
	tests := []struct {
		name string
		argv func(srcDir string) []string
	}{
		{"leading", func(srcDir string) []string { return []string{"add", "--force", srcDir} }},
		{"trailing", func(srcDir string) []string { return []string{"add", srcDir, "--force"} }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataHome := t.TempDir()
			t.Setenv("XDG_DATA_HOME", dataHome)

			srcParent := t.TempDir()
			srcDir := filepath.Join(srcParent, "dupe-name")
			require.NoError(t, os.MkdirAll(srcDir, 0o750))
			writeMinimalTagTemplate(t, srcDir, "dupe-name")

			var first bytes.Buffer
			require.NoError(t, newTestApp(libAddCommand(), &first).Run([]string{"tag", "add", srcDir}))

			var second bytes.Buffer
			err := newTestApp(libAddCommand(), &second).Run(append([]string{"tag"}, tt.argv(srcDir)...))
			require.NoError(t, err, "re-adding an existing name with --force must succeed regardless of flag position")
			assert.Contains(t, second.String(), "Updated")
		})
	}
}

func TestUT_TemplateRenameVar_PositionalCountUsesReparsedSlice(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "tag.template.json"),
		[]byte(`{"vars": {"a": {"type": "string"}}}`), 0o644))

	t.Run("three positionals plus a trailing flag is OK", func(t *testing.T) {
		var buf bytes.Buffer
		var err error
		captureStdout(t, func() {
			err = newTestApp(templateRenameVarCommand(), &buf).
				Run([]string{"tag", "rename-var", "a", "b", root, "--dry-run"})
		})
		require.NoError(t, err)
	})

	t.Run("four positionals is still a usage error", func(t *testing.T) {
		var buf bytes.Buffer
		err := newTestApp(templateRenameVarCommand(), &buf).
			Run([]string{"tag", "rename-var", "a", "b", "c", "d"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "got 4 argument(s)")
	})
}

// A trailing value flag with no value must name the flag rather than fall
// through to the arity check. Before the reparse, the token was discarded and
// lib edit went on to open an editor on the template.
func TestUT_LibEdit_TrailingEditorFlagWithoutValueNamesTheFlag(t *testing.T) {
	t.Setenv("EDITOR", "")
	t.Setenv("VISUAL", "")
	setupFakeLibrary(t, "edit-target")

	var buf bytes.Buffer
	err := newTestApp(libEditCommand(), &buf).Run([]string{"tag", "edit", "edit-target", "--editor"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "flag -editor requires a value")
}

func TestUT_TemplateNewBundle_TrailingBundlePathIsHonoured(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := createTestConfig(t, tmpDir)

	var buf bytes.Buffer
	err := newTestApp(templateNewBundleCommand(cfg), &buf).
		Run([]string{"tag", "bundle", "b", "--bundle-path", "custom"})
	require.NoError(t, err)

	customPath := filepath.Join(tmpDir, "custom", "b", "b.json")
	defaultPath := filepath.Join(tmpDir, "_bundles", "b", "b.json")
	assert.FileExists(t, customPath)
	assert.NoFileExists(t, defaultPath)
}

// Guards the c.App.Flags registration inside reparseTrailingFlags: without it a
// trailing App-level global is rejected as an unknown flag. Most of these
// commands read no global, so "not rejected" is the whole assertion — the cases
// where a global genuinely changes behaviour are covered by
// TestUT_TemplateNewBundle_TrailingBundlePathIsHonoured and the dry-run tests.
func TestUT_TrailingFlags_KnownGlobalIsNotRejectedAsUnknown(t *testing.T) {
	t.Run("lib add", func(t *testing.T) {
		dataHome := t.TempDir()
		t.Setenv("XDG_DATA_HOME", dataHome)

		srcParent := t.TempDir()
		srcDir := filepath.Join(srcParent, "globaltest")
		require.NoError(t, os.MkdirAll(srcDir, 0o750))
		writeMinimalTagTemplate(t, srcDir, "globaltest")

		var buf bytes.Buffer
		err := newTestApp(libAddCommand(), &buf).
			Run([]string{"tag", "add", srcDir, "--path", "ignored"})
		require.NoError(t, err)
	})

	t.Run("lib edit", func(t *testing.T) {
		t.Setenv("EDITOR", "")
		t.Setenv("VISUAL", "")
		setupFakeLibrary(t, "edit-target")

		var buf bytes.Buffer
		err := newTestApp(libEditCommand(), &buf).Run([]string{
			"tag", "edit", "edit-target",
			"--editor", "tag-guard-nonexistent-editor",
			"--path", "ignored",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "tag-guard-nonexistent-editor")
		assert.NotContains(t, err.Error(), "unknown flag -path")
	})

	t.Run("template new bundle", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg := createTestConfig(t, tmpDir)

		var buf bytes.Buffer
		err := newTestApp(templateNewBundleCommand(cfg), &buf).
			Run([]string{"tag", "bundle", "b", "--path", "ignored"})
		require.NoError(t, err)
		assert.FileExists(t, filepath.Join(tmpDir, "_bundles", "b", "b.json"))
	})

	t.Run("template new generator", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg := createTestConfig(t, tmpDir)

		var buf bytes.Buffer
		err := newTestApp(templateNewGeneratorCommand(cfg), &buf).
			Run([]string{"tag", "generator", "g", "--path", "ignored"})
		require.NoError(t, err)
		assert.FileExists(t, filepath.Join(tmpDir, "g", "g.go"))
	})

	t.Run("template rename-var", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "tag.template.json"),
			[]byte(`{"vars": {"old": {"type": "string"}}}`), 0o644))

		var buf bytes.Buffer
		var err error
		captureStdout(t, func() {
			err = newTestApp(templateRenameVarCommand(), &buf).
				Run([]string{"tag", "rename-var", "old", "new", root, "--path", "ignored"})
		})
		require.NoError(t, err)
	})

	t.Run("generate agent-file", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		cfg := createTestConfig(t, dir)

		var buf bytes.Buffer
		err := newTestApp(generateAgentFileCommand(cfg), &buf).
			Run([]string{"tag", "agent-file", "claude", "--path", "ignored"})
		require.NoError(t, err)
		assert.FileExists(t, filepath.Join(dir, "CLAUDE.md"))
	})
}

// The long-name form of every flag is covered by the BothOrders tests above.
// reparseTrailingFlags resolves an alias to its canonical name separately (see
// the canonical map it builds from f.Names()), so the short forms need their
// own trailing-position coverage through a real cli.App.
func TestUT_TrailingFlags_ShortAliasesAreHonoured(t *testing.T) {
	t.Run("-p sets package", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg := createTestConfig(t, tmpDir)

		var buf bytes.Buffer
		err := newTestApp(templateNewGeneratorCommand(cfg), &buf).
			Run([]string{"tag", "generator", "mygen", "-p", "custompkg"})
		require.NoError(t, err)

		data, readErr := os.ReadFile(filepath.Join(tmpDir, "mygen", "mygen.go"))
		require.NoError(t, readErr)
		assert.Contains(t, string(data), "package custompkg")
		assert.NotContains(t, string(data), "package mypackage")
	})

	t.Run("-s sets self-contained", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg := createTestConfig(t, tmpDir)

		var buf bytes.Buffer
		err := newTestApp(templateNewBundleCommand(cfg), &buf).
			Run([]string{"tag", "bundle", "mybundle", "-s"})
		require.NoError(t, err)

		data, readErr := os.ReadFile(filepath.Join(tmpDir, "_bundles", "mybundle", "mybundle.json"))
		require.NoError(t, readErr)

		var bundle engine.Bundle
		require.NoError(t, json.Unmarshal(data, &bundle))
		assert.True(t, bundle.SelfContained)
	})

	t.Run("-B sets in-bundle", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg := createTestConfig(t, tmpDir)

		bundleDir := filepath.Join(tmpDir, "_bundles", "mybundle")
		require.NoError(t, os.MkdirAll(bundleDir, 0o750))

		var buf bytes.Buffer
		err := newTestApp(templateNewGeneratorCommand(cfg), &buf).
			Run([]string{"tag", "generator", "mygen", "-B", "mybundle"})
		require.NoError(t, err)

		assert.FileExists(t, filepath.Join(bundleDir, "mygen", "mygen.go"))
		assert.NoFileExists(t, filepath.Join(tmpDir, "mygen", "mygen.go"))
	})

	t.Run("-l sets lib", func(t *testing.T) {
		templateDir := setupFakeLibrary(t, "my-template")
		tmpDir := t.TempDir()
		cfg := createTestConfigWithLib(t, tmpDir, "my-template")

		var buf bytes.Buffer
		err := newTestApp(templateNewGeneratorCommand(cfg), &buf).
			Run([]string{"tag", "generator", "mygen", "-l"})
		require.NoError(t, err)

		assert.FileExists(t, filepath.Join(templateDir, ".tag", "mygen", "mygen.go"))
		assert.NoFileExists(t, filepath.Join(tmpDir, "mygen", "mygen.go"))
	})

	t.Run("-o sets output", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		cfg := createTestConfig(t, dir)

		var buf bytes.Buffer
		err := newTestApp(generateAgentFileCommand(cfg), &buf).
			Run([]string{"tag", "agent-file", "claude", "-o", "OUT.md"})
		require.NoError(t, err)

		assert.FileExists(t, filepath.Join(dir, "OUT.md"))
		assert.NoFileExists(t, filepath.Join(dir, "CLAUDE.md"))
	})

	t.Run("-f sets force", func(t *testing.T) {
		dataHome := t.TempDir()
		t.Setenv("XDG_DATA_HOME", dataHome)

		srcParent := t.TempDir()
		srcDir := filepath.Join(srcParent, "aliased")
		require.NoError(t, os.MkdirAll(srcDir, 0o750))
		writeMinimalTagTemplate(t, srcDir, "aliased")

		var first bytes.Buffer
		require.NoError(t, newTestApp(libAddCommand(), &first).
			Run([]string{"tag", "add", srcDir}))

		var conflict bytes.Buffer
		err := newTestApp(libAddCommand(), &conflict).Run([]string{"tag", "add", srcDir})
		require.Error(t, err, "a second add without --force must be rejected")

		var forced bytes.Buffer
		require.NoError(t, newTestApp(libAddCommand(), &forced).
			Run([]string{"tag", "add", srcDir, "-f"}))
	})
}
