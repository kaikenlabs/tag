package scaffold

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// maxShebangBytes is the maximum number of bytes to read when checking for a shebang line.
const maxShebangBytes = 256

// extensionInterpreters maps file extensions to their default interpreter commands.
var extensionInterpreters = map[string]string{
	".sh": "sh",
	".rb": "ruby",
	".js": "node",
	".pl": "perl",
}

var (
	pythonOnce        sync.Once
	pythonInterpreter string
)

// resolveInterpreter checks if argv[0] is a script file and prepends the appropriate
// interpreter if needed. Bare commands (go, npm, echo) are returned unchanged.
// Files with shebangs are returned unchanged (the OS handles execution).
// Files without shebangs are matched by extension to find an interpreter.
func resolveInterpreter(argv []string, workDir string) []string {
	if len(argv) == 0 {
		return argv
	}

	cmd := argv[0]

	if !isFileReference(cmd) {
		return argv
	}

	// Resolve the file path for shebang reading
	filePath := cmd
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(workDir, filePath)
	}

	// Check for shebang — if present, OS handles execution
	shebang, err := readShebang(filePath)
	if err != nil {
		// File doesn't exist or can't be read — return unchanged, let exec fail naturally
		return argv
	}
	if shebang != "" {
		return argv
	}

	// No shebang — try extension-based interpreter lookup
	ext := strings.ToLower(filepath.Ext(cmd))

	if ext == ".py" {
		return append([]string{findPythonInterpreter()}, argv...)
	}

	if interpreter, ok := extensionInterpreters[ext]; ok {
		return append([]string{interpreter}, argv...)
	}

	return argv
}

// isFileReference returns true if cmd looks like a file path rather than a bare command.
// A file reference contains path separators (/ or \) or starts with a dot.
func isFileReference(cmd string) bool {
	return strings.ContainsAny(cmd, "/\\") || strings.HasPrefix(cmd, ".")
}

// readShebang reads the first line of a file and returns the shebang line if present.
// Returns empty string if the file doesn't start with #!.
func readShebang(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	buf := make([]byte, maxShebangBytes)
	n, err := f.Read(buf)
	if n == 0 {
		return "", nil
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}

	content := string(buf[:n])
	if !strings.HasPrefix(content, "#!") {
		return "", nil
	}

	// Extract just the first line
	if line, _, ok := strings.Cut(content, "\n"); ok {
		return line, nil
	}
	return content, nil
}

// findPythonInterpreter returns the path to a Python interpreter.
// It tries python3 first, then python, cached via sync.Once.
// Falls back to "python3" so exec fails with a clear "not found" error.
func findPythonInterpreter() string {
	pythonOnce.Do(func() {
		if _, err := exec.LookPath("python3"); err == nil {
			pythonInterpreter = "python3"
			return
		}
		if _, err := exec.LookPath("python"); err == nil {
			pythonInterpreter = "python"
			return
		}
		pythonInterpreter = "python3"
	})
	return pythonInterpreter
}
