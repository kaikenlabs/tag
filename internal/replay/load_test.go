package replay

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUT_Load_WhitespaceSource(t *testing.T) {
	_, err := Load("   ")
	assert.ErrorIs(t, err, ErrEmptyTemplateSource)
}

func TestUT_Load_TabsOnlySource(t *testing.T) {
	_, err := Load("\t\t")
	assert.ErrorIs(t, err, ErrEmptyTemplateSource)
}

func TestUT_GetReplayFilePath_WhitespaceSource(t *testing.T) {
	_, err := GetReplayFilePath("   ")
	assert.ErrorIs(t, err, ErrEmptyTemplateSource)
}
