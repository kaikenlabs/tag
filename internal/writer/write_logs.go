package writer

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

type fileLog struct{}

var _ fileReadWrite = (*fileLog)(nil)

func (f *fileLog) WriteFile(name string, data []byte, perm os.FileMode) error {
	slog.Info("logging to console", "name", name, "data", fmt.Sprintf("\n%s", string(data)))
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
	slog.Info("logging to console", "file", file.Name(), "data", fmt.Sprintf("\n%s", string(b)))
	return 0, nil
}
