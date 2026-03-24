package history

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUT_ConflictError_Error(t *testing.T) {
	t.Parallel()
	err := &ConflictError{Paths: []string{"file1.go", "file2.go"}}
	msg := err.Error()
	assert.Contains(t, msg, "files modified after generation")
	assert.Contains(t, msg, "file1.go")
	assert.Contains(t, msg, "file2.go")
}

func TestUT_ConflictError_Unwrap(t *testing.T) {
	t.Parallel()
	err := &ConflictError{Paths: []string{"a.go"}}
	assert.ErrorIs(t, err, ErrConflict)
	assert.Equal(t, ErrConflict, err.Unwrap())
}

func TestUT_ConflictError_ErrorIs(t *testing.T) {
	t.Parallel()
	err := &ConflictError{Paths: []string{"a.go"}}
	assert.True(t, errors.Is(err, ErrConflict))
	assert.False(t, errors.Is(err, ErrNotFound))
}

func TestUT_ConflictError_EmptyPaths(t *testing.T) {
	t.Parallel()
	err := &ConflictError{Paths: nil}
	msg := err.Error()
	assert.Contains(t, msg, "files modified after generation")
}

func TestUT_SentinelErrors_Defined(t *testing.T) {
	t.Parallel()
	assert.NotNil(t, ErrNotFound)
	assert.NotNil(t, ErrConflict)
	assert.Equal(t, "generation not found", ErrNotFound.Error())
	assert.Equal(t, "files modified after generation", ErrConflict.Error())
}

func TestUT_ConflictError_SinglePath(t *testing.T) {
	t.Parallel()
	err := &ConflictError{Paths: []string{"only.go"}}
	assert.Contains(t, err.Error(), "only.go")
}
