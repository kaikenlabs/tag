package update

import (
	"fmt"
	"runtime"
)

// PlatformInfo describes the target platform for binary downloads.
type PlatformInfo struct {
	OS         string // GoReleaser OS name (e.g., "Linux", "Darwin", "Windows")
	Arch       string // GoReleaser arch name (e.g., "x86_64", "arm64")
	ArchiveExt string // Archive extension (e.g., ".tar.gz", ".zip")
}

// DetectPlatform maps the current runtime OS and architecture to GoReleaser names.
func DetectPlatform() (PlatformInfo, error) {
	osMap := map[string]string{
		"linux":  "Linux",
		"darwin": "Darwin",
	}

	archMap := map[string]string{
		"amd64": "x86_64",
		"arm64": "arm64",
	}

	osName, ok := osMap[runtime.GOOS]
	if !ok {
		return PlatformInfo{}, fmt.Errorf("%w: OS %q", ErrUnsupportedPlatform, runtime.GOOS)
	}

	archName, ok := archMap[runtime.GOARCH]
	if !ok {
		return PlatformInfo{}, fmt.Errorf("%w: architecture %q", ErrUnsupportedPlatform, runtime.GOARCH)
	}

	return PlatformInfo{
		OS:         osName,
		Arch:       archName,
		ArchiveExt: ".tar.gz",
	}, nil
}
