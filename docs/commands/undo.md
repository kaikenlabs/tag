# tag undo

Revert files created or modified by a previous `tag generate` or `tag scaffold`.

## Synopsis

```bash
tag undo [flags]
```

## Description

`tag undo` reads the generation history manifest (`.tag/history.json`) to revert the most recent generation. Created files are deleted; injected, appended, overwritten, or OpenAPI-merged files are restored from backups stored in `.tag/history/backups/`.

**Conflict detection**: Before reverting, `undo` compares the current hash of each file against the hash recorded at generation time. If a file has been modified after generation, `undo` refuses to overwrite it.

`tag undo` takes no positional arguments. Select a generation with `--id`; a stray token (e.g. `tag undo gen_1741000000_a3f2bc` without `--id`) is a usage error rather than a silent fallback to the last generation.

## Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--list` | | Show generation history (newest first) and exit |
| `--id <id>` | `-i` | Undo a specific generation by ID instead of the last one |
| `--yes` | `-y` | Skip the confirmation prompt |
| `--force` | `-f` | Override conflict detection and revert even modified files |
| `--partial` | | Revert unmodified files; silently skip conflicting ones |
| `--format <fmt>` | | Output format: `text` (default) or `json`. `--format json` requires `--yes` — it refuses rather than auto-confirming a destructive operation. Also applies to `--list` |

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

# Machine-readable output (requires --yes)
tag undo --yes --format json
tag undo --list --format json
```

## Machine-Readable Output

`tag undo --format json` requires `--yes`; without it the command refuses rather than treating JSON mode as implicit consent for a destructive operation:

```bash
tag undo --format json          # error: --format json requires --yes
tag undo --yes --format json    # OK
```

```json
{
  "gen_id": "gen_1741000000_a3f2bc",
  "files": [
    { "path": "internal/model/widget.go", "action": "create", "reverted": true }
  ],
  "reverted": 1,
  "skipped": 0
}
```

On a conflict (no `--force`/`--partial`), the document is still written — `files` is empty and `conflicts` carries the affected paths — and the command still exits non-zero. Under `--partial`, a skipped-but-conflicting file appears in both `conflicts` and as a `"reverted": false` entry in `files`.

`--list --format json` wraps the history under a `generations` key:

```json
{
  "generations": [
    { "id": "gen_1741000000_a3f2bc", "template": "handler", "command": "generate", "file_count": 3 }
  ]
}
```

`file_count` is a scalar, deliberately not named `files`: in `tag undo`'s own document (and everywhere else in TAG) `files` is a per-file **array**, so reusing the key for a number here would give the same command two incompatible shapes for one key. `template` is omitted when empty (e.g. a scaffold generation records `command` instead).

In JSON mode, the confirmation preview and any other human-facing text are written to stderr, so stdout stays a single parseable document.

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Generation reverted successfully |
| `1` | Conflict detected (use `--force` or `--partial`) |
| `1` | Generation ID not found |
| `1` | No generations to undo |
| `2` | Usage error: unsupported `--format` value, a positional argument was given, or `--format json` was used without `--yes` |

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
