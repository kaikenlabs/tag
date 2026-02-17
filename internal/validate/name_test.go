package validate

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUT_PathSegmentSafe(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple name", "my-template", false},
		{"valid with numbers", "template123", false},
		{"empty string", "", true},
		{"whitespace only", "   ", true},
		{"dot", ".", true},
		{"dotdot", "..", true},
		{"contains dotdot", "foo..bar", true},
		{"forward slash", "foo/bar", true},
		{"backslash", "foo\\bar", true},
		{"valid with dash", "my-project", false},
		{"valid with underscore", "my_project", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := PathSegmentSafe(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, ErrInvalidName))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUT_TemplateName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid name", "my-template", false},
		{"empty string", "", true},
		{"dot prefix", ".hidden", true},
		{"control character", "bad\x00name", true},
		{"del character", "bad\x7fname", true},
		{"max length", strings.Repeat("a", maxNameLen), false},
		{"exceeds max length", strings.Repeat("a", maxNameLen+1), true},
		{"path separator", "a/b", true},
		{"dotdot", "..", true},
		{"valid with numbers", "template-v2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := TemplateName(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, ErrInvalidName))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
