package validate

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		{"max length", strings.Repeat("a", MaxNameLen), false},
		{"exceeds max length", strings.Repeat("a", MaxNameLen+1), true},
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

func TestUT_GeneratorName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{"valid name", "my-generator", false, ""},
		{"valid name with numbers", "gen123", false, ""},
		{"reserved list", "list", true, "reserved name"},
		{"reserved ls", "ls", true, "reserved name"},
		{"reserved info", "info", true, "reserved name"},
		{"reserved agent-file", "agent-file", true, "reserved name"},
		{"empty string", "", true, "must not be empty"},
		{"path traversal", "../hack", true, "path traversal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := GeneratorName(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalidName)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
