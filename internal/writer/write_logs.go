package writer

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
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
	slog.Info("logging to console", "file", file.Name(), "data", "\n"+string(b))
	return len(b), nil
}
