package engine

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestUT_DiffPromptEnabled_OnlyWhenSinkIsRealTerminal is the only test that can
// distinguish the fixed dry-run prompt gate from the bug it replaces.
//
// The prompt used to be gated on the PROCESS's stdout being a terminal, without
// regard for where the diff was actually being written. That is wrong twice
// over: `tag generate --dry-run --format json` writes the diff to io.Discard yet
// would still stop and wait for y/n/a/q on stdin that nobody is going to type,
// and a test or a pipe holding a *bytes.Buffer would prompt too.
//
// No test anywhere in this repo can reach isTerm == true for real: under
// `go test` os.Stdout is a file or a pipe, and under a subprocess whose output
// is captured it is a pipe as well. That is exactly why the terminal check is
// injected here rather than called directly — asserting on the fd the gate asks
// about is what catches the plausible typo (os.Stdin.Fd() for os.Stdout.Fd()),
// which no end-to-end test could see.
//
// Not parallel: it swaps a package-level var.
func TestUT_DiffPromptEnabled_OnlyWhenSinkIsRealTerminal(t *testing.T) {
	tests := []struct {
		name   string
		out    io.Writer
		isTerm bool
		want   bool
	}{
		{"real stdout on a terminal prompts", os.Stdout, true, true},
		{"real stdout not on a terminal does not prompt", os.Stdout, false, false},
		{"buffer on a terminal does not prompt", &bytes.Buffer{}, true, false},
		{"io.Discard on a terminal does not prompt", io.Discard, true, false},
		{"nil sink does not prompt", nil, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotFd uintptr
			var asked bool

			orig := isTerminalFd
			isTerminalFd = func(fd uintptr) bool {
				gotFd = fd
				asked = true
				return tt.isTerm
			}
			t.Cleanup(func() { isTerminalFd = orig })

			assert.Equal(t, tt.want, diffPromptEnabled(tt.out))

			// The gate must ask about the sink it is actually writing to.
			// Asking about os.Stdin's fd would pass every behavioural test in
			// the suite while reintroducing the hang.
			if asked {
				assert.Equal(t, os.Stdout.Fd(), gotFd, "the terminal check must be made against stdout's fd")
			}
		})
	}
}
