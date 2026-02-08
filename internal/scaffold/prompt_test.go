package scaffold

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- NoopPrompter ---

func TestUT_NoopPrompter_Input(t *testing.T) {
	p := NewNoopPrompter()

	tests := []struct {
		name         string
		label        string
		defaultValue string
		secret       bool
	}{
		{"returns default", "Name", "default_val", false},
		{"empty default", "Name", "", false},
		{"secret flag ignored", "Password", "secret123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Input(tt.label, tt.defaultValue, tt.secret)
			require.NoError(t, err)
			assert.Equal(t, tt.defaultValue, result)
		})
	}
}

func TestUT_NoopPrompter_Select(t *testing.T) {
	p := NewNoopPrompter()

	tests := []struct {
		name         string
		options      []string
		defaultIndex int
		expected     string
		expectErr    bool
	}{
		{
			name:         "returns option at default index",
			options:      []string{"a", "b", "c"},
			defaultIndex: 1,
			expected:     "b",
		},
		{
			name:         "returns first option for index 0",
			options:      []string{"first", "second"},
			defaultIndex: 0,
			expected:     "first",
		},
		{
			name:         "negative index falls back to first",
			options:      []string{"a", "b"},
			defaultIndex: -1,
			expected:     "a",
		},
		{
			name:         "out of range index falls back to first",
			options:      []string{"a", "b"},
			defaultIndex: 99,
			expected:     "a",
		},
		{
			name:         "empty options returns error",
			options:      []string{},
			defaultIndex: 0,
			expectErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Select("Choose", tt.options, tt.defaultIndex)
			if tt.expectErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_NoopPrompter_Confirm(t *testing.T) {
	p := NewNoopPrompter()

	tests := []struct {
		name         string
		defaultValue bool
	}{
		{"returns true default", true},
		{"returns false default", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Confirm("Continue?", tt.defaultValue)
			require.NoError(t, err)
			assert.Equal(t, tt.defaultValue, result)
		})
	}
}

func TestUT_NoopPrompter_Number(t *testing.T) {
	p := NewNoopPrompter()

	tests := []struct {
		name         string
		defaultValue float64
	}{
		{"zero", 0},
		{"positive integer", 42},
		{"positive float", 3.14},
		{"negative integer", -10},
		{"negative float", -2.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Number("Value", tt.defaultValue)
			require.NoError(t, err)
			assert.Equal(t, tt.defaultValue, result)
		})
	}
}

// --- formatNumber ---

func TestUT_FormatNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected string
	}{
		{"zero", 0, "0"},
		{"positive integer", 42, "42"},
		{"negative integer", -42, "-42"},
		{"positive float", 3.14, "3.14"},
		{"negative float", -3.14, "-3.14"},
		{"integer-like float", 100.0, "100"},
		{"large integer", 1000000, "1000000"},
		{"small float", 0.001, "0.001"},
		{"half", 0.5, "0.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatNumber(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- GetPrompter ---

func TestUT_GetPrompter_NoInput(t *testing.T) {
	// When noInput=true, should always return NoopPrompter
	p := GetPrompter(true)
	_, ok := p.(*NoopPrompter)
	assert.True(t, ok, "GetPrompter(true) should return *NoopPrompter")
}

// --- Prompter interface compliance ---

func TestUT_NoopPrompter_ImplementsPrompter(t *testing.T) {
	var _ Prompter = (*NoopPrompter)(nil)
	var _ Prompter = (*InteractivePrompter)(nil)
}
