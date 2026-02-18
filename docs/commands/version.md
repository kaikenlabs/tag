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

## See Also

- [Getting Started](../getting-started.md) - Installation instructions
