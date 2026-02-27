package update

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_DetectPlatform_CurrentOS(t *testing.T) {
	p, err := DetectPlatform()

	switch runtime.GOOS {
	case "linux", "darwin":
		require.NoError(t, err)
		assert.NotEmpty(t, p.OS)
		assert.NotEmpty(t, p.Arch)
		assert.Equal(t, ".tar.gz", p.ArchiveExt)
	default:
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrUnsupportedPlatform)
	}
}

func TestUT_DetectPlatform_OSMapping(t *testing.T) {
	// We can only test the current platform, but we can verify the mapping is correct.
	p, err := DetectPlatform()
	if err != nil {
		t.Skip("unsupported platform")
	}

	switch runtime.GOOS {
	case "linux":
		assert.Equal(t, "Linux", p.OS)
	case "darwin":
		assert.Equal(t, "Darwin", p.OS)
	}
}

func TestUT_DetectPlatform_ArchMapping(t *testing.T) {
	p, err := DetectPlatform()
	if err != nil {
		t.Skip("unsupported platform")
	}

	switch runtime.GOARCH {
	case "amd64":
		assert.Equal(t, "x86_64", p.Arch)
	case "arm64":
		assert.Equal(t, "arm64", p.Arch)
	}
}
