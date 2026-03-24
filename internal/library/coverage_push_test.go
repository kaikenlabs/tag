package library

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// library.go — New creates library with resolver (lines 36-47)
// ===========================================================================

func TestUT_New_CreatesLibraryWithResolver(t *testing.T) {
	t.Parallel()
	lib, err := New(t.TempDir())
	require.NoError(t, err)
	assert.NotNil(t, lib)
	assert.NotNil(t, lib.resolver)
	assert.NotNil(t, lib.store)
}
