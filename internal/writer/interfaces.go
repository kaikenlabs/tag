package writer

import (
	"io/fs"
	"os"
)

type fileReadWrite interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm os.FileMode) error
	OpenFile(name string, flag int, perm fs.FileMode) (*os.File, error)
	Write(file *os.File, b []byte) (n int, err error)
}
