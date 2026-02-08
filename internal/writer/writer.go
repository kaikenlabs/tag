package writer

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	// FileModeDefault is the default file permission (rw-r--r--).
	FileModeDefault = 0o644
)

// New - create a new writer
func New(dryRun bool) Write {
	return Write{
		mx: sync.Mutex{},
		fs: setFileWriter(dryRun),
	}
}

// validatePathWithinDir ensures that path stays within the base directory.
// This prevents path traversal attacks where template output paths could escape the working directory.
func validatePathWithinDir(path, baseDir string) error {
	absPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	absBase, err := filepath.Abs(filepath.Clean(baseDir))
	if err != nil {
		return fmt.Errorf("failed to resolve absolute base: %w", err)
	}

	if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) && absPath != absBase {
		return fmt.Errorf("path %q escapes base directory %q", absPath, absBase)
	}

	return nil
}

// WriteFile thin wrapper to decouple dependency of write file.
// Validates that the output path stays within the current working directory
// to prevent path traversal via malicious generator templates.
func (w *Write) WriteFile(name string, data []byte, perm fs.FileMode) error {
	w.mx.Lock()
	defer w.mx.Unlock()

	// Validate path containment against working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}
	if err := validatePathWithinDir(name, cwd); err != nil {
		return fmt.Errorf("path safety check failed: %w", err)
	}

	return w.fs.WriteFile(name, data, perm)
}

// AppendFile - append data to the end of file
func (w *Write) AppendFile(name string, data []byte) error {
	w.mx.Lock()
	defer w.mx.Unlock()
	file, err := w.fs.OpenFile(name, os.O_APPEND|os.O_WRONLY, FileModeDefault)
	if err != nil {
		slog.Error("cannot open file", "file", name, "error", err)
		return err
	}
	if _, err := w.fs.Write(file, data); err != nil {
		slog.Error("cannot append data to file", "file", file, "error", err)
		return err
	}
	return nil
}

// InjectIntoFile - inject before, or after a matcher for a source file.
// If the matcher can't be found, don't do anything to the file
func (w *Write) InjectIntoFile(name string, data []byte, inject Inject) error {
	w.mx.Lock()
	defer w.mx.Unlock()
	source, err := w.fs.ReadFile(name)
	if err != nil {
		slog.Error("cannot inject data", "file", name, "error", err)
		return err
	}
	formatedOutput, err := mergeInjection(source, data, inject)
	if err != nil {
		slog.Error("cannot inject via matcher", "error", err, "file", name, "matcher", inject.Matcher, "clause", inject.Clause)
		return err
	}
	if err := w.fs.WriteFile(name, formatedOutput, FileModeDefault); err != nil {
		slog.Error("cannot write to file", "file", name, "error", err)
		return err
	}
	return nil
}

// setFileWriter - return a writer based on the dry run flag.
// If the dry fun flag is true, return a writer that logs to stdout,
// otherwise return a file writer.
func setFileWriter(dryrun bool) fileReadWrite {
	if dryrun {
		return &fileLog{}
	}
	return &fileWrite{}
}
