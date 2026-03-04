package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_IsTruthy(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected bool
	}{
		{"nil is falsy", nil, false},
		{"true is truthy", true, true},
		{"false is falsy", false, false},
		{"non-empty string is truthy", "hello", true},
		{"empty string is falsy", "", false},
		{"whitespace string is truthy", "   ", true},
		{"string zero is truthy", "0", true},
		{"positive float64 is truthy", 1.5, true},
		{"zero float64 is falsy", 0.0, false},
		{"negative float64 is truthy", -1.0, true},
		{"positive int is truthy", 1, true},
		{"zero int is falsy", 0, false},
		{"slice is truthy", []string{"a"}, true},
		{"map is truthy", map[string]any{"a": 1}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isTruthy(tt.value))
		})
	}
}

func TestUT_CheckRequirements_AllMet(t *testing.T) {
	vars := map[string]any{
		"use_postgres": true,
		"use_amqp":     true,
	}
	err := checkRequirements("mybundle", "bundle", []string{"use_postgres", "use_amqp"}, vars)
	require.NoError(t, err)
}

func TestUT_CheckRequirements_OneUnmet(t *testing.T) {
	vars := map[string]any{
		"use_postgres": true,
		"use_amqp":     false,
	}
	err := checkRequirements("mybundle", "bundle", []string{"use_postgres", "use_amqp"}, vars)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use_amqp")
	assert.NotContains(t, err.Error(), "use_postgres")
}

func TestUT_CheckRequirements_MultipleUnmet(t *testing.T) {
	vars := map[string]any{
		"use_postgres": false,
		"use_amqp":     false,
	}
	err := checkRequirements("mybundle", "bundle", []string{"use_postgres", "use_amqp"}, vars)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use_postgres")
	assert.Contains(t, err.Error(), "use_amqp")
}

func TestUT_CheckRequirements_EmptyRequires(t *testing.T) {
	err := checkRequirements("mybundle", "bundle", nil, nil)
	require.NoError(t, err)

	err = checkRequirements("mybundle", "bundle", []string{}, nil)
	require.NoError(t, err)
}

func TestUT_CheckRequirements_NilVars(t *testing.T) {
	err := checkRequirements("mybundle", "bundle", []string{"use_postgres"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use_postgres")
	assert.Contains(t, err.Error(), "not set")
}

func TestUT_CheckRequirements_MissingVar(t *testing.T) {
	vars := map[string]any{
		"use_amqp": true,
	}
	err := checkRequirements("mybundle", "bundle", []string{"use_postgres"}, vars)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use_postgres")
	assert.Contains(t, err.Error(), "not set")
}

func TestUT_CheckRequirements_FalsyVarShowsDisabled(t *testing.T) {
	vars := map[string]any{
		"use_postgres": false,
	}
	err := checkRequirements("mybundle", "bundle", []string{"use_postgres"}, vars)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "currently disabled")
}

func TestUT_CheckRequirements_DuplicateRequires(t *testing.T) {
	vars := map[string]any{
		"use_postgres": false,
	}
	err := checkRequirements("mybundle", "bundle", []string{"use_postgres", "use_postgres"}, vars)
	require.Error(t, err)
	// Should only mention use_postgres once
	errStr := err.Error()
	first := indexOf(errStr, "use_postgres")
	second := indexOf(errStr[first+len("use_postgres"):], "use_postgres")
	assert.Equal(t, -1, second, "use_postgres should appear only once in error")
}

func TestUT_CheckRequirements_NonBooleanTruthyValues(t *testing.T) {
	vars := map[string]any{
		"module_path": "github.com/example/project",
		"port":        float64(8080),
	}
	err := checkRequirements("mybundle", "bundle", []string{"module_path", "port"}, vars)
	require.NoError(t, err)
}

func TestUT_CheckRequirements_ErrorContainsBundleName(t *testing.T) {
	vars := map[string]any{}
	err := checkRequirements("crud-context", "bundle", []string{"use_postgres"}, vars)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "crud-context")
	assert.Contains(t, err.Error(), "bundle")
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
