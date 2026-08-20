package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

var tagBinary string

func TestMain(m *testing.M) {
	os.Exit(runIntegrationTests(m))
}

func runIntegrationTests(m *testing.M) int {
	dir, err := os.MkdirTemp("", "tag-integration-bin")
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot create temp dir for the tag binary:", err)
		return 1
	}
	defer os.RemoveAll(dir)

	name := "tag"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	tagBinary = filepath.Join(dir, name)

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Fprintln(os.Stderr, "cannot resolve the integration package path")
		return 1
	}

	build := exec.CommandContext(context.Background(), "go", "build", "-o", tagBinary, ".")
	build.Dir = filepath.Join(filepath.Dir(thisFile), "..", "..")
	if out, buildErr := build.CombinedOutput(); buildErr != nil {
		fmt.Fprintf(os.Stderr, "cannot build the tag binary: %v\n%s", buildErr, out)
		return 1
	}

	return m.Run()
}
