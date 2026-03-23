package template

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_TemplateError_Error_OpOnly(t *testing.T) {
	t.Parallel()
	err := &TemplateError{
		Op:  "parse",
		Err: errors.New("syntax error"),
	}
	assert.Equal(t, "parse: syntax error", err.Error())
}

func TestUT_TemplateError_Error_WithTemplate(t *testing.T) {
	t.Parallel()
	err := &TemplateError{
		Op:       "parse",
		Template: "main.tmpl",
		Err:      errors.New("unexpected token"),
	}
	assert.Equal(t, "parse: main.tmpl: unexpected token", err.Error())
}

func TestUT_TemplateError_Error_WithLine(t *testing.T) {
	t.Parallel()
	err := &TemplateError{
		Op:       "parse",
		Template: "main.tmpl",
		Line:     42,
		Err:      errors.New("undefined variable"),
	}
	assert.Equal(t, "parse: main.tmpl:42: undefined variable", err.Error())
}

func TestUT_TemplateError_Error_WithLineAndColumn(t *testing.T) {
	t.Parallel()
	err := &TemplateError{
		Op:       "parse",
		Template: "main.tmpl",
		Line:     42,
		Column:   10,
		Err:      errors.New("missing brace"),
	}
	assert.Equal(t, "parse: main.tmpl:42:10: missing brace", err.Error())
}

func TestUT_TemplateError_Unwrap(t *testing.T) {
	t.Parallel()
	inner := errors.New("inner error")
	err := &TemplateError{Op: "execute", Err: inner}
	assert.Equal(t, inner, err.Unwrap())
}

func TestUT_NewParseError(t *testing.T) {
	t.Parallel()
	inner := errors.New("bad syntax")
	err := NewParseError("header.tmpl", 5, 3, inner)

	var tmplErr *TemplateError
	require.ErrorAs(t, err, &tmplErr)
	assert.Equal(t, "parse", tmplErr.Op)
	assert.Equal(t, "header.tmpl", tmplErr.Template)
	assert.Equal(t, 5, tmplErr.Line)
	assert.Equal(t, 3, tmplErr.Column)
	assert.ErrorIs(t, err, ErrParse)
}

func TestUT_NewExecuteError(t *testing.T) {
	t.Parallel()
	inner := errors.New("nil pointer")
	err := NewExecuteError("body.tmpl", inner)

	var tmplErr *TemplateError
	require.ErrorAs(t, err, &tmplErr)
	assert.Equal(t, "execute", tmplErr.Op)
	assert.Equal(t, "body.tmpl", tmplErr.Template)
	assert.ErrorIs(t, err, ErrExecute)
}

func TestUT_SentinelErrors(t *testing.T) {
	t.Parallel()
	assert.NotNil(t, ErrParse)
	assert.NotNil(t, ErrExecute)
	assert.Equal(t, "template parse error", ErrParse.Error())
	assert.Equal(t, "template execution error", ErrExecute.Error())
}
