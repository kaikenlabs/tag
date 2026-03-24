package scaffold

import (
	"errors"
	"testing"

	"github.com/charmbracelet/huh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mapPromptErr ---

func TestUT_MapPromptErr_UserAborted(t *testing.T) {
	t.Parallel()
	err := mapPromptErr(huh.ErrUserAborted)
	assert.ErrorIs(t, err, ErrPromptCancelled)
}

func TestUT_MapPromptErr_GenericError(t *testing.T) {
	t.Parallel()
	inner := errors.New("something went wrong")
	err := mapPromptErr(inner)
	assert.Contains(t, err.Error(), "prompt failed")
	assert.ErrorIs(t, err, inner)
}

// --- NoopPrompter edge cases ---

func TestUT_NoopPrompter_Select_NilOptions(t *testing.T) {
	t.Parallel()
	p := NewNoopPrompter()
	_, err := p.Select("Choose", nil, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no options provided")
}

func TestUT_NoopPrompter_Select_LargeNegativeIndex(t *testing.T) {
	t.Parallel()
	p := NewNoopPrompter()
	result, err := p.Select("Choose", []string{"a", "b"}, -100)
	require.NoError(t, err)
	assert.Equal(t, "a", result, "should fall back to first option")
}

func TestUT_NoopPrompter_Select_ExactBoundary(t *testing.T) {
	t.Parallel()
	p := NewNoopPrompter()

	// Last valid index
	result, err := p.Select("Choose", []string{"a", "b", "c"}, 2)
	require.NoError(t, err)
	assert.Equal(t, "c", result)

	// First invalid index
	result, err = p.Select("Choose", []string{"a", "b", "c"}, 3)
	require.NoError(t, err)
	assert.Equal(t, "a", result, "should fall back to first option")
}

func TestUT_NoopPrompter_Input_EmptyLabel(t *testing.T) {
	t.Parallel()
	p := NewNoopPrompter()
	result, err := p.Input("", "default", false)
	require.NoError(t, err)
	assert.Equal(t, "default", result)
}

func TestUT_NoopPrompter_Number_Zero(t *testing.T) {
	t.Parallel()
	p := NewNoopPrompter()
	result, err := p.Number("Value", 0)
	require.NoError(t, err)
	assert.Equal(t, float64(0), result)
}

func TestUT_NoopPrompter_Number_LargeValue(t *testing.T) {
	t.Parallel()
	p := NewNoopPrompter()
	result, err := p.Number("Value", 999999.99)
	require.NoError(t, err)
	assert.Equal(t, 999999.99, result)
}

// --- formatNumber edge cases ---

func TestUT_FormatNumber_NegativeZero(t *testing.T) {
	t.Parallel()
	negZero := -1.0 * 0.0
	result := formatNumber(negZero)
	assert.Equal(t, "0", result, "negative zero should format as 0")
}

func TestUT_FormatNumber_VerySmallFloat(t *testing.T) {
	t.Parallel()
	result := formatNumber(0.0001)
	assert.Equal(t, "0.0001", result)
}

func TestUT_FormatNumber_MaxInt(t *testing.T) {
	t.Parallel()
	result := formatNumber(9007199254740992) // 2^53
	assert.NotEmpty(t, result)
}

// --- GetPrompter ---

func TestUT_GetPrompter_NoInputTrue_ReturnsNoop(t *testing.T) {
	t.Parallel()
	p := GetPrompter(true)
	_, ok := p.(*NoopPrompter)
	assert.True(t, ok, "noInput=true should return NoopPrompter")
}

// --- Prompter interface checks ---

func TestUT_InteractivePrompter_ImplementsPrompter(t *testing.T) {
	t.Parallel()
	var _ Prompter = (*InteractivePrompter)(nil)
}

func TestUT_NoopPrompter_ImplementsPrompter_Check(t *testing.T) {
	t.Parallel()
	var _ Prompter = (*NoopPrompter)(nil)
}

func TestUT_NewInteractivePrompter_NotNil(t *testing.T) {
	t.Parallel()
	p := NewInteractivePrompter()
	assert.NotNil(t, p)
}

func TestUT_NewNoopPrompter_NotNil(t *testing.T) {
	t.Parallel()
	p := NewNoopPrompter()
	assert.NotNil(t, p)
}
