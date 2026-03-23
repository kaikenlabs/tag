package templateupdate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUT_NewDiffer_SetsFields(t *testing.T) {
	t.Parallel()

	d := NewDiffer(nil, nil)

	assert.NotNil(t, d)
	assert.Nil(t, d.renderer)
	assert.Nil(t, d.resolver)
}

func TestUT_Differ_Diff_MissingConfig(t *testing.T) {
	t.Parallel()

	d := NewDiffer(nil, nil)

	_, err := d.Diff(t.Context(), DiffOptions{
		ProjectDir: t.TempDir(),
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tagconfig")
}
