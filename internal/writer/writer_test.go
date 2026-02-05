package writer

import (
	"fmt"
	"io/fs"
	"os"
	"sync"
	"testing"

	"github.com/code-gorilla-au/odize"
)

func TestWrite_WriteFile_should_not_return_error(t *testing.T) {
	mockWr := fileReadWriteMock{}

	w := Write{
		mx: sync.RWMutex{},
		fs: &mockWr,
	}
	err := w.WriteFile("blood", []byte("hello world"), 0o700)
	odize.AssertNoError(t, err)
}

func TestWrite_WriteFile_write_error_should_return_error(t *testing.T) {
	mockWr := fileReadWriteMock{}
	mockWr.WriteFileFunc = func(name string, data []byte, perm fs.FileMode) error {
		return fmt.Errorf("error")
	}
	w := Write{
		mx: sync.RWMutex{},
		fs: &mockWr,
	}
	err := w.WriteFile("blood", []byte("hello world"), 0o700)
	odize.AssertError(t, err)
}

func TestWrite_AppendFile_should_return_ok(t *testing.T) {
	mockWr := fileReadWriteMock{}

	w := Write{
		mx: sync.RWMutex{},
		fs: &mockWr,
	}
	err := w.AppendFile("blood", []byte("hello world"))
	odize.AssertNoError(t, err)
}

func TestWrite_AppendFile_open_file_error_should_return_error(t *testing.T) {
	mockWr := fileReadWriteMock{}
	mockWr.OpenFileFunc = func(name string, flag int, perm fs.FileMode) (*os.File, error) {
		return nil, fmt.Errorf("error")
	}

	w := Write{
		mx: sync.RWMutex{},
		fs: &mockWr,
	}
	err := w.AppendFile("blood", []byte("hello world"))
	odize.AssertError(t, err)
}

func TestWrite_AppendFile_write_file_error_should_return_error(t *testing.T) {
	mockWr := fileReadWriteMock{}
	mockWr.WriteFunc = func(file *os.File, b []byte) (int, error) {
		return 0, fmt.Errorf("error")
	}

	w := Write{
		mx: sync.RWMutex{},
		fs: &mockWr,
	}
	err := w.AppendFile("blood", []byte("hello world"))
	odize.AssertError(t, err)
}

func TestWrite_InjectIntoFile_inject_after_should_return_no_error(t *testing.T) {
	mockWr := fileReadWriteMock{}
	mockWr.ReadFileFunc = func(name string) ([]byte, error) {
		return []byte(" // after "), nil
	}

	w := Write{
		mx: sync.RWMutex{},
		fs: &mockWr,
	}
	err := w.InjectIntoFile("blood", []byte("hello world"), Inject{
		Matcher: "// after",
		Clause:  InjectAfter,
	})
	odize.AssertNoError(t, err)
}

func TestWrite_InjectIntoFile_read_file_error_should_return_error(t *testing.T) {
	mockWr := fileReadWriteMock{}
	mockWr.ReadFileFunc = func(name string) ([]byte, error) {
		return nil, fmt.Errorf("error")
	}

	w := Write{
		mx: sync.RWMutex{},
		fs: &mockWr,
	}
	err := w.InjectIntoFile("blood", []byte("hello world"), Inject{
		Matcher: "// after",
		Clause:  InjectAfter,
	})
	odize.AssertError(t, err)
}

func TestWrite_InjectIntoFile_inject_before_should_return_no_error(t *testing.T) {
	mockWr := fileReadWriteMock{}
	mockWr.ReadFileFunc = func(name string) ([]byte, error) {
		return []byte(" // before "), nil
	}

	w := Write{
		mx: sync.RWMutex{},
		fs: &mockWr,
	}
	err := w.InjectIntoFile("blood", []byte("hello world"), Inject{
		Matcher: "// before",
		Clause:  InjectBefore,
	})
	odize.AssertNoError(t, err)
}

func TestWrite_InjectIntoFile_missing_token_should_return_error(t *testing.T) {
	mockWr := fileReadWriteMock{}
	mockWr.ReadFileFunc = func(name string) ([]byte, error) {
		return []byte(" // "), nil
	}

	w := Write{
		mx: sync.RWMutex{},
		fs: &mockWr,
	}
	err := w.InjectIntoFile("blood", []byte("hello world"), Inject{
		Matcher: "// before",
		Clause:  InjectBefore,
	})
	odize.AssertError(t, err)
}

func TestWrite_InjectIntoFile_write_file_error_should_return_error(t *testing.T) {
	mockWr := fileReadWriteMock{}
	mockWr.ReadFileFunc = func(name string) ([]byte, error) {
		return []byte(" // before "), nil
	}
	mockWr.WriteFileFunc = func(name string, data []byte, perm fs.FileMode) error {
		return fmt.Errorf("error")
	}

	w := Write{
		mx: sync.RWMutex{},
		fs: &mockWr,
	}
	err := w.InjectIntoFile("blood", []byte("hello world"), Inject{
		Matcher: "// before",
		Clause:  InjectBefore,
	})
	odize.AssertError(t, err)
}
