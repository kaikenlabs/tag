package hooks

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ===========================================================================
// hooks.go — NewHookRunner (line 66-68)
// ===========================================================================

func TestUT_NewHookRunner_ReturnsArgvRunner(t *testing.T) {
	t.Parallel()
	runner := NewHookRunner()
	assert.NotNil(t, runner)

	// Should be an *ArgvHookRunner
	_, ok := runner.(*ArgvHookRunner)
	assert.True(t, ok, "expected *ArgvHookRunner")
}
