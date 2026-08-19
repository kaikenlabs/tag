package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestMain points HOME and the XDG base directories at a throwaway tree for the
// whole test binary.
//
// Several commands resolve user state from the environment rather than through
// an injectable seam: replay.Save calls os.UserHomeDir() directly
// (internal/replay/save.go), doctorCheckLibraries calls xdg.DataHome() rather
// than the overridable newLocalLibrary var, and remote.NewFSCache("") falls back
// to $HOME/.tag/cache. Any test that drives a command far enough to reach one of
// those therefore wrote into the developer's real home — hundreds of stray
// ~/.tag/replay entries had accumulated that way, and one test was re-fetching
// the real library over the network on every run.
//
// Isolating per-test is whack-a-mole: the next test to exercise a new code path
// reintroduces it silently, because writing to a real directory succeeds. This
// makes the safe default binary-wide. Individual tests may still t.Setenv these
// to something more specific; that continues to work and takes precedence.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "tag-commands-home-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot create isolated HOME for tests: %v\n", err)
		os.Exit(1)
	}

	for k, v := range map[string]string{
		"HOME":            home,
		"XDG_DATA_HOME":   filepath.Join(home, "data"),
		"XDG_CACHE_HOME":  filepath.Join(home, "cache"),
		"XDG_CONFIG_HOME": filepath.Join(home, "config"),
	} {
		if setErr := os.Setenv(k, v); setErr != nil {
			fmt.Fprintf(os.Stderr, "cannot set %s: %v\n", k, setErr)
			os.Exit(1)
		}
	}

	code := m.Run()
	_ = os.RemoveAll(home)
	os.Exit(code)
}
