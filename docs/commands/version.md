# tag version

Display version information and check for updates.

## Synopsis

```bash
tag version [flags]
```

## Description

The `tag version` command displays the current TAG version. With the `--check` flag, it also checks whether a newer version is available.

## Flags

| Flag | Description |
|------|-------------|
| `--check` | Check if a newer version is available |
| `--format <fmt>` | Output format: `text` (default) or `json` |

## Examples

### Show Current Version

```bash
tag version
# v2.3.0
```

### Check for Updates

```bash
tag version --check
# Current version: v2.3.0
# Latest version:  v2.4.1
# Update available! Run: go install github.com/kaikenlabs/tag@latest
```

If you are already on the latest version:

```bash
tag version --check
# Current version: v2.4.1
# You are up to date.
```

### Machine-Readable Output

```bash
tag version --format json
# {"version": "v2.3.0", "dev_build": false}

tag version --check --format json
# {"version": "v2.3.0", "dev_build": false, "latest": "v2.4.1", "update_available": true}
```

Without `--check`, only `version` and `dev_build` are present — `latest` and `update_available` are omitted entirely (not `false`), because no check ran to produce a value for them. Plain `tag version --format json` never makes a network call.

A dev build reports `dev_build: true`. Under `--check`, a dev build reports `update_available: false` without making a network call — a dev build has no real "latest" to compare against — and `latest` stays omitted.

If `--check` cannot reach the network, the command aborts with a text error on stderr and a non-zero exit code; no JSON is written.

## See Also

- [Getting Started](../getting-started.md) - Installation instructions
