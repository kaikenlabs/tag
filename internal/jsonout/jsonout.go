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
// "data":...} envelope: four commands shipped bare objects before this
// convention was written down, and wrapping them now would either break those
// or leave two conventions in the tree forever.
//
// The no-envelope rule still holds without exception. The no-schema_version
// rule that used to sit alongside it does not, and the distinction is worth
// stating precisely because the two are easy to conflate:
//
//   - An ENVELOPE wraps the result in a generic container, so every document
//     shares one outer shape and the payload moves under a key. Still rejected.
//   - A ROOT-LEVEL KEY on an otherwise bare, command-specific document adds one
//     more member beside the others. Different thing, and permitted for exactly
//     two documents.
//
// `tag template info` and `tag scaffold` carry schema_version (int) and
// tag_version (string) at their document root. Epic #388 added them so a
// Backstage plugin pinned to a contract can detect the binary's version without
// a second process spawn. The other 20 JSON-emitting commands are deliberately
// untouched — extending the keys to them is a separate, larger decision, and
// TestUT_VersionKeys_ScopedToTwoDocuments fails the build if one drifts into a
// third document. Bump policy: docs/reference/json-contract.md.
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
