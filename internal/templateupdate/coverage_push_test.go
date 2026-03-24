package templateupdate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ===========================================================================
// hooks.go — HookChangeType.String (lines 24-35)
// ===========================================================================

func TestUT_HookChangeType_String(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ct       HookChangeType
		expected string
	}{
		{HookAdded, "added"},
		{HookRemoved, "removed"},
		{HookModified, "modified"},
		{HookChangeType(99), "unknown"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.ct.String())
	}
}

// ===========================================================================
// variables.go — VarChangeType.String (lines 26-39)
// ===========================================================================

func TestUT_VarChangeType_String(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ct       VarChangeType
		expected string
	}{
		{VarAdded, "added"},
		{VarRemoved, "removed"},
		{VarDefaultChanged, "default-changed"},
		{VarTypeChanged, "type-changed"},
		{VarChangeType(99), "unknown"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.ct.String())
	}
}
