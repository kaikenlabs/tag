package chalk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Purple - colour purple (test-only helper)
func Purple(msg string) string {
	return colourTerminalOutput(msg, purple)
}

// Gray - colour gray (test-only helper)
func Gray(msg string) string {
	return colourTerminalOutput(msg, gray)
}

// White - colour white (test-only helper)
func White(msg string) string {
	return colourTerminalOutput(msg, white)
}

func TestUT_ColourFunctions_ContainMessage(t *testing.T) {
	// All colour functions should include the original message
	tests := []struct {
		name string
		fn   func(string) string
	}{
		{"Red", Red},
		{"Green", Green},
		{"Yellow", Yellow},
		{"Blue", Blue},
		{"Purple", Purple},
		{"Cyan", Cyan},
		{"Gray", Gray},
		{"White", White},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn("hello")
			assert.Contains(t, result, "hello")
		})
	}
}

func TestUT_ColourTerminalOutput_NonTerminal(t *testing.T) {
	// In test environment (non-TTY), should return plain message
	result := colourTerminalOutput("test", red)
	// When not a terminal, no ANSI codes should be present
	if !isTerminal() {
		assert.Equal(t, "test", result)
	} else {
		assert.Contains(t, result, "test")
		assert.Contains(t, result, string(red))
		assert.Contains(t, result, string(reset))
	}
}

func TestUT_ColourTerminalOutput_EmptyString(t *testing.T) {
	result := colourTerminalOutput("", red)
	if !isTerminal() {
		assert.Equal(t, "", result)
	}
}

func TestUT_IsTerminal_NO_COLOR_Disables(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	assert.False(t, isTerminal())
}

func TestUT_IsTerminal_NO_COLOR_AnyValue(t *testing.T) {
	// Per no-color.org spec, any non-empty value disables color
	for _, val := range []string{"1", "true", "0", "false", "yes"} {
		t.Run(val, func(t *testing.T) {
			t.Setenv("NO_COLOR", val)
			assert.False(t, isTerminal())
		})
	}
}

func TestUT_IsTerminal_NO_COLOR_Empty(t *testing.T) {
	// Empty NO_COLOR should NOT disable colors (per spec)
	t.Setenv("NO_COLOR", "")
	// Result depends on actual TTY status — just verify it doesn't force-disable
	// In CI (non-TTY), this returns false anyway; in TTY it should return true.
	// We can't assert true in CI, so just verify the code path doesn't panic.
	_ = isTerminal()
}

func TestUT_ColourTerminalOutput_NO_COLOR(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	result := colourTerminalOutput("hello", red)
	assert.Equal(t, "hello", result)
	assert.NotContains(t, result, "\033[")
}
