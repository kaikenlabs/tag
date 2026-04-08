//go:generate moq -out mocks.go -stub . fileReadWrite

package writer

import (
	"io/fs"
	"os"
)

// FileWriter defines the public interface for file operations used by the
// code generation engine. It abstracts create, append, inject, and merge operations.
type FileWriter interface {
	WriteFile(name string, data []byte, perm fs.FileMode) error
	AppendFile(name string, data []byte) error
	InjectIntoFile(name string, data []byte, inject Inject) error
	MergeOpenAPIFile(name string, fragment []byte, opts OpenAPIMergeOptions) (OpenAPIMergeResult, error)
}

// OpenAPIMergeOptions controls OpenAPI merge behavior.
type OpenAPIMergeOptions struct {
	ValidateResult bool // run kin-openapi validation after merge
}

// OpenAPIMergeResult describes what the OpenAPI merge did.
type OpenAPIMergeResult struct {
	Changed        bool
	AddedPaths     []string
	AddedSchemas   []string
	SkippedPaths   []string
	SkippedSchemas []string
}

// Compile-time check: *Write satisfies FileWriter.
var _ FileWriter = (*Write)(nil)

type fileReadWrite interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm os.FileMode) error
	OpenFile(name string, flag int, perm fs.FileMode) (*os.File, error)
	Write(file *os.File, b []byte) (n int, err error)
}
