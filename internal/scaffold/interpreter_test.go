package scaffold

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_IsFileReference(t *testing.T) {
	tests := []struct {
		cmd      string
		expected bool
	}{
		{"go", false},
		{"npm", false},
		{"echo", false},
		{"./script.sh", true},
		{"hooks/run.py", true},
		{"/usr/bin/python3", true},
		{"scripts/setup.rb", true},
		{".hidden", true},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			assert.Equal(t, tt.expected, isFileReference(tt.cmd))
		})
	}
}

func TestUT_ReadShebang_WithShebang(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "script.py")
	require.NoError(t, os.WriteFile(path, []byte("#!/usr/bin/env python3\nprint('hi')\n"), 0o755))

	shebang, err := readShebang(path)
	require.NoError(t, err)
	assert.Equal(t, "#!/usr/bin/env python3", shebang)
}

func TestUT_ReadShebang_NoShebang(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "script.py")
	require.NoError(t, os.WriteFile(path, []byte("print('hi')\n"), 0o644))

	shebang, err := readShebang(path)
	require.NoError(t, err)
	assert.Equal(t, "", shebang)
}

func TestUT_ReadShebang_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.sh")
	require.NoError(t, os.WriteFile(path, []byte(""), 0o644))

	shebang, err := readShebang(path)
	require.NoError(t, err)
	assert.Equal(t, "", shebang)
}

func TestUT_ReadShebang_FileNotFound(t *testing.T) {
	_, err := readShebang("/nonexistent/path/script.py")
	assert.Error(t, err)
}

func TestUT_ResolveInterpreter_BareCommand(t *testing.T) {
	argv := []string{"go", "mod", "tidy"}
	result := resolveInterpreter(argv, "/tmp")
	assert.Equal(t, []string{"go", "mod", "tidy"}, result)
}

func TestUT_ResolveInterpreter_EmptyArgv(t *testing.T) {
	result := resolveInterpreter([]string{}, "/tmp")
	assert.Empty(t, result)
}

func TestUT_ResolveInterpreter_FileWithShebang(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "script.py")
	require.NoError(t, os.WriteFile(path, []byte("#!/usr/bin/env python3\nprint('hi')\n"), 0o755))

	argv := []string{path}
	result := resolveInterpreter(argv, tmpDir)
	assert.Equal(t, []string{path}, result, "file with shebang should be unchanged")
}

func TestUT_ResolveInterpreter_PythonNoShebang(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "setup.py")
	require.NoError(t, os.WriteFile(path, []byte("print('hello')\n"), 0o644))

	argv := []string{path}
	result := resolveInterpreter(argv, tmpDir)
	require.Len(t, result, 2)
	// First element should be a python interpreter
	assert.Contains(t, result[0], "python")
	assert.Equal(t, path, result[1])
}

func TestUT_ResolveInterpreter_PythonRelativePath(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))
	scriptPath := filepath.Join(hooksDir, "post_gen_project.py")
	require.NoError(t, os.WriteFile(scriptPath, []byte("print('done')\n"), 0o644))

	// Use relative path — resolveInterpreter joins with workDir for shebang check
	argv := []string{"hooks/post_gen_project.py"}
	result := resolveInterpreter(argv, tmpDir)
	require.Len(t, result, 2)
	assert.Contains(t, result[0], "python")
	assert.Equal(t, "hooks/post_gen_project.py", result[1])
}

func TestUT_ResolveInterpreter_ShellNoShebang(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "setup.sh")
	require.NoError(t, os.WriteFile(path, []byte("echo hello\n"), 0o755))

	argv := []string{path}
	result := resolveInterpreter(argv, tmpDir)
	assert.Equal(t, []string{"sh", path}, result)
}

func TestUT_ResolveInterpreter_RubyNoShebang(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "setup.rb")
	require.NoError(t, os.WriteFile(path, []byte("puts 'hello'\n"), 0o644))

	argv := []string{path}
	result := resolveInterpreter(argv, tmpDir)
	assert.Equal(t, []string{"ruby", path}, result)
}

func TestUT_ResolveInterpreter_NodeNoShebang(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "setup.js")
	require.NoError(t, os.WriteFile(path, []byte("console.log('hello')\n"), 0o644))

	argv := []string{path}
	result := resolveInterpreter(argv, tmpDir)
	assert.Equal(t, []string{"node", path}, result)
}

func TestUT_ResolveInterpreter_UnknownExtension(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "script.xyz")
	require.NoError(t, os.WriteFile(path, []byte("something\n"), 0o644))

	argv := []string{path}
	result := resolveInterpreter(argv, tmpDir)
	assert.Equal(t, []string{path}, result, "unknown extension should be unchanged")
}

func TestUT_ResolveInterpreter_NonexistentFile(t *testing.T) {
	argv := []string{"./nonexistent.py"}
	result := resolveInterpreter(argv, "/tmp")
	assert.Equal(t, []string{"./nonexistent.py"}, result, "nonexistent file should be unchanged")
}

func TestUT_ResolveInterpreter_PreservesArgs(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "setup.py")
	require.NoError(t, os.WriteFile(path, []byte("import sys\n"), 0o644))

	argv := []string{path, "--flag", "value"}
	result := resolveInterpreter(argv, tmpDir)
	require.Len(t, result, 4)
	assert.Contains(t, result[0], "python")
	assert.Equal(t, path, result[1])
	assert.Equal(t, "--flag", result[2])
	assert.Equal(t, "value", result[3])
}

func TestUT_FindPythonInterpreter(t *testing.T) {
	// Check if python3 or python is available on this system
	_, err3 := exec.LookPath("python3")
	_, err2 := exec.LookPath("python")

	if err3 != nil && err2 != nil {
		t.Skip("neither python3 nor python found on system")
	}

	result := findPythonInterpreter()
	// Should return a bare command name, not an absolute path
	if err3 == nil {
		assert.Equal(t, "python3", result)
	} else {
		assert.Equal(t, "python", result)
	}
}

// Integration tests

func TestIT_RunArgv_PythonScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	// Check if python is available
	pythonCmd := findPythonInterpreter()
	if _, err := exec.LookPath(pythonCmd); err != nil {
		t.Skip("python not found on system")
	}

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "hooks", "test.py")
	require.NoError(t, os.MkdirAll(filepath.Dir(scriptPath), 0o755))
	require.NoError(t, os.WriteFile(scriptPath, []byte("print('hello from python')\n"), 0o644))

	runner := NewArgvHookRunner()
	results, err := runner.RunArgv(HookPhasePost, [][]string{{"hooks/test.py"}}, tmpDir, nil)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Output, "hello from python")
	assert.Equal(t, 0, results[0].ExitCode)
}

func TestIT_RunArgv_ShellScriptNoShebang(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "hooks", "test.sh")
	require.NoError(t, os.MkdirAll(filepath.Dir(scriptPath), 0o755))
	// No shebang — resolveInterpreter should prepend "sh"
	require.NoError(t, os.WriteFile(scriptPath, []byte("echo 'hello from sh'\n"), 0o755))

	runner := NewArgvHookRunner()
	results, err := runner.RunArgv(HookPhasePost, [][]string{{"hooks/test.sh"}}, tmpDir, nil)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Output, "hello from sh")
	assert.Equal(t, 0, results[0].ExitCode)
}

func TestIT_RunArgv_ShellScriptWithShebang(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "hooks", "test.sh")
	require.NoError(t, os.MkdirAll(filepath.Dir(scriptPath), 0o755))
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/sh\necho 'hello from shebang'\n"), 0o755))

	runner := NewArgvHookRunner()
	results, err := runner.RunArgv(HookPhasePost, [][]string{{"hooks/test.sh"}}, tmpDir, nil)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Output, "hello from shebang")
	assert.Equal(t, 0, results[0].ExitCode)
}
