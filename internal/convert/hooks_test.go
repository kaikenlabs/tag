package convert

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_DetectHooks_PythonHooks(t *testing.T) {
	// Create temp directory with hooks
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))

	// Create Python hook files
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "pre_gen_project.py"), []byte("# pre hook"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "post_gen_project.py"), []byte("# post hook"), 0o644))

	processor := NewHooksProcessor(tmpDir, t.TempDir(), true)
	findings, err := processor.DetectHooks()

	require.NoError(t, err)
	assert.Len(t, findings, 2)

	// Check that Python hooks are detected
	for _, f := range findings {
		assert.Equal(t, string(HookKindPython), f.Kind)
		assert.Contains(t, f.Message, "Python hook")
		assert.Contains(t, f.Message, "python3")
	}
}

func TestUT_DetectHooks_ShellHooks(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))

	// Create shell hook files
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "pre_gen_project.sh"), []byte("#!/bin/bash"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "post_gen_project.sh"), []byte("#!/bin/bash"), 0o755))

	processor := NewHooksProcessor(tmpDir, t.TempDir(), true)
	findings, err := processor.DetectHooks()

	require.NoError(t, err)
	assert.Len(t, findings, 2)

	for _, f := range findings {
		assert.Equal(t, string(HookKindShell), f.Kind)
		assert.Contains(t, f.Message, "Shell hook")
	}
}

func TestUT_DetectHooks_MixedHooks(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))

	// Create mixed hook files
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "pre_gen_project.py"), []byte("# python"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "post_gen_project.sh"), []byte("#!/bin/bash"), 0o755))

	processor := NewHooksProcessor(tmpDir, t.TempDir(), true)
	findings, err := processor.DetectHooks()

	require.NoError(t, err)
	assert.Len(t, findings, 2)

	kinds := make(map[string]bool)
	for _, f := range findings {
		kinds[f.Kind] = true
	}
	assert.True(t, kinds[string(HookKindPython)])
	assert.True(t, kinds[string(HookKindShell)])
}

func TestUT_DetectHooks_NoHooksDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	// Don't create hooks directory

	processor := NewHooksProcessor(tmpDir, t.TempDir(), true)
	findings, err := processor.DetectHooks()

	require.NoError(t, err)
	assert.Empty(t, findings)
}

func TestUT_DetectHooks_EmptyHooksDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))
	// Empty directory

	processor := NewHooksProcessor(tmpDir, t.TempDir(), true)
	findings, err := processor.DetectHooks()

	require.NoError(t, err)
	assert.Empty(t, findings)
}

func TestUT_CopyHooks_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	destDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))

	hookPath := filepath.Join(hooksDir, "pre_gen_project.sh")
	require.NoError(t, os.WriteFile(hookPath, []byte("#!/bin/bash\necho hello"), 0o755))

	processor := NewHooksProcessor(tmpDir, destDir, true) // dryRun: true
	findings, err := processor.CopyHooks()

	require.NoError(t, err)
	assert.Len(t, findings, 1)
	assert.False(t, findings[0].IsCopied) // Not copied in dry-run

	// Verify file was NOT copied
	_, err = os.Stat(filepath.Join(destDir, "hooks", "pre_gen_project.sh"))
	assert.True(t, os.IsNotExist(err))
}

func TestUT_CopyHooks_ActualCopy(t *testing.T) {
	tmpDir := t.TempDir()
	destDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))

	hookContent := []byte("#!/bin/bash\necho hello")
	hookPath := filepath.Join(hooksDir, "pre_gen_project.sh")
	require.NoError(t, os.WriteFile(hookPath, hookContent, 0o755))

	processor := NewHooksProcessor(tmpDir, destDir, false) // dryRun: false
	findings, err := processor.CopyHooks()

	require.NoError(t, err)
	assert.Len(t, findings, 1)
	assert.True(t, findings[0].IsCopied)

	// Verify file was copied
	destHookPath := filepath.Join(destDir, "hooks", "pre_gen_project.sh")
	copiedContent, err := os.ReadFile(destHookPath)
	require.NoError(t, err)
	assert.Equal(t, hookContent, copiedContent)

	// Verify permissions preserved
	info, err := os.Stat(destHookPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
}

func TestUT_IsHooksDir(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"hooks", true},
		{"hooks/", true}, // HasPrefix matches hooks/
		{"hooks/pre_gen.py", true},
		{"hooks\\post_gen.py", true}, // Windows separator
		{"src/hooks", false},
		{"my_hooks", false},
		{"hookss", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsHooksDir(tt.path))
		})
	}
}

func TestUT_SuggestTagHooksConfig(t *testing.T) {
	findings := []HookFinding{
		{Path: "hooks/pre_gen_project.sh", Kind: string(HookKindShell)},
		{Path: "hooks/post_gen_project.sh", Kind: string(HookKindShell)},
		{Path: "hooks/pre_gen_project.py", Kind: string(HookKindPython)},
	}

	preHooks, postHooks := SuggestTagHooksConfig(findings)

	assert.Len(t, preHooks, 2)
	assert.Len(t, postHooks, 1)
	assert.Equal(t, "sh hooks/pre_gen_project.sh", preHooks[0])
	assert.Equal(t, "hooks/pre_gen_project.py", preHooks[1])
	assert.Equal(t, "sh hooks/post_gen_project.sh", postHooks[0])
}

func TestUT_SuggestTagHooksConfig_PythonOnly(t *testing.T) {
	findings := []HookFinding{
		{Path: "hooks/pre_gen_project.py", Kind: string(HookKindPython)},
		{Path: "hooks/post_gen_project.py", Kind: string(HookKindPython)},
	}

	preHooks, postHooks := SuggestTagHooksConfig(findings)

	assert.Len(t, preHooks, 1)
	assert.Len(t, postHooks, 1)
	assert.Equal(t, "hooks/pre_gen_project.py", preHooks[0])
	assert.Equal(t, "hooks/post_gen_project.py", postHooks[0])
}

func TestUT_SuggestTagHooksConfig_BatchIgnored(t *testing.T) {
	findings := []HookFinding{
		{Path: "hooks/pre_gen_project.bat", Kind: string(HookKindBatch)},
	}

	preHooks, postHooks := SuggestTagHooksConfig(findings)

	assert.Empty(t, preHooks)
	assert.Empty(t, postHooks)
}
