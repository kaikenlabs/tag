# tag undo

Revert files created or modified by a previous `tag generate` or `tag scaffold`.

## Synopsis

```bash
tag undo [flags]
```

## Description

`tag undo` reads the generation history manifest (`.tag/history.json`) to revert the most recent generation. Created files are deleted; injected, appended, overwritten, or OpenAPI-merged files are restored from backups stored in `.tag/history/backups/`.

**Conflict detection**: Before reverting, `undo` compares the current hash of each file against the hash recorded at generation time. If a file has been modified after generation, `undo` refuses to overwrite it.

## Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--list` | | Show generation history (newest first) and exit |
| `--id <id>` | `-i` | Undo a specific generation by ID instead of the last one |
| `--yes` | `-y` | Skip the confirmation prompt |
| `--force` | `-f` | Override conflict detection and revert even modified files |
| `--partial` | | Revert unmodified files; silently skip conflicting ones |

## Examples

```bash
# View what can be undone
tag undo --list

# Undo the last generation (shows preview + confirmation)
tag undo

# Undo without prompting
tag undo --yes

# Undo a specific generation
tag undo --id gen_1741000000_a3f2bc

# Force-revert even if files were modified
tag undo --force

# Partial revert: only unmodified files
tag undo --partial
```

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Generation reverted successfully |
| `1` | Conflict detected (use `--force` or `--partial`) |
| `1` | Generation ID not found |
| `1` | No generations to undo |

## History Manifest

The manifest is stored at `.tag/history.json`. Each entry contains:

- **ID**: Unique generation ID (`gen_<unix>_<6hex>`)
- **Timestamp**: When the generation was recorded (UTC)
- **Template**: Generator or bundle name
- **Files**: List of affected files with `action`, `hash_before`, and `hash_after`

**Actions**:
- `create` — file was newly created (undo deletes it)
- `inject` — content was injected at a marker (undo restores backup)
- `append` — content was appended to a file (undo restores backup)
- `overwrite` — an existing file was replaced in full, e.g. via `--on-existing overwrite` (undo restores backup)
- `openapi-merge` — an OpenAPI fragment was merged into an existing spec file, e.g. via `action: openapi` (undo restores backup)

## Notes

- Only one undo is possible per generation entry. After undo, the entry is removed from the manifest.
- Scaffold generations are recorded in the *output project's* `.tag/history.json`, not the template's.
- The `--list` output is ordered newest first.
