package writer

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrite_WriteFile_should_not_return_error(t *testing.T) {
	mockWr := fileReadWriteMock{}

	w := Write{
		mx: sync.Mutex{},
		fs: &mockWr,
	}
	err := w.WriteFile("blood", []byte("hello world"), 0o700)
	require.NoError(t, err)
}

func TestWrite_WriteFile_write_error_should_return_error(t *testing.T) {
	mockWr := fileReadWriteMock{}
	mockWr.WriteFileFunc = func(name string, data []byte, perm fs.FileMode) error {
		return fmt.Errorf("error")
	}
	w := Write{
		mx: sync.Mutex{},
		fs: &mockWr,
	}
	err := w.WriteFile("blood", []byte("hello world"), 0o700)
	assert.Error(t, err)
}

func TestUT_WriteFile_PathContainment(t *testing.T) {
	t.Run("rejects paths outside working directory", func(t *testing.T) {
		mockWr := fileReadWriteMock{}
		writeCalled := false
		mockWr.WriteFileFunc = func(name string, data []byte, perm fs.FileMode) error {
			writeCalled = true
			return nil
		}

		w := Write{
			mx: sync.Mutex{},
			fs: &mockWr,
		}

		err := w.WriteFile("/etc/cron.d/backdoor", []byte("malicious"), 0o750)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path safety check failed")
		assert.False(t, writeCalled, "file should not have been written")
	})

	t.Run("rejects dotdot traversal", func(t *testing.T) {
		mockWr := fileReadWriteMock{}
		writeCalled := false
		mockWr.WriteFileFunc = func(name string, data []byte, perm fs.FileMode) error {
			writeCalled = true
			return nil
		}

		w := Write{
			mx: sync.Mutex{},
			fs: &mockWr,
		}

		err := w.WriteFile("../../etc/passwd", []byte("malicious"), 0o750)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path safety check failed")
		assert.False(t, writeCalled, "file should not have been written")
	})

	t.Run("allows paths within working directory", func(t *testing.T) {
		mockWr := fileReadWriteMock{}

		w := Write{
			mx: sync.Mutex{},
			fs: &mockWr,
		}

		err := w.WriteFile("mypackage/output.go", []byte("package mypackage"), 0o750)
		require.NoError(t, err)
	})
}

func TestUT_ValidatePathWithinDir(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		path    string
		baseDir string
		wantErr bool
	}{
		{
			name:    "path within base",
			path:    filepath.Join(tmpDir, "sub", "file.go"),
			baseDir: tmpDir,
			wantErr: false,
		},
		{
			name:    "path escapes base",
			path:    filepath.Join(tmpDir, "..", "escape"),
			baseDir: tmpDir,
			wantErr: true,
		},
		{
			name:    "absolute path outside base",
			path:    "/etc/passwd",
			baseDir: tmpDir,
			wantErr: true,
		},
		{
			name:    "path equals base",
			path:    tmpDir,
			baseDir: tmpDir,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePathWithinDir(tt.path, tt.baseDir)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUT_AppendFile_PathContainment(t *testing.T) {
	t.Run("rejects paths outside working directory", func(t *testing.T) {
		mockWr := fileReadWriteMock{}
		openCalled := false
		mockWr.OpenFileFunc = func(name string, flag int, perm fs.FileMode) (*os.File, error) {
			openCalled = true
			return nil, nil
		}

		w := Write{
			mx: sync.Mutex{},
			fs: &mockWr,
		}

		err := w.AppendFile("/etc/cron.d/backdoor", []byte("malicious"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path safety check failed")
		assert.False(t, openCalled, "file should not have been opened")
	})

	t.Run("rejects dotdot traversal", func(t *testing.T) {
		mockWr := fileReadWriteMock{}
		openCalled := false
		mockWr.OpenFileFunc = func(name string, flag int, perm fs.FileMode) (*os.File, error) {
			openCalled = true
			return nil, nil
		}

		w := Write{
			mx: sync.Mutex{},
			fs: &mockWr,
		}

		err := w.AppendFile("../../etc/passwd", []byte("malicious"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path safety check failed")
		assert.False(t, openCalled, "file should not have been opened")
	})

	t.Run("allows paths within working directory", func(t *testing.T) {
		mockWr := fileReadWriteMock{}

		w := Write{
			mx: sync.Mutex{},
			fs: &mockWr,
		}

		err := w.AppendFile("mypackage/output.go", []byte("data"))
		require.NoError(t, err)
	})
}

func TestUT_InjectIntoFile_PathContainment(t *testing.T) {
	t.Run("rejects paths outside working directory", func(t *testing.T) {
		mockWr := fileReadWriteMock{}
		readCalled := false
		mockWr.ReadFileFunc = func(name string) ([]byte, error) {
			readCalled = true
			return nil, nil
		}

		w := Write{
			mx: sync.Mutex{},
			fs: &mockWr,
		}

		err := w.InjectIntoFile("/etc/cron.d/backdoor", []byte("malicious"), Inject{
			Matcher: "// marker",
			Clause:  InjectAfter,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path safety check failed")
		assert.False(t, readCalled, "file should not have been read")
	})

	t.Run("rejects dotdot traversal", func(t *testing.T) {
		mockWr := fileReadWriteMock{}
		readCalled := false
		mockWr.ReadFileFunc = func(name string) ([]byte, error) {
			readCalled = true
			return nil, nil
		}

		w := Write{
			mx: sync.Mutex{},
			fs: &mockWr,
		}

		err := w.InjectIntoFile("../../etc/passwd", []byte("malicious"), Inject{
			Matcher: "// marker",
			Clause:  InjectAfter,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path safety check failed")
		assert.False(t, readCalled, "file should not have been read")
	})

	t.Run("allows paths within working directory", func(t *testing.T) {
		mockWr := fileReadWriteMock{}
		mockWr.ReadFileFunc = func(name string) ([]byte, error) {
			return []byte(" // marker "), nil
		}

		w := Write{
			mx: sync.Mutex{},
			fs: &mockWr,
		}

		err := w.InjectIntoFile("mypackage/output.go", []byte("data"), Inject{
			Matcher: "// marker",
			Clause:  InjectAfter,
		})
		require.NoError(t, err)
	})
}

func TestWrite_AppendFile_should_return_ok(t *testing.T) {
	mockWr := fileReadWriteMock{}

	w := Write{
		mx: sync.Mutex{},
		fs: &mockWr,
	}
	err := w.AppendFile("blood", []byte("hello world"))
	require.NoError(t, err)
}

func TestWrite_AppendFile_open_file_error_should_return_error(t *testing.T) {
	mockWr := fileReadWriteMock{}
	mockWr.OpenFileFunc = func(name string, flag int, perm fs.FileMode) (*os.File, error) {
		return nil, fmt.Errorf("error")
	}

	w := Write{
		mx: sync.Mutex{},
		fs: &mockWr,
	}
	err := w.AppendFile("blood", []byte("hello world"))
	assert.Error(t, err)
}

func TestWrite_AppendFile_write_file_error_should_return_error(t *testing.T) {
	mockWr := fileReadWriteMock{}
	mockWr.WriteFunc = func(file *os.File, b []byte) (int, error) {
		return 0, fmt.Errorf("error")
	}

	w := Write{
		mx: sync.Mutex{},
		fs: &mockWr,
	}
	err := w.AppendFile("blood", []byte("hello world"))
	assert.Error(t, err)
}

func TestWrite_InjectIntoFile_inject_after_should_return_no_error(t *testing.T) {
	mockWr := fileReadWriteMock{}
	mockWr.ReadFileFunc = func(name string) ([]byte, error) {
		return []byte(" // after "), nil
	}

	w := Write{
		mx: sync.Mutex{},
		fs: &mockWr,
	}
	err := w.InjectIntoFile("blood", []byte("hello world"), Inject{
		Matcher: "// after",
		Clause:  InjectAfter,
	})
	require.NoError(t, err)
}

func TestWrite_InjectIntoFile_read_file_error_should_return_error(t *testing.T) {
	mockWr := fileReadWriteMock{}
	mockWr.ReadFileFunc = func(name string) ([]byte, error) {
		return nil, fmt.Errorf("error")
	}

	w := Write{
		mx: sync.Mutex{},
		fs: &mockWr,
	}
	err := w.InjectIntoFile("blood", []byte("hello world"), Inject{
		Matcher: "// after",
		Clause:  InjectAfter,
	})
	assert.Error(t, err)
}

func TestWrite_InjectIntoFile_inject_before_should_return_no_error(t *testing.T) {
	mockWr := fileReadWriteMock{}
	mockWr.ReadFileFunc = func(name string) ([]byte, error) {
		return []byte(" // before "), nil
	}

	w := Write{
		mx: sync.Mutex{},
		fs: &mockWr,
	}
	err := w.InjectIntoFile("blood", []byte("hello world"), Inject{
		Matcher: "// before",
		Clause:  InjectBefore,
	})
	require.NoError(t, err)
}

func TestWrite_InjectIntoFile_missing_token_should_return_error(t *testing.T) {
	mockWr := fileReadWriteMock{}
	mockWr.ReadFileFunc = func(name string) ([]byte, error) {
		return []byte(" // "), nil
	}

	w := Write{
		mx: sync.Mutex{},
		fs: &mockWr,
	}
	err := w.InjectIntoFile("blood", []byte("hello world"), Inject{
		Matcher: "// before",
		Clause:  InjectBefore,
	})
	assert.Error(t, err)
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
		mx: sync.Mutex{},
		fs: &mockWr,
	}
	err := w.InjectIntoFile("blood", []byte("hello world"), Inject{
		Matcher: "// before",
		Clause:  InjectBefore,
	})
	assert.Error(t, err)
}
