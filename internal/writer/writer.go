package writer

import (
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"sync"

	"github.com/kaikenlabs/tag/internal/fileutil"
	"github.com/kaikenlabs/tag/internal/types"
)

// writerConfig holds optional configuration for NewFileWriter.
type writerConfig struct {
	out   io.Writer
	in    io.Reader
	isTTY bool
}

// WriterOption configures NewFileWriter behaviour.
type WriterOption func(*writerConfig)

// WithDiffOutput enables enhanced dry-run diff display.
// out receives the colored diff text; in is the source for interactive prompt
// responses; isTTY controls whether a per-file y/n/a/q prompt is shown.
func WithDiffOutput(out io.Writer, in io.Reader, isTTY bool) WriterOption {
	return func(c *writerConfig) {
		c.out = out
		c.in = in
		c.isTTY = isTTY
	}
}

// NewFileWriter creates a new writer with a cached working directory,
// returning the FileWriter interface for dependency injection.
// The working directory is resolved once at construction time and reused
// for path containment validation on every write operation.
func NewFileWriter(dryRun bool, opts ...WriterOption) (FileWriter, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("cannot determine working directory: %w", err)
	}
	cfg := &writerConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	w := Write{
		mx:  sync.Mutex{},
		fs:  setFileWriter(dryRun, cfg),
		cwd: cwd,
	}
	return &w, nil
}

// WriteFile thin wrapper to decouple dependency of write file.
// Validates that the output path stays within the cached working directory
// to prevent path traversal via malicious generator templates.
func (w *Write) WriteFile(name string, data []byte, perm fs.FileMode) error {
	w.mx.Lock()
	defer w.mx.Unlock()

	if err := fileutil.ValidatePathContainment(w.cwd, name); err != nil {
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

	if err := fileutil.ValidatePathContainment(w.cwd, name); err != nil {
		return fmt.Errorf("path safety check failed: %w", err)
	}

	file, err := w.fs.OpenFile(name, os.O_APPEND|os.O_WRONLY, types.FileMode)
	if err != nil {
		slog.Error("cannot open file", "file", name, "error", err)
		return err
	}
	if file != nil {
		defer func() {
			if err := file.Close(); err != nil {
				slog.Error("cannot close file", "file", file.Name(), "error", err) //nolint:gosec // G706: slog structured logging; log injection not a concern in a CLI tool
			}
		}()
	}
	if _, err := w.fs.Write(file, data); err != nil {
		slog.Error("cannot append data to file", "file", file, "error", err) //nolint:gosec // G706: slog structured logging; log injection not a concern in a CLI tool
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

	if err := fileutil.ValidatePathContainment(w.cwd, name); err != nil {
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
	if err := w.fs.WriteFile(name, formattedOutput, types.FileMode); err != nil {
		slog.Error("cannot write to file", "file", name, "error", err)
		return err
	}
	return nil
}

// setFileWriter returns a fileReadWrite implementation based on dryRun and opts.
// When dryRun is true and cfg.out is non-nil, an enhanced diff writer is returned.
// When dryRun is true and cfg.out is nil, a simple slog-based logger is returned.
func setFileWriter(dryrun bool, cfg *writerConfig) fileReadWrite {
	if dryrun {
		if cfg.out != nil {
			return &fileDiff{
				out:   cfg.out,
				in:    cfg.in,
				isTTY: cfg.isTTY,
			}
		}
		return &fileLog{}
	}
	return &fileWrite{}
}
