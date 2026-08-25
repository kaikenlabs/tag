package writer

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/fileutil"
	"github.com/kaikenlabs/tag/internal/types"
)

// testCwd returns the current working directory for test helpers.
func testCwd(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	require.NoError(t, err)
	return cwd
}

func TestUT_NewFileWriter_CachesGetwd(t *testing.T) {
	fw, err := NewFileWriter(false)
	require.NoError(t, err)

	// Verify the writer is functional by checking it implements the interface
	require.NotNil(t, fw)

	// Verify the internal cwd is set correctly
	w, ok := fw.(*Write)
	require.True(t, ok)
	assert.NotEmpty(t, w.cwd)

	cwd, err := os.Getwd()
	require.NoError(t, err)
	assert.Equal(t, cwd, w.cwd)
}

func TestUT_Write_WriteFile_NoError(t *testing.T) {
	mockWr := fileReadWriteMock{}

	w := Write{
		mx:  sync.Mutex{},
		fs:  &mockWr,
		cwd: testCwd(t),
	}
	err := w.WriteFile("blood", []byte("hello world"), 0o700)
	require.NoError(t, err)
}

func TestWrite_WriteFile_write_error_should_return_error(t *testing.T) {
	mockWr := fileReadWriteMock{}
	mockWr.WriteFileFunc = func(name string, data []byte, perm fs.FileMode) error {
		return errors.New("error")
	}
	w := Write{
		mx:  sync.Mutex{},
		fs:  &mockWr,
		cwd: testCwd(t),
	}
	err := w.WriteFile("blood", []byte("hello world"), 0o700)
	assert.Error(t, err)
}

func TestUT_WriteFile_PathContainment(t *testing.T) {
	cwd := testCwd(t)

	t.Run("rejects paths outside working directory", func(t *testing.T) {
		mockWr := fileReadWriteMock{}
		writeCalled := false
		mockWr.WriteFileFunc = func(name string, data []byte, perm fs.FileMode) error {
			writeCalled = true
			return nil
		}

		w := Write{
			mx:  sync.Mutex{},
			fs:  &mockWr,
			cwd: cwd,
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
			mx:  sync.Mutex{},
			fs:  &mockWr,
			cwd: cwd,
		}

		err := w.WriteFile("../../etc/passwd", []byte("malicious"), 0o750)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path safety check failed")
		assert.False(t, writeCalled, "file should not have been written")
	})

	t.Run("allows paths within working directory", func(t *testing.T) {
		mockWr := fileReadWriteMock{}

		w := Write{
			mx:  sync.Mutex{},
			fs:  &mockWr,
			cwd: cwd,
		}

		err := w.WriteFile("mypackage/output.go", []byte("package mypackage"), 0o750)
		require.NoError(t, err)
	})
}

func TestUT_PathContainmentIntegration(t *testing.T) {
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
			err := fileutil.ValidatePathContainment(tt.baseDir, tt.path)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUT_AppendFile_PathContainment(t *testing.T) {
	cwd := testCwd(t)

	t.Run("rejects paths outside working directory", func(t *testing.T) {
		mockWr := fileReadWriteMock{}
		openCalled := false
		mockWr.OpenFileFunc = func(name string, flag int, perm fs.FileMode) (*os.File, error) {
			openCalled = true
			return nil, nil //nolint:nilnil // mock function that should never be called
		}

		w := Write{
			mx:  sync.Mutex{},
			fs:  &mockWr,
			cwd: cwd,
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
			return nil, nil //nolint:nilnil // mock function that should never be called
		}

		w := Write{
			mx:  sync.Mutex{},
			fs:  &mockWr,
			cwd: cwd,
		}

		err := w.AppendFile("../../etc/passwd", []byte("malicious"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path safety check failed")
		assert.False(t, openCalled, "file should not have been opened")
	})

	t.Run("allows paths within working directory", func(t *testing.T) {
		mockWr := fileReadWriteMock{}

		w := Write{
			mx:  sync.Mutex{},
			fs:  &mockWr,
			cwd: cwd,
		}

		err := w.AppendFile("mypackage/output.go", []byte("data"))
		require.NoError(t, err)
	})
}

func TestUT_InjectIntoFile_PathContainment(t *testing.T) {
	cwd := testCwd(t)

	t.Run("rejects paths outside working directory", func(t *testing.T) {
		mockWr := fileReadWriteMock{}
		readCalled := false
		mockWr.ReadFileFunc = func(name string) ([]byte, error) {
			readCalled = true
			return nil, nil
		}

		w := Write{
			mx:  sync.Mutex{},
			fs:  &mockWr,
			cwd: cwd,
		}

		err := w.InjectIntoFile("/etc/cron.d/backdoor", []byte("malicious"), Inject{
			Matcher: "// marker",
			Clause:  types.InjectAfter,
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
			mx:  sync.Mutex{},
			fs:  &mockWr,
			cwd: cwd,
		}

		err := w.InjectIntoFile("../../etc/passwd", []byte("malicious"), Inject{
			Matcher: "// marker",
			Clause:  types.InjectAfter,
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
			mx:  sync.Mutex{},
			fs:  &mockWr,
			cwd: cwd,
		}

		err := w.InjectIntoFile("mypackage/output.go", []byte("data"), Inject{
			Matcher: "// marker",
			Clause:  types.InjectAfter,
		})
		require.NoError(t, err)
	})
}

func TestWrite_AppendFile_should_return_ok(t *testing.T) {
	mockWr := fileReadWriteMock{}

	w := Write{
		mx:  sync.Mutex{},
		fs:  &mockWr,
		cwd: testCwd(t),
	}
	err := w.AppendFile("blood", []byte("hello world"))
	require.NoError(t, err)
}

func TestWrite_AppendFile_open_file_error_should_return_error(t *testing.T) {
	mockWr := fileReadWriteMock{}
	mockWr.OpenFileFunc = func(name string, flag int, perm fs.FileMode) (*os.File, error) {
		return nil, errors.New("error")
	}

	w := Write{
		mx:  sync.Mutex{},
		fs:  &mockWr,
		cwd: testCwd(t),
	}
	err := w.AppendFile("blood", []byte("hello world"))
	assert.Error(t, err)
}

func TestWrite_AppendFile_write_file_error_should_return_error(t *testing.T) {
	mockWr := fileReadWriteMock{}
	mockWr.WriteFunc = func(file *os.File, b []byte) (int, error) {
		return 0, errors.New("error")
	}

	w := Write{
		mx:  sync.Mutex{},
		fs:  &mockWr,
		cwd: testCwd(t),
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
		mx:  sync.Mutex{},
		fs:  &mockWr,
		cwd: testCwd(t),
	}
	err := w.InjectIntoFile("blood", []byte("hello world"), Inject{
		Matcher: "// after",
		Clause:  types.InjectAfter,
	})
	require.NoError(t, err)
}

func TestWrite_InjectIntoFile_read_file_error_should_return_error(t *testing.T) {
	mockWr := fileReadWriteMock{}
	mockWr.ReadFileFunc = func(name string) ([]byte, error) {
		return nil, errors.New("error")
	}

	w := Write{
		mx:  sync.Mutex{},
		fs:  &mockWr,
		cwd: testCwd(t),
	}
	err := w.InjectIntoFile("blood", []byte("hello world"), Inject{
		Matcher: "// after",
		Clause:  types.InjectAfter,
	})
	assert.Error(t, err)
}

func TestWrite_InjectIntoFile_inject_before_should_return_no_error(t *testing.T) {
	mockWr := fileReadWriteMock{}
	mockWr.ReadFileFunc = func(name string) ([]byte, error) {
		return []byte(" // before "), nil
	}

	w := Write{
		mx:  sync.Mutex{},
		fs:  &mockWr,
		cwd: testCwd(t),
	}
	err := w.InjectIntoFile("blood", []byte("hello world"), Inject{
		Matcher: "// before",
		Clause:  types.InjectBefore,
	})
	require.NoError(t, err)
}

func TestWrite_InjectIntoFile_missing_token_should_return_error(t *testing.T) {
	mockWr := fileReadWriteMock{}
	mockWr.ReadFileFunc = func(name string) ([]byte, error) {
		return []byte(" // "), nil
	}

	w := Write{
		mx:  sync.Mutex{},
		fs:  &mockWr,
		cwd: testCwd(t),
	}
	err := w.InjectIntoFile("blood", []byte("hello world"), Inject{
		Matcher: "// before",
		Clause:  types.InjectBefore,
	})
	assert.Error(t, err)
}

func TestWrite_InjectIntoFile_write_file_error_should_return_error(t *testing.T) {
	mockWr := fileReadWriteMock{}
	mockWr.ReadFileFunc = func(name string) ([]byte, error) {
		return []byte(" // before "), nil
	}
	mockWr.WriteFileFunc = func(name string, data []byte, perm fs.FileMode) error {
		return errors.New("error")
	}

	w := Write{
		mx:  sync.Mutex{},
		fs:  &mockWr,
		cwd: testCwd(t),
	}
	err := w.InjectIntoFile("blood", []byte("hello world"), Inject{
		Matcher: "// before",
		Clause:  types.InjectBefore,
	})
	assert.Error(t, err)
}

func TestUT_WriteFile_RelativeNameUnderSymlinkedCwd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("os.Getwd does not consult PWD on Windows, so a symlinked cwd cannot be reproduced")
	}

	realDir := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(realDir, link))

	t.Chdir(link)

	cwd := testCwd(t)
	resolved, err := filepath.EvalSymlinks(cwd)
	require.NoError(t, err)
	require.NotEqual(t, resolved, cwd, "fixture did not reproduce a symlinked cwd")

	written := false
	mockWr := fileReadWriteMock{}
	mockWr.WriteFileFunc = func(name string, data []byte, perm fs.FileMode) error {
		written = true
		return nil
	}

	w := Write{fs: &mockWr, cwd: cwd}
	require.NoError(t, w.WriteFile("blood", []byte("hello world"), 0o700))
	assert.True(t, written, "write was blocked by the containment check")
}

// TestUT_WriteFile_DanglingSymlinkTargetIsRefused proves that #418's
// fail-closed ValidatePathContainment verdict is what authorizes (or
// refuses) a real write: a name that is a dangling symlink pointing outside
// cwd used to resolve with a nil error, and WriteFile then genuinely wrote
// through the symlink into the outside file. The PRIMARY oracle is that the
// outside file never gets created — os.WriteFile follows a symlink by
// default, so on unfixed code this test creates and populates a file the
// containment check was supposed to block.
func TestUT_WriteFile_DanglingSymlinkTargetIsRefused(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	base, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	outside, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	evilLink := filepath.Join(base, "evil.txt")
	outsideTarget := filepath.Join(outside, "pwned.txt")
	require.NoError(t, os.Symlink(outsideTarget, evilLink))

	w := Write{fs: &fileWrite{}, cwd: base}
	writeErr := w.WriteFile(evilLink, []byte("payload"), 0o644)

	assert.NoFileExists(t, outsideTarget, "write must not escape cwd through a dangling symlink")
	assert.Error(t, writeErr)
}
