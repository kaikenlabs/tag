package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUT_ShortCommitSHA_Long(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "abc1234", shortCommitSHA("abc1234567890def"))
}

func TestUT_ShortCommitSHA_Short(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "abc", shortCommitSHA("abc"))
}

func TestUT_ShortCommitSHA_Empty(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", shortCommitSHA(""))
}

func TestUT_ShortCommitSHA_ExactlySeven(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "abc1234", shortCommitSHA("abc1234"))
}
