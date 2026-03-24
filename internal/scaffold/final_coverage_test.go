package scaffold

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// prompt.go — coverage for formatNumber, NoopPrompter, mapPromptErr,
// GetPrompter, InteractivePrompter creation
// ===========================================================================

func TestUT_FormatNumber_PositiveInteger(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "42", formatNumber(42.0))
}

func TestUT_FormatNumber_NegativeInteger(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "-5", formatNumber(-5.0))
}

func TestUT_FormatNumber_PositiveFloat(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "3.14", formatNumber(3.14))
}

func TestUT_FormatNumber_Zero(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "0", formatNumber(0.0))
}

func TestUT_FormatNumber_LargeFloat(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "1e+20", formatNumber(1e20))
}

func TestUT_FormatNumber_SmallNegativeFloat(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "-3.14", formatNumber(-3.14))
}

// --- NoopPrompter ---

func TestUT_NoopPrompter_Input_DefaultReturned(t *testing.T) {
	t.Parallel()
	p := NewNoopPrompter()
	result, err := p.Input("Enter name", "default-val", false)
	require.NoError(t, err)
	assert.Equal(t, "default-val", result)
}

func TestUT_NoopPrompter_Input_Secret(t *testing.T) {
	t.Parallel()
	p := NewNoopPrompter()
	result, err := p.Input("Password", "secret", true)
	require.NoError(t, err)
	assert.Equal(t, "secret", result)
}

func TestUT_NoopPrompter_Select_ValidIndex(t *testing.T) {
	t.Parallel()
	p := NewNoopPrompter()
	result, err := p.Select("Choose", []string{"a", "b", "c"}, 1)
	require.NoError(t, err)
	assert.Equal(t, "b", result)
}

func TestUT_NoopPrompter_Select_OutOfBoundsIndex(t *testing.T) {
	t.Parallel()
	p := NewNoopPrompter()
	result, err := p.Select("Choose", []string{"a", "b"}, 99)
	require.NoError(t, err)
	assert.Equal(t, "a", result) // Falls back to first
}

func TestUT_NoopPrompter_Select_NegativeIndex(t *testing.T) {
	t.Parallel()
	p := NewNoopPrompter()
	result, err := p.Select("Choose", []string{"x", "y"}, -1)
	require.NoError(t, err)
	assert.Equal(t, "x", result) // Falls back to first
}

func TestUT_NoopPrompter_Select_EmptyOptions(t *testing.T) {
	t.Parallel()
	p := NewNoopPrompter()
	_, err := p.Select("Choose", []string{}, 0)
	require.Error(t, err)
}

func TestUT_NoopPrompter_Number_Negative(t *testing.T) {
	t.Parallel()
	p := NewNoopPrompter()
	result, err := p.Number("Amount", -99.5)
	require.NoError(t, err)
	assert.Equal(t, -99.5, result)
}

func TestUT_NoopPrompter_Confirm_True(t *testing.T) {
	t.Parallel()
	p := NewNoopPrompter()
	result, err := p.Confirm("Really?", true)
	require.NoError(t, err)
	assert.True(t, result)
}

func TestUT_NoopPrompter_Confirm_False(t *testing.T) {
	t.Parallel()
	p := NewNoopPrompter()
	result, err := p.Confirm("Really?", false)
	require.NoError(t, err)
	assert.False(t, result)
}

// --- GetPrompter ---

func TestUT_GetPrompter_NoInputReturnsNoop(t *testing.T) {
	t.Parallel()

	p := GetPrompter(true)
	_, ok := p.(*NoopPrompter)
	assert.True(t, ok)
}

func TestUT_GetPrompter_NonTTYReturnsNoop(t *testing.T) {
	// In test env, stdin is not a TTY
	p := GetPrompter(false)
	_, ok := p.(*NoopPrompter)
	assert.True(t, ok)
}

// --- IsTTY ---

func TestUT_IsTTY_InTestsReturnsFalse(t *testing.T) {
	t.Parallel()
	assert.False(t, IsTTY())
}

// --- Constructor checks ---

func TestUT_NewInteractivePrompter_ReturnsNonNil(t *testing.T) {
	t.Parallel()
	p := NewInteractivePrompter()
	assert.NotNil(t, p)
}

func TestUT_NewNoopPrompter_ReturnsNonNil(t *testing.T) {
	t.Parallel()
	p := NewNoopPrompter()
	assert.NotNil(t, p)
}
