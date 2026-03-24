package scaffold

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUT_GetPrompter_NonTTYNoInput(t *testing.T) {
	// In test environment, stdin is not a TTY,
	// so GetPrompter(false) should return NoopPrompter.
	p := GetPrompter(false)
	_, ok := p.(*NoopPrompter)
	assert.True(t, ok, "GetPrompter(false) in non-TTY should return *NoopPrompter")
}

func TestUT_IsTTY_ReturnsFalseInTests(t *testing.T) {
	// Just verify it doesn't panic and returns a valid bool.
	// In test environment, stdin is typically not a TTY.
	result := IsTTY()
	assert.False(t, result)
}

func TestUT_FormatNumber_NegativeHalf(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "-0.5", formatNumber(-0.5))
}

func TestUT_FormatNumber_VeryLargeInteger(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "10000000", formatNumber(10000000.0))
}

func TestUT_NoopPrompter_Confirm_DefaultTrue(t *testing.T) {
	t.Parallel()
	p := NewNoopPrompter()
	result, err := p.Confirm("Continue?", true)
	assert.NoError(t, err)
	assert.True(t, result)
}

func TestUT_NoopPrompter_Confirm_DefaultFalse(t *testing.T) {
	t.Parallel()
	p := NewNoopPrompter()
	result, err := p.Confirm("Continue?", false)
	assert.NoError(t, err)
	assert.False(t, result)
}
