# JSON Contract

`tag template info --format json` and `tag scaffold --format json` carry two root-level keys
that no other `--format json` command emits:

| Key | Type | Description |
|-----|------|--------------|
| `schema_version` | integer | Contract version of this document. Currently `1` for both commands. |
| `tag_version` | string | Version of the binary that produced the document. Same value `tag version --format json` reports (under its own `version` key) for the same binary: `dev` for a plain `go build`, `dev-<sha>` for `make build`, a semver (e.g. `v2.3.0`) for a release. |

These two commands are the only ones that carry these keys. The other 20 JSON-emitting commands
are deliberately untouched — extending the keys to them is a separate decision, not an oversight.

## Error documents

When `tag template info --format json` or `tag scaffold --format json` fails, they emit a document
in this shape instead of their normal success document — never a mix of the two:

```json
{
  "schema_version": 1,
  "tag_version": "dev",
  "error": {
    "code": "template_not_found",
    "message": "not a TAG template: tag.template.json not found in /path/to/dir",
    "exit_code": 1
  }
}
```

`schema_version` and `tag_version` carry the same meaning and the same per-command constant
(`infoSchemaVersion`, `scaffoldSchemaVersion`) as the success document above — this is still the
same two commands, not a third document type. `error.code` is one of a fixed, stable vocabulary
(`invalid_reference`, `template_not_found`, `auth_required`, `version_not_found`,
`required_variable_missing`, `output_exists`, `circular_dependency`, `usage`, `internal`);
`error.message` is the human-readable error text also written to stderr as `tag error: <message>`;
`error.exit_code` mirrors the process exit code. No other command emits an `error` key.

If a command with a nonzero exit already writes a complete document before failing (`check`,
`generate`, `undo`, `update`), that document is unaffected by this contract — it is not an error
document and carries no `error` key.

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
