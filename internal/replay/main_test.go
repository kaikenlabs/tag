package replay

import (
	"os"
	"testing"
)

// TestMain clears the directory override this package resolves. Every
// HOME-based test here would otherwise fail in a shell that exports
// TAG_REPLAY_DIR, which is exactly what a multi-tenant deployment asks
// operators to do.
func TestMain(m *testing.M) {
	_ = os.Unsetenv(EnvReplayDir)
	os.Exit(m.Run())
}
