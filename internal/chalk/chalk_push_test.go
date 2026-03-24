package chalk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUT_Dim_ReturnsString(t *testing.T) {
	t.Parallel()
	result := Dim("dimmed text")
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "dimmed text")
}
