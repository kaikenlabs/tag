//go:generate moq -out mocks.go -stub . fileReadWrite

package writer

import (
	"io/fs"
	"os"
)

// FileWriter defines the public interface for file operations used by the
// code generation engine. It abstracts create, append, and inject operations.
type FileWriter interface {
	WriteFile(name string, data []byte, perm fs.FileMode) error
	AppendFile(name string, data []byte) error
	InjectIntoFile(name string, data []byte, inject Inject) error
}

// Compile-time check: *Write satisfies FileWriter.
var _ FileWriter = (*Write)(nil)

type fileReadWrite interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm os.FileMode) error
	OpenFile(name string, flag int, perm fs.FileMode) (*os.File, error)
	Write(file *os.File, b []byte) (n int, err error)
}
