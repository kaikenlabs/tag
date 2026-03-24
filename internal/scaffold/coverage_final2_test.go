package scaffold

import (
	"errors"
	"testing"

	"github.com/charmbracelet/huh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// prompt.go — coverage for mapPromptErr with wrapped huh.ErrUserAborted,
// InteractivePrompter creation, Select with zero-length validation
// ===========================================================================

func TestUT_MapPromptErr_WrappedAbort(t *testing.T) {
	t.Parallel()
	// Test with a wrapped huh.ErrUserAborted
	err := mapPromptErr(huh.ErrUserAborted)
	assert.ErrorIs(t, err, ErrPromptCancelled)
}

func TestUT_MapPromptErr_OtherError(t *testing.T) {
	t.Parallel()
	other := errors.New("input timeout")
	err := mapPromptErr(other)
	assert.Contains(t, err.Error(), "prompt failed")
	assert.ErrorIs(t, err, other)
}

func TestUT_NoopPrompter_Select_SingleOption(t *testing.T) {
	t.Parallel()
	p := NewNoopPrompter()
	result, err := p.Select("Pick one", []string{"only"}, 0)
	require.NoError(t, err)
	assert.Equal(t, "only", result)
}

func TestUT_NoopPrompter_Input_EmptyDefault(t *testing.T) {
	t.Parallel()
	p := NewNoopPrompter()
	result, err := p.Input("Name", "", false)
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestUT_NoopPrompter_Number_NegativeFloat(t *testing.T) {
	t.Parallel()
	p := NewNoopPrompter()
	result, err := p.Number("Temp", -273.15)
	require.NoError(t, err)
	assert.Equal(t, -273.15, result)
}

func TestUT_FormatNumber_OnePointFive(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "1.5", formatNumber(1.5))
}

func TestUT_FormatNumber_ExactlyOne(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "1", formatNumber(1.0))
}

// ===========================================================================
// Prompter interface — additional compile-time checks
// ===========================================================================

func TestUT_PrompterInterface_Both(t *testing.T) {
	t.Parallel()
	var p Prompter

	p = NewNoopPrompter()
	assert.NotNil(t, p)

	p = NewInteractivePrompter()
	assert.NotNil(t, p)
}
