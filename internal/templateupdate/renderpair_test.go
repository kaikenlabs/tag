package templateupdate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUT_ShortSHA(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"abc123def456789012345678901234567890abcd", "abc123d"},
		{"short", "short"},
		{"abc1234", "abc1234"},
		{"", ""},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, shortSHA(tt.input))
	}
}

func TestUT_ToPointerMap_EmptyInput(t *testing.T) {
	result := ToPointerMap(nil)
	assert.Empty(t, result)
}

func TestUT_ToPointerMap_PreservesContent(t *testing.T) {
	input := map[string]RenderedFile{
		"file.txt": {Content: []byte("content"), Mode: 0o644, IsBinary: false},
	}

	result := ToPointerMap(input)
	assert.Equal(t, []byte("content"), result["file.txt"].Content)
	assert.Equal(t, input["file.txt"].Mode, result["file.txt"].Mode)
}
