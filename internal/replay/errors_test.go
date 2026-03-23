package replay

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_ReplayError_Error_WithErr(t *testing.T) {
	t.Parallel()

	inner := errors.New("disk full")
	re := &ReplayError{
		TemplateID: "my-template",
		Operation:  "save",
		Err:        inner,
	}

	got := re.Error()
	assert.Contains(t, got, "save")
	assert.Contains(t, got, "my-template")
	assert.Contains(t, got, "disk full")
}

func TestUT_ReplayError_Error_NilErr(t *testing.T) {
	t.Parallel()

	re := &ReplayError{
		TemplateID: "my-template",
		Operation:  "load",
		Err:        nil,
	}

	got := re.Error()
	assert.Contains(t, got, "load")
	assert.Contains(t, got, "my-template")
	assert.NotContains(t, got, "nil")
}

func TestUT_ReplayError_Unwrap(t *testing.T) {
	t.Parallel()

	inner := errors.New("wrapped error")
	re := &ReplayError{Err: inner}
	assert.Equal(t, inner, re.Unwrap())
}

func TestUT_ReplayError_Unwrap_Nil(t *testing.T) {
	t.Parallel()

	re := &ReplayError{}
	assert.Nil(t, re.Unwrap())
}

func TestUT_NewReplayError(t *testing.T) {
	t.Parallel()

	inner := errors.New("something broke")
	re := NewReplayError("tmpl-id", "delete", inner)

	require.NotNil(t, re)
	assert.Equal(t, "tmpl-id", re.TemplateID)
	assert.Equal(t, "delete", re.Operation)
	assert.Equal(t, inner, re.Err)
}

func TestUT_SentinelErrors(t *testing.T) {
	t.Parallel()

	assert.NotNil(t, ErrReplayNotFound)
	assert.NotNil(t, ErrReplayCorrupt)
	assert.NotNil(t, ErrEmptyTemplateSource)

	assert.EqualError(t, ErrReplayNotFound, "replay file not found")
	assert.EqualError(t, ErrReplayCorrupt, "replay file is corrupt or invalid")
	assert.EqualError(t, ErrEmptyTemplateSource, "empty template source")
}
