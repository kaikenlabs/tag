package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_CreateMemoryLoaderFromMap_Empty(t *testing.T) {
	t.Parallel()
	loader := CreateMemoryLoaderFromMap(map[string]string{})
	assert.NotNil(t, loader)
}

func TestUT_CreateMemoryLoaderFromMap_NormalizesKeys(t *testing.T) {
	t.Parallel()
	templates := map[string]string{
		"header.tmpl":  "header content",
		"/footer.tmpl": "footer content",
	}

	loader := CreateMemoryLoaderFromMap(templates)
	require.NotNil(t, loader)

	// Both should be resolvable with "/" prefix
	resolved, err := loader.Resolve("/header.tmpl")
	require.NoError(t, err)
	assert.Equal(t, "/header.tmpl", resolved)

	resolved, err = loader.Resolve("/footer.tmpl")
	require.NoError(t, err)
	assert.Equal(t, "/footer.tmpl", resolved)
}

func TestUT_CreateMemoryLoaderFromMap_MultipleTemplates(t *testing.T) {
	t.Parallel()
	templates := map[string]string{
		"a.tmpl": "alpha",
		"b.tmpl": "beta",
		"c.tmpl": "gamma",
	}
	loader := CreateMemoryLoaderFromMap(templates)
	require.NotNil(t, loader)

	for _, name := range []string{"a.tmpl", "b.tmpl", "c.tmpl"} {
		resolved, err := loader.Resolve("/" + name)
		require.NoError(t, err)
		assert.NotEmpty(t, resolved)
	}
}
