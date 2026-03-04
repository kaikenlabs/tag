# TAG Reference

Detailed reference for template syntax, variable system, hooks, and remote templates.

## Template Syntax

TAG uses **Gonja** (Go port of Jinja2). Templates use `{{ vars.* }}` namespace.

### Filters

#### Case Transforms

| Filter | Input | Output |
|--------|-------|--------|
| `snake` | `UserService` | `user_service` |
| `pascal` | `user_service` | `UserService` |
| `camel` | `user_service` | `userService` |
| `kebab` | `user_service` | `user-service` |
| `lower` | `UserService` | `userservice` |
| `upper` | `UserService` | `USERSERVICE` |
| `title` | `user service` | `User Service` |

Aliases: `snake_case`, `pascal_case`, `camel_case`, `kebab_case`

#### Inflection

| Filter | Input | Output |
|--------|-------|--------|
| `plural` / `pluralize` | `service` | `services` |
| `singular` / `singularize` | `services` | `service` |
| `past` / `past_tense` | `OrderCancel` | `OrderCancelled` |
| `humanize` | `user_service` | `User service` |
| `titleize` | `user_service` | `User Service` |
| `ordinalize` | `3` | `3rd` |

The `past` filter handles irregular verbs, consonant doubling, and preserves casing style.

#### String Operations

| Filter | Signature | Description |
|--------|-----------|-------------|
| `replace` | `replace(old, new)` | Replace all occurrences |
| `split` | `split(delim?)` | Split string (default: whitespace) |
| `join` | `join(sep?)` | Join list to string |
| `trim` | `trim(cutset?)` | Trim whitespace or cutset |
| `contains` | `contains(substr)` | Returns boolean |
| `hasprefix` | `hasprefix(prefix)` | Returns boolean |
| `hassuffix` | `hassuffix(suffix)` | Returns boolean |
| `default` | `default(value)` | Fallback if nil/empty |
| `truncate` | `truncate(len, ellipsis?)` | Truncate with "..." |

### String Methods

Python-style method calls on strings:

```jinja2
{{ vars.name.lower() }}
{{ vars.name.upper() }}
{{ vars.name.replace("old", "new") }}
{{ vars.name.replace("a", "b", 1) }}    {# with count #}
{{ vars.name.startswith("prefix") }}
```

### Control Structures

```jinja2
{% if vars.use_docker %}...{% endif %}
{% for item in vars.features %}...{% endfor %}
{% if vars.db == "postgres" %}...{% elif vars.db == "mysql" %}...{% else %}...{% endif %}
```

### Includes

Shared fragments in `_shared/` resolve by basename:

```jinja2
{% include "header.tmpl" %}   {# resolves _shared/header.tmpl #}
```

### Whitespace Control

```jinja2
{%- if condition -%}    {# strips surrounding whitespace #}
{{- value -}}
```

### Comments

```jinja2
{# This won't appear in output #}
```

## Scaffold Template Configuration

### tag.template.json

```json
{
  "name": "my-template",
  "description": "A Go web service template",
  "version": "1.0.0",
  "vars": {
    "project_name": "my-project",
    "author": {
      "type": "string",
      "prompt": "Who is the author?",
      "required": true
    },
    "license": {
      "type": "choice",
      "prompt": "Select a license",
      "options": ["MIT", "Apache-2.0", "GPL-3.0"],
      "default": "MIT"
    },
    "use_docker": {
      "type": "boolean",
      "prompt": "Include Docker support?",
      "default": true
    },
    "port": {
      "type": "number",
      "default": 8080
    },
    "_slug": "{{ vars.project_name | snake }}"
  },
  "hooks": {
    "pre_scaffold": ["echo 'Starting...'"],
    "post_scaffold": ["go mod tidy", "git init"]
  }
}
```

### Variable Definition Formats

**Short form** — just a default value:

```json
{ "vars": { "project_name": "my-project", "use_docker": true, "port": 8080 } }
```

**Long form** — full definition:

```json
{
  "vars": {
    "author": {
      "type": "string",
      "prompt": "Who is the author?",
      "default": "Anonymous",
      "required": true,
      "secret": false
    }
  }
}
```

### Variable Types

| Type | JSON type | Prompt behavior |
|------|-----------|-----------------|
| `string` | `"value"` | Text input |
| `boolean` | `true`/`false` | Yes/No confirmation |
| `number` | `123` | Numeric input |
| `choice` | requires `options` | Selection from list |

### Special Variable Kinds

**Private** (prefix `_`): Not prompted, internal use.

```json
{ "_internal_slug": "{{ vars.project_name | snake }}" }
```

**Derived**: Minimal form with expression default. Not prompted, computed from other variables.

```json
{
  "display_name": "My Package",
  "package_name": "{{ vars.display_name | lower | replace(' ', '_') }}"
}
```

Only `display_name` is prompted; `package_name` is computed.

**Evaluated default**: Expanded form with explicit `prompt` AND an expression default. Prompted interactively — the expression is resolved first and shown as the suggested default. User can accept or override.

```json
{
  "project_name": "my-service",
  "module_path": {
    "type": "string",
    "prompt": "Go module path",
    "default": "bitbucket.org/myorg/{{ vars.project_name }}"
  }
}
```

In non-TTY mode the expression is resolved silently (same as derived).

**Dependency ordering**: Variables are prompted in dependency order — if `module_path`'s default references `{{ vars.project_name }}`, then `project_name` is prompted first regardless of alphabetical order. Positional arguments and `--meta` overrides are applied before prompting, so evaluated defaults see CLI-provided values.

**Circular dependencies**: If variable defaults form a cycle (e.g., `A` references `B` and `B` references `A`), TAG reports a clear error.

### Variable Resolution Priority

From lowest to highest:

1. Default values from `tag.template.json`
2. Replay values (`--replay`)
3. Values file (`--values`)
4. Interactive prompts (if TTY)
5. `--meta` / `-m` flags (highest priority)

### Path Placeholders

Directory and file names support template expressions:

```
{{ vars.project_name | snake }}/
  {{ vars.project_name | snake }}_test.go
```

### .tagignore

Excludes files from scaffold output using gitignore syntax. Place in template root.

```
# Exclude authoring tools
.serena/
CLAUDE.md
.mcp.json
.vscode/
*.log
```

- Standard gitignore patterns (globs, `**/`, negation with `!`)
- `.tagignore` itself is always excluded from output
- Directories matching a pattern are pruned entirely

### Bundled Generators

Templates can include generators in `_generators/` — copied to scaffolded project's `.tag/`.

## Hooks

### Phases

| Phase | Working dir | When | On failure |
|-------|-------------|------|------------|
| `pre_scaffold` | Template dir | Before generation | Fatal — stops |
| `post_scaffold` | Output dir | After generation | Warning only |
| `pre_generate` | Project dir | Before codegen | Fatal |
| `post_generate` | Project dir | After codegen | Fatal |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `TAG_TEMPLATE_DIR` | Absolute path to template directory |
| `TAG_OUTPUT_DIR` | Absolute path to project root |
| `TAG_PROJECT_NAME` | Value of `project_name` variable |
| `TAG_VAR_<NAME>` | Each variable as `TAG_VAR_` + UPPER_SNAKE |

### Execution Rules

- Commands support `{{ vars.* }}` expressions (rendered before execution)
- Parsed with POSIX shell quoting (shlex) — no pipes/redirects by default
- For shell features: `sh -c 'echo hello | grep hello'`
- Script files auto-detected by extension (`.py` → python, `.sh` → sh)
- Shebangs respected if executable bit set
- 5-minute timeout, 1MB output limit

### Confirmation Behavior

- Interactive: User prompted to confirm
- `--accept-hooks`: Run without prompting
- `--no-input` without `--accept-hooks`: Hooks skipped

## Remote Templates

### Reference Formats

| Format | Example |
|--------|---------|
| GitHub shorthand | `gh:user/repo` |
| GitLab shorthand | `gl:user/repo` |
| Bitbucket shorthand | `bb:user/repo` |
| With version | `gh:user/repo@v1.0.0` |
| With subpath | `gh:user/repo/templates/go-api` |
| Version + subpath | `gh:user/repo@v2.0.0/templates/go-api` |
| HTTPS URL | `https://github.com/user/repo.git` |
| SSH URL | `git@github.com:user/repo.git` |
| Zip URL | `https://example.com/template.zip` |
| Local path | `./my-template` or `/absolute/path` |

### Authentication

| Provider | Env var | Method |
|----------|---------|--------|
| GitHub | `GITHUB_TOKEN` | Basic auth (`x-access-token`) |
| GitLab | `GITLAB_TOKEN` | Basic auth (`x-access-token`) |
| Bitbucket | `BITBUCKET_TOKEN` | Bearer token |
| SSH | SSH agent or default keys | SSH key |

### Library Management

```bash
tag lib add gh:user/template           # Install
tag lib add gh:user/template --as name # Custom name
tag lib ls                             # List
tag lib edit my-template               # Open in editor
tag lib update my-template             # Re-fetch
tag lib update                         # Update all
tag lib rm my-template                 # Remove
```

Cookiecutter templates are auto-detected and converted when added.

### Template Linting

```bash
tag template lint                      # Lint current directory
tag template lint ./path/to/template   # Lint specific template
tag template lint --format json        # Machine-readable output for CI
```

Validates:
- `tag.template.json` against JSON Schema
- Gonja template syntax (parse-only, no execution)
- `{{ vars.* }}` references against declared variables

Exit codes: `0` = pass, `1` = lint errors, `2` = usage error.

### Cache Management

```bash
tag cache list                         # Show cached templates
tag cache clear                        # Clear expired entries
tag cache clear --all                  # Clear entire cache
```

### Code Generation Flags

| Flag | Description |
|------|-------------|
| `--on-existing <policy>` | How to handle create-action files that already exist: `fail` (default — atomic, no writes if any conflict), `skip` (silently skip existing files), `overwrite` (replace existing files) |
| `-v` / `--verbose` | Print per-file operation details (created/skipped/overwritten/modified) after generation |
| `--dry-run` / `-d` | Preview what would be written without touching the filesystem. Behavior differs by command — see below. |
| `-m key=value` | Set variable values inline |

**`--dry-run` behavior by command**:

- **`tag generate --dry-run`**: Renders templates and displays a colored unified diff for each file (green `+` additions, red `-` deletions). On a TTY, each diff is followed by a `[y]es/[n]o/[a]ll/[q]uit` prompt. `y`/`n` advance to the next file; `a` skips remaining prompts; `q` exits immediately. Nothing is written regardless of input. Hooks are not executed.
- **`tag scaffold --dry-run`**: Lists each file path that would be created as `(dry-run) would write: <path>`, including binary files. No output directory is created.

**`--on-existing` behavior**:

- `fail` (default): Pre-scans all create-action targets before writing anything. If any conflict is found, the entire generation is aborted with no files written (atomic).
- `skip`: Writes new files, silently skips files that already exist.
- `overwrite`: Replaces existing files. Overwrites are recorded in generation history with a pre-modification backup, enabling `tag undo`.

### Generation History & Undo

Every `tag generate` and `tag scaffold` records a manifest entry in `.tag/history.json` with SHA256 hashes of affected files. `tag undo` uses this manifest to safely revert changes.

**Overwrite history**: When `--on-existing=overwrite` is used, the pre-modification content is snapshotted and recorded as an `overwrite` action with `hash_before`, enabling full undo support.

```bash
tag undo                               # Revert the last generation (with confirmation)
tag undo --yes                         # Skip confirmation prompt
tag undo --list                        # Show generation history (newest first)
tag undo --id gen_1741000000_a3f2bc    # Revert a specific generation by ID
tag undo --force                       # Override conflict detection
tag undo --partial                     # Revert unmodified files, skip conflicting ones
```

**Conflict detection**: If a file was modified after generation was recorded, `undo` refuses to overwrite it with a clear error. Use `--force` to override or `--partial` to skip conflicting files.

**Manifest location**: `.tag/history.json` — generated automatically, do not edit manually.

**Backup location**: `.tag/history/backups/<generation-id>/` — stores pre-modification copies for inject/append operations.

### Self-Update

```bash
tag update                             # Download and install latest release
tag version --check                    # Check if update available
```

Downloads the platform-appropriate binary from GitHub Releases, verifies its SHA256 checksum, and replaces the current binary in-place.
