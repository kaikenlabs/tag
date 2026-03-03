package writer

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// fileLog is a dry-run writer that logs output to console instead of writing to disk.
// ReadFile and OpenFile perform real I/O because the inject and append operations
// need to read existing file content to compute the merged output, even in dry-run mode.
type fileLog struct{}

var _ fileReadWrite = (*fileLog)(nil)

func (f *fileLog) WriteFile(name string, data []byte, perm os.FileMode) error {
	slog.Info("logging to console", "name", name, "data", "\n"+string(data))
	return nil
}

func (f *fileLog) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(filepath.Clean(name))
}

func (f *fileLog) OpenFile(name string, flag int, perm fs.FileMode) (*os.File, error) {
	// #nosec G304
	return os.OpenFile(filepath.Clean(name), flag, perm)
}

func (f *fileLog) Write(file *os.File, b []byte) (n int, err error) {
	slog.Info("logging to console", "file", file.Name(), "data", "\n"+string(b)) //nolint:gosec // G706: slog structured logging; log injection not a concern in a CLI tool
	return len(b), nil
}

// ErrUserQuit is returned from the interactive dry-run prompt when the user
// chooses to quit reviewing changes.
var ErrUserQuit = errors.New("dry-run review quit by user")

// fileDiff is an enhanced dry-run writer that shows colored unified diffs
// instead of writing to disk. When isTTY is true, it prompts the user
// interactively (y/n/a/q) after displaying each change.
type fileDiff struct {
	out       io.Writer
	in        io.Reader
	isTTY     bool
	acceptAll bool
}

var _ fileReadWrite = (*fileDiff)(nil)

func (f *fileDiff) WriteFile(name string, data []byte, _ os.FileMode) error {
	existing, err := os.ReadFile(filepath.Clean(name))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("dry-run: read existing file %s: %w", name, err)
		}
		// New file – show all content as additions.
		f.printNewFile(name, data)
	} else {
		// Existing file – show unified line-level diff.
		f.printDiff(name, existing, data)
	}
	if f.isTTY && !f.acceptAll {
		return f.prompt(name)
	}
	return nil
}

func (f *fileDiff) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(filepath.Clean(name))
}

func (f *fileDiff) OpenFile(name string, flag int, perm fs.FileMode) (*os.File, error) {
	// #nosec G304
	return os.OpenFile(filepath.Clean(name), flag, perm)
}

func (f *fileDiff) Write(file *os.File, b []byte) (n int, err error) {
	fmt.Fprintf(f.out, "\n+++ (append) %s\n", file.Name()) //nolint:gosec // G705: file.Name() is a controlled path from the CLI generator; log injection is not a concern in a CLI tool
	for line := range strings.SplitSeq(strings.TrimRight(string(b), "\n"), "\n") {
		fmt.Fprintf(f.out, "\033[32m+%s\033[0m\n", line)
	}
	if f.isTTY && !f.acceptAll {
		if err := f.prompt(file.Name()); err != nil {
			return 0, err
		}
	}
	return len(b), nil
}

// printNewFile prints all lines of a newly-created file with a green + prefix.
func (f *fileDiff) printNewFile(name string, data []byte) {
	fmt.Fprintf(f.out, "\n+++ (new file) %s\n", name)
	for line := range strings.SplitSeq(strings.TrimRight(string(data), "\n"), "\n") {
		fmt.Fprintf(f.out, "\033[32m+%s\033[0m\n", line)
	}
}

// printDiff prints a line-level unified diff between existing and new content.
// It uses DiffMain with checklines=true for a line-biased speedup, then
// walks each diff chunk splitting on newlines to reconstruct per-line output.
func (f *fileDiff) printDiff(name string, existing, newContent []byte) {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(string(existing), string(newContent), true)

	// Check for actual changes.
	hasChanges := false
	for _, d := range diffs {
		if d.Type != diffmatchpatch.DiffEqual {
			hasChanges = true
			break
		}
	}
	if !hasChanges {
		fmt.Fprintf(f.out, "\n--- %s (no changes)\n", name)
		return
	}

	fmt.Fprintf(f.out, "\n--- a/%s\n+++ b/%s\n", name, name)
	for _, d := range diffs {
		// Split chunk into lines; drop trailing empty entry from trailing newline.
		parts := strings.Split(d.Text, "\n")
		if len(parts) > 0 && parts[len(parts)-1] == "" {
			parts = parts[:len(parts)-1]
		}
		for _, line := range parts {
			switch d.Type {
			case diffmatchpatch.DiffInsert:
				fmt.Fprintf(f.out, "\033[32m+%s\033[0m\n", line)
			case diffmatchpatch.DiffDelete:
				fmt.Fprintf(f.out, "\033[31m-%s\033[0m\n", line)
			case diffmatchpatch.DiffEqual:
				fmt.Fprintf(f.out, " %s\n", line)
			}
		}
	}
}

// prompt shows an interactive y/n/a/q prompt.
// Returns ErrUserQuit when the user chooses to quit.
func (f *fileDiff) prompt(name string) error {
	fmt.Fprintf(f.out, "\nApply %s? [y]es/[n]o/[a]ll/[q]uit: ", name)
	scanner := bufio.NewScanner(f.in)
	if scanner.Scan() {
		switch strings.ToLower(strings.TrimSpace(scanner.Text())) {
		case "a", "all":
			f.acceptAll = true
		case "q", "quit":
			return ErrUserQuit
		}
	}
	return nil
}
