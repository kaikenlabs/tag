package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// engine.go — SetLoader (line 245-247)
// ===========================================================================

func TestUT_Engine_SetLoader(t *testing.T) {
	t.Parallel()
	engine, err := NewEngine()
	require.NoError(t, err)

	// Setting a nil loader should not panic
	engine.SetLoader(nil)
	assert.Nil(t, engine.loader)
}

// ===========================================================================
// engine.go — SetSharedContent (lines 253-270)
// ===========================================================================

func TestUT_Engine_SetSharedContent(t *testing.T) {
	t.Parallel()
	engine, err := NewEngine()
	require.NoError(t, err)

	content := map[string]string{
		"path/to/header.tmpl": "header content",
		"dir/footer.tmpl":     "footer content",
	}

	engine.SetSharedContent(content)

	// Verify content is normalised to basenames
	assert.NotEmpty(t, engine.sharedContent)
	assert.Equal(t, "header content", engine.sharedContent["header.tmpl"])
	assert.Equal(t, "footer content", engine.sharedContent["footer.tmpl"])
}

func TestUT_Engine_SetSharedContent_Empty(t *testing.T) {
	t.Parallel()
	engine, err := NewEngine()
	require.NoError(t, err)

	engine.SetSharedContent(nil)
	assert.Empty(t, engine.sharedContent)
}
