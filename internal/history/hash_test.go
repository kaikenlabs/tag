package history

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_HashBytes_KnownContent(t *testing.T) {
	// echo -n "hello" | sha256sum → 2cf24dba...
	h := HashBytes([]byte("hello"))
	assert.Equal(t, "sha256:2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", h)
}

func TestUT_HashBytes_EmptyInput(t *testing.T) {
	h := HashBytes([]byte{})
	assert.Equal(t, "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", h)
}

func TestUT_HashFile_KnownContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello"), 0o644))

	h, err := HashFile(path)
	require.NoError(t, err)
	assert.Equal(t, "sha256:2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", h)
}

func TestUT_HashFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	require.NoError(t, os.WriteFile(path, []byte{}, 0o644))

	h, err := HashFile(path)
	require.NoError(t, err)
	assert.Equal(t, "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", h)
}

func TestUT_HashFile_NonExistentFile(t *testing.T) {
	_, err := HashFile("/nonexistent/path/file.txt")
	assert.Error(t, err)
}
