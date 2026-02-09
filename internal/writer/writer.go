package writer

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"sync"

	"github.com/kaikenlabs/tag/internal/fileutil"
)

const (
	// FileModeDefault is the default file permission (rw-r--r--).
	FileModeDefault = 0o644
)

// New creates a new writer with a cached working directory.
// The working directory is resolved once at construction time and reused
// for path containment validation on every write operation.
//
// Deprecated: New returns a concrete Write value. Callers should program
// against the FileWriter interface instead so the writer can be injected.
func New(dryRun bool) (Write, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return Write{}, fmt.Errorf("cannot determine working directory: %w", err)
	}
	return Write{
		mx:  sync.Mutex{},
		fs:  setFileWriter(dryRun),
		cwd: cwd,
	}, nil
}

// validatePathWithinDir ensures that path stays within the base directory.
// This prevents path traversal attacks where template output paths could escape the working directory.
func validatePathWithinDir(path, baseDir string) error {
	return fileutil.ValidatePathContainment(baseDir, path)
}

// WriteFile thin wrapper to decouple dependency of write file.
// Validates that the output path stays within the cached working directory
// to prevent path traversal via malicious generator templates.
func (w *Write) WriteFile(name string, data []byte, perm fs.FileMode) error {
	w.mx.Lock()
	defer w.mx.Unlock()

	if err := validatePathWithinDir(name, w.cwd); err != nil {
		return fmt.Errorf("path safety check failed: %w", err)
	}

	return w.fs.WriteFile(name, data, perm)
}

// AppendFile - append data to the end of file.
// Validates that the target path stays within the cached working directory
// to prevent path traversal via malicious generator templates.
func (w *Write) AppendFile(name string, data []byte) error {
	w.mx.Lock()
	defer w.mx.Unlock()

	if err := validatePathWithinDir(name, w.cwd); err != nil {
		return fmt.Errorf("path safety check failed: %w", err)
	}

	file, err := w.fs.OpenFile(name, os.O_APPEND|os.O_WRONLY, FileModeDefault)
	if err != nil {
		slog.Error("cannot open file", "file", name, "error", err)
		return err
	}
	if file != nil {
		defer func() {
			if err := file.Close(); err != nil {
				slog.Error("cannot close file", "file", file.Name(), "error", err)
			}
		}()
	}
	if _, err := w.fs.Write(file, data); err != nil {
		slog.Error("cannot append data to file", "file", file, "error", err)
		return err
	}
	return nil
}

// InjectIntoFile - inject before, or after a matcher for a source file.
// If the matcher can't be found, don't do anything to the file.
// Validates that the target path stays within the cached working directory
// to prevent path traversal via malicious generator templates.
func (w *Write) InjectIntoFile(name string, data []byte, inject Inject) error {
	w.mx.Lock()
	defer w.mx.Unlock()

	if err := validatePathWithinDir(name, w.cwd); err != nil {
		return fmt.Errorf("path safety check failed: %w", err)
	}

	source, err := w.fs.ReadFile(name)
	if err != nil {
		slog.Error("cannot inject data", "file", name, "error", err)
		return err
	}
	formattedOutput, err := mergeInjection(source, data, inject)
	if err != nil {
		slog.Error("cannot inject via matcher", "error", err, "file", name, "matcher", inject.Matcher, "clause", inject.Clause)
		return err
	}
	if err := w.fs.WriteFile(name, formattedOutput, FileModeDefault); err != nil {
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
