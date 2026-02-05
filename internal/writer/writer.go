package writer

import (
	"io/fs"
	"log/slog"
	"os"
	"sync"
)

const (
	FileModeOwnerRWX = 0o644
)

// New - create a new writer
func New(dryRun bool) Write {
	return Write{
		mx: sync.RWMutex{},
		fs: setFileWriter(dryRun),
	}
}

// WriteFile thin wrapper to decouple dependency of write file
func (w *Write) WriteFile(name string, data []byte, perm fs.FileMode) error {
	w.mx.Lock()
	defer w.mx.Unlock()
	return w.fs.WriteFile(name, data, perm)
}

// AppendFile - append data to the end of file
func (w *Write) AppendFile(name string, data []byte) error {
	w.mx.Lock()
	defer w.mx.Unlock()
	file, err := w.fs.OpenFile(name, os.O_APPEND|os.O_WRONLY, FileModeOwnerRWX)
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
	if err := w.fs.WriteFile(name, formatedOutput, FileModeOwnerRWX); err != nil {
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
