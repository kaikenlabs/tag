package hooks

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_HookError_Error_WithExitCode(t *testing.T) {
	t.Parallel()

	he := &HookError{
		Phase:    HookPhasePre,
		Command:  "make build",
		ExitCode: 2,
	}

	got := he.Error()
	assert.Contains(t, got, "pre_scaffold")
	assert.Contains(t, got, "make build")
	assert.Contains(t, got, "exited with code 2")
}

func TestUT_HookError_Error_WithErr(t *testing.T) {
	t.Parallel()

	he := &HookError{
		Phase:   HookPhasePost,
		Command: "deploy.sh",
		Err:     errors.New("permission denied"),
	}

	got := he.Error()
	assert.Contains(t, got, "post_scaffold")
	assert.Contains(t, got, "deploy.sh")
	assert.Contains(t, got, "permission denied")
}

func TestUT_HookError_Error_Neither(t *testing.T) {
	t.Parallel()

	he := &HookError{
		Phase:   HookPhasePre,
		Command: "cleanup.sh",
	}

	got := he.Error()
	assert.Contains(t, got, "pre_scaffold")
	assert.Contains(t, got, "cleanup.sh")
	assert.NotContains(t, got, "exited with code")
}

func TestUT_HookError_Unwrap(t *testing.T) {
	t.Parallel()

	inner := errors.New("underlying")
	he := &HookError{Err: inner}
	assert.Equal(t, inner, he.Unwrap())
}

func TestUT_HookError_Unwrap_Nil(t *testing.T) {
	t.Parallel()

	he := &HookError{}
	assert.Nil(t, he.Unwrap())
}

func TestUT_NewHookError(t *testing.T) {
	t.Parallel()

	inner := errors.New("exec failed")
	he := NewHookError(HookPhasePost, "run.sh", "some output", 1, inner)

	require.NotNil(t, he)
	assert.Equal(t, HookPhasePost, he.Phase)
	assert.Equal(t, "run.sh", he.Command)
	assert.Equal(t, "some output", he.Output)
	assert.Equal(t, 1, he.ExitCode)
	assert.Equal(t, inner, he.Err)
}

func TestUT_HookPhaseConstants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, HookPhase("pre_scaffold"), HookPhasePre)
	assert.Equal(t, HookPhase("post_scaffold"), HookPhasePost)
}
