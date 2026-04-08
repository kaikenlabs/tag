package openapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUT_NormalizeYAML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "trims indentation",
			input: "  type: string\n  format: uuid",
			want:  "type: string\nformat: uuid",
		},
		{
			name:  "removes empty lines",
			input: "type: string\n\nformat: uuid\n",
			want:  "type: string\nformat: uuid",
		},
		{
			name:  "handles mixed indentation",
			input: "    type: object\n        properties:\n            name:\n                type: string",
			want:  "type: object\nproperties:\nname:\ntype: string",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "whitespace only",
			input: "  \n  \n  ",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeYAML(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
