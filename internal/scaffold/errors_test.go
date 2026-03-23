package scaffold

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_VariableError_Error_WithErr(t *testing.T) {
	t.Parallel()
	inner := errors.New("type mismatch")
	err := NewVariableError("port", "invalid type", inner)
	assert.Equal(t, `variable "port": invalid type: type mismatch`, err.Error())
}

func TestUT_VariableError_Error_WithoutErr(t *testing.T) {
	t.Parallel()
	err := &VariableError{Name: "port", Message: "is required"}
	assert.Equal(t, `variable "port": is required`, err.Error())
}

func TestUT_VariableError_Unwrap(t *testing.T) {
	t.Parallel()
	inner := errors.New("inner")
	err := NewVariableError("x", "msg", inner)
	assert.Equal(t, inner, err.Unwrap())
}

func TestUT_VariableError_Unwrap_Nil(t *testing.T) {
	t.Parallel()
	err := NewVariableError("x", "msg", nil)
	assert.Nil(t, err.Unwrap())
}

func TestUT_PathError_Error_WithErr(t *testing.T) {
	t.Parallel()
	inner := errors.New("traversal")
	err := NewPathError("/foo/bar", "escapes base", inner)
	assert.Equal(t, `path "/foo/bar": escapes base: traversal`, err.Error())
}

func TestUT_PathError_Error_WithoutErr(t *testing.T) {
	t.Parallel()
	err := &PathError{Path: "/foo", Message: "not found"}
	assert.Equal(t, `path "/foo": not found`, err.Error())
}

func TestUT_PathError_Unwrap(t *testing.T) {
	t.Parallel()
	inner := errors.New("inner")
	err := NewPathError("/p", "msg", inner)
	assert.Equal(t, inner, err.Unwrap())
}

func TestUT_FileProcessingError_Error_WithErr(t *testing.T) {
	t.Parallel()
	inner := errors.New("parse failure")
	err := NewFileProcessingError("main.go", "render failed", inner)
	assert.Equal(t, `template "main.go": render failed: parse failure`, err.Error())
}

func TestUT_FileProcessingError_Error_WithoutErr(t *testing.T) {
	t.Parallel()
	err := &FileProcessingError{File: "main.go", Message: "skipped"}
	assert.Equal(t, `template "main.go": skipped`, err.Error())
}

func TestUT_FileProcessingError_Unwrap(t *testing.T) {
	t.Parallel()
	inner := errors.New("inner")
	err := NewFileProcessingError("f.go", "msg", inner)
	assert.Equal(t, inner, err.Unwrap())
}

func TestUT_CookiecutterDetectedError_Message(t *testing.T) {
	t.Parallel()
	err := &CookiecutterDetectedError{CookiecutterPath: "/path/to/cookiecutter.json"}
	assert.Equal(t, "cookiecutter template detected: /path/to/cookiecutter.json", err.Error())
}

func TestUT_SentinelErrors_AllDefined(t *testing.T) {
	t.Parallel()
	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrPromptCancelled", ErrPromptCancelled},
		{"ErrRequiredVariableMissing", ErrRequiredVariableMissing},
		{"ErrInvalidVariableType", ErrInvalidVariableType},
		{"ErrOutputExists", ErrOutputExists},
		{"ErrTemplateNotFound", ErrTemplateNotFound},
		{"ErrConfigNotFound", ErrConfigNotFound},
	}

	for _, tt := range sentinels {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.NotNil(t, tt.err)
			assert.NotEmpty(t, tt.err.Error())
		})
	}
}

func TestUT_VariableError_ErrorIs(t *testing.T) {
	t.Parallel()
	inner := ErrRequiredVariableMissing
	err := NewVariableError("name", "missing", inner)
	assert.ErrorIs(t, err, ErrRequiredVariableMissing)
}
