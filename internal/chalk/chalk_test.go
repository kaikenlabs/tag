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
