package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_UpgradeCommand_Structure(t *testing.T) {
	t.Parallel()
	cmd := UpgradeCommand("v1.0.0")

	assert.Equal(t, "upgrade", cmd.Name)
	assert.Equal(t, []string{"u"}, cmd.Aliases)
	assert.NotEmpty(t, cmd.Usage)
	assert.NotEmpty(t, cmd.Description)
	assert.NotNil(t, cmd.Action)
}

func TestUT_ResolveBinaryPath(t *testing.T) {
	t.Parallel()
	// Should resolve the test binary path without error
	path, err := resolveBinaryPath()
	require.NoError(t, err)
	assert.NotEmpty(t, path)
}
