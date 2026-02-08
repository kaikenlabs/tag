package fileutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_IsTextContent(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected bool
	}{
		{
			name:     "valid utf8 text",
			content:  []byte("Hello, World!"),
			expected: true,
		},
		{
			name:     "text with newlines",
			content:  []byte("line1\nline2\nline3"),
			expected: true,
		},
		{
			name:     "empty content",
			content:  []byte{},
			expected: true,
		},
		{
			name:     "binary with null bytes",
			content:  []byte{0x00, 0x01, 0x02},
			expected: false,
		},
		{
			name:     "PNG header",
			content:  []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
			expected: false,
		},
		{
			name:     "text with tabs and carriage returns",
			content:  []byte("col1\tcol2\r\nval1\tval2\r\n"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsTextContent(tt.content))
		})
	}
}

func TestUT_CopyFile(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("copies file with content", func(t *testing.T) {
		src := filepath.Join(tmpDir, "source.txt")
		dst := filepath.Join(tmpDir, "dest.txt")

		err := os.WriteFile(src, []byte("hello world"), 0o644)
		require.NoError(t, err)

		err = CopyFile(src, dst)
		require.NoError(t, err)

		content, err := os.ReadFile(dst)
		require.NoError(t, err)
		assert.Equal(t, "hello world", string(content))
	})

	t.Run("preserves file permissions", func(t *testing.T) {
		src := filepath.Join(tmpDir, "exec.sh")
		dst := filepath.Join(tmpDir, "exec_copy.sh")

		err := os.WriteFile(src, []byte("#!/bin/sh\necho hi"), 0o755)
		require.NoError(t, err)

		err = CopyFile(src, dst)
		require.NoError(t, err)

		info, err := os.Stat(dst)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
	})

	t.Run("strips setuid setgid and sticky bits", func(t *testing.T) {
		src := filepath.Join(tmpDir, "setuid_file")
		dst := filepath.Join(tmpDir, "setuid_copy")

		err := os.WriteFile(src, []byte("content"), 0o755)
		require.NoError(t, err)

		// Set setuid, setgid, and sticky bits on the source file
		err = os.Chmod(src, 0o755|os.ModeSetuid|os.ModeSetgid|os.ModeSticky)
		require.NoError(t, err)

		// Verify source has dangerous bits set
		srcInfo, err := os.Stat(src)
		require.NoError(t, err)
		assert.NotZero(t, srcInfo.Mode()&os.ModeSetuid, "source should have setuid bit")

		err = CopyFile(src, dst)
		require.NoError(t, err)

		dstInfo, err := os.Stat(dst)
		require.NoError(t, err)

		// Verify dangerous bits are stripped
		assert.Zero(t, dstInfo.Mode()&os.ModeSetuid, "setuid bit should be stripped")
		assert.Zero(t, dstInfo.Mode()&os.ModeSetgid, "setgid bit should be stripped")
		assert.Zero(t, dstInfo.Mode()&os.ModeSticky, "sticky bit should be stripped")

		// Verify regular permissions are preserved
		assert.Equal(t, os.FileMode(0o755), dstInfo.Mode().Perm())
	})

	t.Run("source not found returns error", func(t *testing.T) {
		err := CopyFile(filepath.Join(tmpDir, "nonexistent"), filepath.Join(tmpDir, "out"))
		assert.Error(t, err)
	})
}
