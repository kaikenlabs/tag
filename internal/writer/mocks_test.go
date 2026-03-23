package writer

import (
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_FileReadWriteMock_OpenFile_WithFunc(t *testing.T) {
	t.Parallel()
	tmpFile, err := os.CreateTemp(t.TempDir(), "mock-*")
	require.NoError(t, err)
	defer tmpFile.Close()

	mock := &fileReadWriteMock{
		OpenFileFunc: func(name string, flag int, perm fs.FileMode) (*os.File, error) {
			return tmpFile, nil
		},
	}

	f, err := mock.OpenFile("test.txt", os.O_RDONLY, 0o644)
	require.NoError(t, err)
	assert.Equal(t, tmpFile, f)

	calls := mock.OpenFileCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "test.txt", calls[0].Name)
	assert.Equal(t, os.O_RDONLY, calls[0].Flag)
	assert.Equal(t, fs.FileMode(0o644), calls[0].Perm)
}

func TestUT_FileReadWriteMock_OpenFile_NilFunc(t *testing.T) {
	t.Parallel()
	mock := &fileReadWriteMock{}

	f, err := mock.OpenFile("test.txt", os.O_RDONLY, 0o644)
	assert.NoError(t, err)
	assert.Nil(t, f)
	assert.Len(t, mock.OpenFileCalls(), 1)
}

func TestUT_FileReadWriteMock_OpenFile_Error(t *testing.T) {
	t.Parallel()
	expectedErr := errors.New("open failed")
	mock := &fileReadWriteMock{
		OpenFileFunc: func(_ string, _ int, _ fs.FileMode) (*os.File, error) {
			return nil, expectedErr
		},
	}

	f, err := mock.OpenFile("bad.txt", os.O_RDONLY, 0o644)
	assert.Nil(t, f)
	assert.ErrorIs(t, err, expectedErr)
}

func TestUT_FileReadWriteMock_ReadFile_WithFunc(t *testing.T) {
	t.Parallel()
	expected := []byte("file content")
	mock := &fileReadWriteMock{
		ReadFileFunc: func(name string) ([]byte, error) {
			return expected, nil
		},
	}

	data, err := mock.ReadFile("input.txt")
	require.NoError(t, err)
	assert.Equal(t, expected, data)

	calls := mock.ReadFileCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "input.txt", calls[0].Name)
}

func TestUT_FileReadWriteMock_ReadFile_NilFunc(t *testing.T) {
	t.Parallel()
	mock := &fileReadWriteMock{}

	data, err := mock.ReadFile("any.txt")
	assert.NoError(t, err)
	assert.Nil(t, data)
	assert.Len(t, mock.ReadFileCalls(), 1)
}

func TestUT_FileReadWriteMock_ReadFile_Error(t *testing.T) {
	t.Parallel()
	expectedErr := errors.New("read failed")
	mock := &fileReadWriteMock{
		ReadFileFunc: func(_ string) ([]byte, error) {
			return nil, expectedErr
		},
	}

	data, err := mock.ReadFile("missing.txt")
	assert.Nil(t, data)
	assert.ErrorIs(t, err, expectedErr)
}

func TestUT_FileReadWriteMock_Write_WithFunc(t *testing.T) {
	t.Parallel()
	mock := &fileReadWriteMock{
		WriteFunc: func(_ *os.File, b []byte) (int, error) {
			return len(b), nil
		},
	}

	n, err := mock.Write(nil, []byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)

	calls := mock.WriteCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, []byte("hello"), calls[0].B)
}

func TestUT_FileReadWriteMock_Write_NilFunc(t *testing.T) {
	t.Parallel()
	mock := &fileReadWriteMock{}

	n, err := mock.Write(nil, []byte("data"))
	assert.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Len(t, mock.WriteCalls(), 1)
}

func TestUT_FileReadWriteMock_Write_Error(t *testing.T) {
	t.Parallel()
	expectedErr := errors.New("write failed")
	mock := &fileReadWriteMock{
		WriteFunc: func(_ *os.File, _ []byte) (int, error) {
			return 0, expectedErr
		},
	}

	n, err := mock.Write(nil, []byte("data"))
	assert.Equal(t, 0, n)
	assert.ErrorIs(t, err, expectedErr)
}

func TestUT_FileReadWriteMock_WriteFile_WithFunc(t *testing.T) {
	t.Parallel()
	mock := &fileReadWriteMock{
		WriteFileFunc: func(name string, data []byte, perm fs.FileMode) error {
			return nil
		},
	}

	err := mock.WriteFile("output.txt", []byte("content"), 0o644)
	require.NoError(t, err)

	calls := mock.WriteFileCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "output.txt", calls[0].Name)
	assert.Equal(t, []byte("content"), calls[0].Data)
	assert.Equal(t, fs.FileMode(0o644), calls[0].Perm)
}

func TestUT_FileReadWriteMock_WriteFile_NilFunc(t *testing.T) {
	t.Parallel()
	mock := &fileReadWriteMock{}

	err := mock.WriteFile("output.txt", []byte("data"), 0o644)
	assert.NoError(t, err)
	assert.Len(t, mock.WriteFileCalls(), 1)
}

func TestUT_FileReadWriteMock_WriteFile_Error(t *testing.T) {
	t.Parallel()
	expectedErr := errors.New("write file failed")
	mock := &fileReadWriteMock{
		WriteFileFunc: func(_ string, _ []byte, _ fs.FileMode) error {
			return expectedErr
		},
	}

	err := mock.WriteFile("bad.txt", []byte("data"), 0o644)
	assert.ErrorIs(t, err, expectedErr)
}

func TestUT_FileReadWriteMock_MultipleCalls_TrackedCorrectly(t *testing.T) {
	t.Parallel()
	mock := &fileReadWriteMock{
		ReadFileFunc: func(name string) ([]byte, error) {
			return []byte(name), nil
		},
	}

	_, _ = mock.ReadFile("a.txt")
	_, _ = mock.ReadFile("b.txt")
	_, _ = mock.ReadFile("c.txt")

	calls := mock.ReadFileCalls()
	require.Len(t, calls, 3)
	assert.Equal(t, "a.txt", calls[0].Name)
	assert.Equal(t, "b.txt", calls[1].Name)
	assert.Equal(t, "c.txt", calls[2].Name)
}

func TestUT_FileReadWriteMock_ImplementsInterface(t *testing.T) {
	t.Parallel()
	var _ fileReadWrite = &fileReadWriteMock{}
}
