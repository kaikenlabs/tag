// Package jsonout centralises the encoder policy for `--format json` output.
//
// Every command that can emit JSON writes it through Write, so the wire shape
// (two-space indent, one trailing newline, Go's default HTML escaping) is
// decided in exactly one place rather than re-derived at each call site.
//
// Write deliberately does no normalisation of the value it is given. Callers
// own their own shape — in particular, a slice that must serialise as `[]`
// rather than `null` is built with make([]T, 0, n) where it is assembled, not
// patched up on the way out.
//
// The envelope convention across commands, so the next one does not invent a
// fifth: a command whose result is a LIST wraps it under a single key naming
// the noun — {"entries": [...]}, {"templates": [...]}, {"results": [...]},
// {"dialects": [...]}. A command whose result is a single thing or a report
// emits that object bare, with no wrapper — `check`, `dialect show`, and the
// lint/vars/graph reports all do this. There is deliberately no {"ok":true,
// "data":...} envelope and no schema_version: four commands shipped bare
// objects before this convention was written down, and wrapping them now would
// either break those or leave two conventions in the tree forever.
package jsonout

import (
	"encoding/json"
	"io"
)

// Write encodes v as indented JSON followed by a newline.
func Write(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
