# JSON Contract

`tag template info --format json` and `tag scaffold --format json` carry two root-level keys
that no other `--format json` command emits:

| Key | Type | Description |
|-----|------|--------------|
| `schema_version` | integer | Contract version of this document. Currently `1` for both commands. |
| `tag_version` | string | Version of the binary that produced the document. Same value `tag version --format json` reports (under its own `version` key) for the same binary: `dev` for a plain `go build`, `dev-<sha>` for `make build`, a semver (e.g. `v2.3.0`) for a release. |

These two commands are the only ones that carry these keys. The other 20 JSON-emitting commands
are deliberately untouched — extending the keys to them is a separate decision, not an oversight.

## Not an envelope

`schema_version` and `tag_version` are root-level members added beside a command's existing
fields, not a wrapper around them. TAG still rejects the `{"ok": true, "data": {...}}` envelope
shape everywhere, without exception — see the package doc on `internal/jsonout` for the full
envelope convention every JSON-emitting command follows. Adding a root-level key to one document
does not change that convention or open the door to a generic envelope.

## Independent versioning

`template info` and `scaffold` version independently. Each has its own constant
(`infoSchemaVersion`, `scaffoldSchemaVersion` in `internal/commands`), so bumping one document's
contract never bumps the other's.

## Bump policy

| Change | Breaking? | Bumps `schema_version`? |
|--------|-----------|--------------------------|
| Adding a new key | No | No |
| Removing a key | Yes | Yes |
| Renaming a key | Yes | Yes |
| Changing a value's type | Yes | Yes |
| Changing a value's meaning (same key, same type, different semantics) | Yes | Yes |

A consumer pinned to a `schema_version` can rely on every key present at that version continuing
to exist, keep its name, keep its type, and keep its meaning. New keys can appear in a later
release without a bump; a consumer that ignores unknown keys is forward-compatible by
construction.

## See Also

- [tag template info](../commands/template.md#tag-template-info) — `keywords`, `categories`,
  `resolved_commit`, `depends_on`, and the full `--format json` example
- [tag scaffold](../commands/scaffold.md#machine-readable-output) — the scaffold `--format json`
  example
- `internal/jsonout` package doc — the envelope convention this document assumes
