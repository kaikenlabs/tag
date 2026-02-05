package writer

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

type Write struct {
	mx sync.RWMutex
	fs fileReadWrite
}

type fileWrite struct {
	DryRun bool
}

var _ fileReadWrite = (*fileWrite)(nil)

func (f *fileWrite) WriteFile(name string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o750); err != nil {
		return err
	}
	return os.WriteFile(name, data, perm)
}

func (f *fileWrite) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(filepath.Clean(name))
}

func (f *fileWrite) OpenFile(name string, flag int, perm fs.FileMode) (*os.File, error) {
	// #nosec G304
	return os.OpenFile(filepath.Clean(name), flag, perm)
}

func (f *fileWrite) Write(file *os.File, b []byte) (n int, err error) {
	defer func() {
		if err := file.Close(); err != nil {
			slog.Error("cannot close file", "file", file.Name(), "error", err)
		}
	}()
	return file.Write(b)
}
