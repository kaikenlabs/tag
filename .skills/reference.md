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
| `indent` | `indent(width, first?)` | Indent lines by N spaces (skip first line by default; `true` to include first) |

#### Dialect Type-Mapping

| Filter | Signature | Description |
|--------|-----------|-------------|
| `to` | `to("dialect")` | Map canonical type to dialect-specific type |

**Built-in dialects:** `go`, `postgres`, `mysql`, `typescript`, `openapi`, `protobuf`

**Canonical types:** `string`, `text`, `int`, `int32`, `int64`, `float`, `float32`, `float64`, `bool`, `byte`, `bytes`, `uuid`, `datetime`, `date`, `decimal`, `json`

```jinja
{{ "uuid" | to("postgres") }}     → UUID
{{ "datetime" | to("go") }}       → time.Time
{{ field.type | to("go") | upper }} → chaining works
```

**OpenAPI dialect (`to("openapi")`)** — special built-in that maps Go types to multi-line OpenAPI YAML fragments. Unlike other dialects, this returns structured YAML (type + format + nullable) rather than a single string. Supports Go primitives, pointers (`*T` → nullable), slices (`[]T` → array), `time.Time`, `uuid.UUID`, and `[]byte`.

```jinja
{{ "int" | to("openapi") }}       → type: integer\nformat: int64
{{ "*string" | to("openapi") }}   → type: string\nnullable: true
{{ "[]uuid.UUID" | to("openapi") }} → type: array\nitems:\n  type: string\n  format: uuid
```

Override built-in mappings by placing YAML files in `_dialects/` within a template. Three-tier loading: built-in → user-global (`~/.local/share/tag/dialects/`) → template-local.

CLI: `tag dialect list`, `tag dialect show <name>`

### Global Functions

| Function | Description |
|----------|-------------|
| `now()` | Current time in RFC3339 format |
| `now("20060102150405")` | Current time with Go format layout |
| `range(stop)` | Integer range (Gonja built-in) |
| `dict(key=val)` | Create dict (Gonja built-in) |

**Common `now()` patterns:**

| Template | Output |
|----------|--------|
| `{{ now("20060102150405") }}` | `20260321143022` (migration number) |
| `{{ now("2006-01-02") }}` | `2026-03-21` (date only) |
| `{{ now("2006-01-02T15:04:05Z07:00") }}` | RFC3339 |
| `{{ now() }}` | RFC3339 (default) |

Format uses Go's `time.Format` layout (reference time: `Mon Jan 2 15:04:05 MST 2006`).

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

### Conditional File Generation

File and directory names can use `{% if %}` blocks to conditionally include or exclude entire files from output. When the condition evaluates to false, the path renders to an empty string and the file is skipped entirely — it is never created.

```
my-template/
  {{ vars.project_name | snake }}/
    main.go
    {% if vars.use_docker %}Dockerfile{% endif %}
    {% if vars.use_docker %}docker-compose.yml{% endif %}
    {% if vars.use_ci %}.github/{% endif %}
```

- `vars.use_docker = true` → `Dockerfile` and `docker-compose.yml` are generated
- `vars.use_docker = false` → both files are skipped completely
- `vars.use_ci = false` → the `.github/` directory is skipped entirely

**Go file caveat**: If you use conditional *content* (rather than a conditional filename) to gate an entire `.go` file, the false path produces an empty file which fails compilation. Either use a conditional filename to skip the file, or provide a fallback:

```go
{%- if vars.use_grpc %}
package server

func StartGRPC() { ... }
{%- else %}
package server
{%- endif %}
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

## OpenAPI input

Drive `tag generate` from an OpenAPI 3.x contract. Two modes: a **single operation** →
`vars.operation`, or **many operations** → `vars.operations` (an ordered list).

```
# single operation
tag generate <gen> <name> --openapi ./api.yaml --operation getUserById
tag generate <gen> <name> --openapi ./api.yaml --operation "GET /users/{id}"

# many operations
tag generate <gen> <name> --openapi ./api.yaml --operations              # every operation
tag generate <gen> <name> --openapi ./api.yaml --operation-tag users     # operations tagged "users"
```

- `--openapi <path>` is required for any OpenAPI input, together with **exactly one selection
  mode**: `--operation` (single), `--operations` (all), or `--operation-tag <name>` (tag filter).
- `--operation` is **mutually exclusive** with the multi-op selectors (error if combined).
- `--operations` and `--operation-tag` are **not** exclusive: `--operations` turns on whole-spec
  mode and `--operation-tag` narrows it to a tag, so `--operations --operation-tag users` is
  equivalent to `--operation-tag users` alone. (`--operation-tag` already implies multi-op mode.)
- **Single-op selector** — primary key is `operationId`; fallback is `"METHOD /path"`
  (case-insensitive method). Not-found or ambiguous selectors hard-error listing the available
  operations.
- **Tag filter** — matches the OpenAPI `tags` on each operation, **case-sensitive**. A tag that
  matches nothing (or an empty spec) hard-errors listing the available operations **and** tags.
- Parses 3.0 **and** 3.1 (via `libopenapi`); `$ref` is resolved. Malformed/non-3.x specs surface
  the parser error.

### `vars` shape

The selected operation lands in five reserved namespaces:

```
vars.operation
  ├── operationId, method (upper-case), path, summary, description
  ├── tags          []string
  ├── parameters    [] { name, in, required, description, schema }   # path-level + operation-level, operation wins
  ├── requestBody   { required, description, content{ mediaType -> Schema } }
  └── responses     { "200"|"default"|… -> { description, content{ mediaType -> Schema } } }
vars.schemas   map[name]Schema   # component schemas referenced by the operation, deref'd, deduped
vars.info      { title, version, description }
vars.servers   [] { url, description }
vars.security  []                # operation-level if present (even []=no auth), else spec-level

Schema (recursive, raw OpenAPI):
  { type, format, nullable, required[], enum[], default, description,
    ref, items, properties{name->Schema}, composition{allOf|oneOf|anyOf: []} }
```

- A `$ref` **inlines its body once**; a `$ref` nested inside that body is a **leaf** — it carries
  only `ref: "<Name>"`, and the full body lives once under `vars.schemas.<Name>`. Walk
  `vars.schemas` to resolve nested types.
- OpenAPI 3.1 `type: [T, "null"]` normalizes to `type: T` + `nullable: true`. A non-null union
  (`type: [string, integer]`) collapses to the first type.
- `allOf`/`oneOf`/`anyOf` are exposed raw under `composition` (not flattened/merged).
- **Precedence:** the five keys are reserved — they win over `.tagconfig.json` / template vars on
  collision (a warning is logged). An explicit `--meta` still overrides them (scalar only; `--meta`
  cannot reach nested OpenAPI data).

### Gotcha: the `in` field

`in` is a Gonja keyword, so `param.in` fails to parse. Access it with subscript:

```jinja
{% for p in vars.operation.parameters %}
// {{ p.name }} ({{ p['in'] }}): {{ p.schema.type }}
{% endfor %}
```

### Worked example

```jinja
---
to: {{ name | snake }}_handler.go
---
package handlers

// {{ vars.operation.operationId }} — {{ vars.operation.method }} {{ vars.operation.path }}
func {{ vars.operation.operationId | pascal }}(
{%- for p in vars.operation.parameters %}
    {{ p.name }} {{ p.schema.type | to('go') }},  // {{ p['in'] }}
{%- endfor %}
) {}
```

### Multi-operation (`vars.operations`)

`--operations` / `--operation-tag` expose an **ordered list** instead of a single operation. The
current engine renders one template body to one `to:` path, so the template **loops** and emits a
single file (one `routes.go`, one client, etc.) — there is no file-per-operation emission yet.

```
vars.operations   [] {                 # sorted by path, then HTTP method (deterministic)
  operationId, method, path, summary, description, tags[],
  parameters[], requestBody, responses,   # same shape as vars.operation above
  security                                # effective auth for THIS op (op-level, else spec-level)
}
vars.schemas   map[name]Schema   # deduped UNION of components referenced by all selected operations
vars.info      { title, version, description }
vars.servers   [] { url, description }
```

- There is **no top-level `vars.security`** in multi-op mode — auth differs per operation, so it
  travels inside each `vars.operations[]` entry instead.
- `operations` is a reserved namespace (it wins over template/config vars on collision; a warning
  is logged). An explicit `--meta` still overrides it.

```jinja
---
to: {{ name | snake }}_routes.go
---
package routes
{% for op in vars.operations %}
// {{ op.operationId | pascal }} — {{ op.method }} {{ op.path }}
func {{ op.operationId | pascal }}() {}
{% endfor %}
```

## Hooks

### Phases

| Phase | Working dir | When | On failure |
|-------|-------------|------|------------|
| `pre_scaffold` | Template dir | Before generation | Fatal — stops |
| `post_scaffold` | Output dir | After generation | Warning only |
| `pre_generate` | Project dir | Before codegen | Fatal |
| `post_generate` | Project dir | After codegen | Warning only |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `TAG_TEMPLATE_DIR` | Absolute path to template directory |
| `TAG_OUTPUT_DIR` | Absolute path to project root |
| `TAG_PROJECT_NAME` | Value of `project_name` variable |
| `TAG_VAR_<NAME>` | Each variable as `TAG_VAR_` + UPPER_SNAKE |
| `TAG_GENERATOR_NAME` | Generator or bundle name being run (generate hooks only) |
| `TAG_TARGET_NAME` | Positional name argument (generate hooks only) |

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

Comments (`{# ... #}`), `{% raw %}...{% endraw %}` bodies, and string literals inside a `{{ }}` / `{% %}` block are never scanned for references — `{{ replace("{{ vars.ghost }}") }}` does not reference `ghost`. A `{% raw %}` tag's opening tag is scanned normally; only its body is skipped. Same reference rules as `rename-var` below, including the `vars["name"]` subscript limitation and `vars.0` being index access, not a variable named `0`.

Exit codes: `0` = pass, `1` = lint errors, `2` = usage error.

### Variable Auditing

```bash
tag template variables                 # Audit current directory
tag template vars ./path/to/template   # Audit specific template (vars is an alias)
tag template variables --format json   # Machine-readable output
tag template variables --strict        # Non-zero exit on issues (for CI)
```

Cross-references declared variables in `tag.template.json` with usage in templates:
- Lists declared variables with usage counts and file locations
- Detects undeclared variables used in templates
- Detects declared but unused variables
- Scans generator-level configs inside `_generators/`
- Comments, `{% raw %}...{% endraw %}` bodies, and string literals inside a `{{ }}` / `{% %}` block are never scanned — a variable referenced only inside one of them counts as unused, not used. Same reference rules as `lint` above and `rename-var` below.

Exit codes: `0` = no issues (or non-strict), `1` = issues found (`--strict`), `2` = usage error.

### Variable Renaming

```bash
tag template rename-var --dry-run old_name new_name   # Preview every change
tag template rename-var old_name new_name             # Apply in current directory
tag template rename-var old_name new_name ./template  # Apply to a specific template
```

Flags must precede the positional arguments.

Rewrites the declaration in `tag.template.json`, derived defaults, hook commands, bundle and generator `requires` entries, all `{{ vars.* }}` / `{% ... vars.* ... %}` expressions, and file/directory name placeholders (renamed on disk).

Left untouched: plain text, comments (`{# ... #}`), the body of `{% raw %}` blocks (the opening `{% raw %}` tag itself is an ordinary block and is rewritten normally), string literals inside expressions, `.tagignore`d files, `_dialects/`, symlinks, and binary files. `_generators/` and `.tag/` are included.

Planning is read-only, so `--dry-run` cannot write. A failed apply rolls back every file and path already changed.

Only dot access is rewritten — `vars["old_name"]` is not recognised. A name must start with a letter or underscore: `vars.0` is index access, not a variable named `0`, and is never renamed.

Exit codes: `0` = applied or previewed, `1` = rename error (undeclared, name taken, path collision), `2` = usage error.

### Dependency Graph

```bash
tag template graph                     # Analyze current directory
tag template graph ./path/to/template  # Analyze a specific template
tag template graph --format json       # Machine-readable output
tag template graph --format dot | dot -Tpng -o graph.png
```

Flags must precede the positional argument.

Builds the implicit dependency graph between generators by reading each template's frontmatter:

- **Generators** — every generator with its actions (`create`, `inject`, `append`) and target paths. Inject actions also show the marker and clause, e.g. `[inject after "// ROUTES"]`.
- **Bundles** — each bundle's generator execution order, flagged `valid` or `INVALID ORDER`. Order is invalid when a generator injects into a file that a *later* generator in the same bundle creates.
- **Injection markers** — the marker strings actually found in the project's source files, with line numbers and the generators that reference them. Skips `.tag/`, `_generators/`, dotfiles, and binary files.

Warnings (`code` in JSON output):

| Code | Meaning |
|------|---------|
| `file_conflict` | Two or more generators create the same target file |
| `missing_target` | A generator injects into a file no generator creates (often fine — the file may come from the scaffold) |
| `order_violation` | A bundle injects into a file before the generator that creates it |
| `malformed_metadata` | A template's frontmatter could not be extracted or parsed |

Targets containing `{{ ... }}` are skipped by `missing_target`, since they cannot be resolved statically.

Exit code is `0` even when warnings are present — `graph` reports, it does not gate. Use `2` for usage errors (unknown `--format`, more than one path argument).

### Cache Management

```bash
tag cache list                         # Show cached templates
tag cache clear                        # Clear expired entries
tag cache clear --all                  # Clear entire cache
```

### Bundle Prerequisites

Bundles and generators can declare a `requires` field — an array of `.tagconfig.json` variable names that must be present and truthy for the generator/bundle to run.

**In a bundle manifest** (`.tag/_bundles/<name>/<name>.json`):

```json
{
  "name": "crud",
  "vars": {
    "domain": "tenant"
  },
  "requires": ["use_db"],
  "generators": [
    { "name": "model" },
    { "name": "repository" }
  ]
}
```

**In a generator config** (`tag.template.json` inside the generator directory):

```json
{
  "requires": ["use_docker"],
  "vars": { "port": 8080 }
}
```

When `tag generate` is invoked and requirements are unmet, it aborts with a message listing each unmet variable and whether it is "not set" or "currently disabled":

```
generator "service" requires the following variables to be enabled:
  - use_db (not set in .tagconfig.json)
  hint: re-scaffold with the required variables enabled to use this generator
```

`tag generate list` and `tag template list` hide items with unmet requirements by default. Use `--all` to show everything. Items with requirements display them as `[requires: x, y]` in the list output.

### Generator Info

```bash
tag generate info <name>
```

Outputs JSON metadata for a generator or bundle. Useful for AI agents and tooling.

**Generator output** includes: name, type (`"generator"`), description, source (`"template"` or `"local"`), variables (with types, prompts, defaults, options), hooks, template files (with `to` paths and actions), requires, and usage string.

**Bundle output** includes: name, type (`"bundle"`), description, generators list, self_contained flag, requires, and usage string.

Template file `to` paths contain raw template expressions (e.g. `{{ name | snake }}.go`) since no target name is available at info time.

### Agent File

```bash
tag generate agent-file <format> [-o <path>]
```

Generates a reference file for AI coding agents listing available generators and bundles.

**Formats**:

| Format | Default path |
|--------|-------------|
| `claude` | `CLAUDE.md` |
| `cursor` | `.cursorrules` |
| `windsurf` | `.windsurfrules` |
| `copilot` | `.github/copilot-instructions.md` |

**Flags**:

| Flag | Description |
|------|-------------|
| `-o <path>` | Override default output path |

**Behavior**:
- Creates file with `<!-- tag:generators:start -->` / `<!-- tag:generators:end -->` markers
- Re-running replaces content between markers (idempotent)
- Appending to existing file without markers preserves existing content
- Creates parent directories if needed (e.g. `.github/` for copilot format)

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

### Template Lifecycle (Check, Diff, Update)

Commands for keeping scaffolded projects in sync with upstream template changes.

```bash
tag check                              # Check if upstream has newer commits (exit 0/1)
tag check --quiet                      # CI mode: exit code only, no output
tag check --ref main                   # Check against a specific branch/tag

tag diff                               # Show unified diff of proposed changes
tag diff --stat                        # Show compact diffstat summary
tag diff --no-color                    # Pipe-friendly (no ANSI)
tag diff --ref v2.0.0                  # Diff against a specific ref

tag update                             # Apply upstream template changes (3-way merge)
tag update --dry-run                   # Preview changes without applying
tag update --accept-ours               # Auto-resolve conflicts with your version
tag update --accept-theirs             # Auto-resolve conflicts with template version
tag update --set author="Jane"         # Override variables during update
tag update --set go_version=1.22       # Pre-answer new required variables (CI)
tag update --skip "*.md"               # Skip patterns for this run
tag update --backup=false              # Skip backup creation
tag update --skip-hooks                # Skip all hook execution
tag update --accept-hooks              # Run changed hooks without prompting
tag update --continue                  # Resume after manual conflict resolution
tag update --abort                     # Abort and restore from backup
```

**How update works**: Renders the template at the old commit SHA and the new commit SHA (both with your variables), reads your current project files, then performs a 3-way merge (base=old template, ours=your files, theirs=new template). Conflicts are written with standard `<<<<<<<`/`=======`/`>>>>>>>` markers.

**Variable changes**: When the template introduces new variables, optional ones use their defaults automatically. New required variables without defaults need values via `--set key=value`. Removed variables are cleaned from `.tagconfig.json`. Default-only changes keep your stored value.

**Hook changes**: When template hooks are added or modified, the update displays their content and prompts for execution. Use `--skip-hooks` to suppress all execution, or `--accept-hooks` to auto-execute. Non-interactive mode skips hooks by default. Changed hooks receive `TAG_UPDATE_MODE=true` in their environment.

**Binary files**: Binary files (detected by null bytes in first 8KB) cannot be text-merged. When both sides modify a binary file, the update prompts for a choice. Use `--accept-ours` or `--accept-theirs` to auto-resolve. Binary changes are identified by SHA256 hash.

**Backup/rollback**: By default, `tag update` creates a backup in `.tag/backup/{timestamp}/` with a `manifest.json` tracking which files were modified, deleted, or added. On `--abort`, modified/deleted files are restored and newly-added files are removed. Backups older than 30 days are auto-cleaned. Use `--backup=false` to skip backup creation.

**Conflict workflow**: If conflicts occur, resolve them manually in the affected files, then run `tag update --continue` to finalize. Or run `tag update --abort` to restore from backup.

**`.tagconfig.json` tracking**: After a successful update, `.tagconfig.json` is updated with the new `commit` SHA. The `tag check` command compares this SHA against the latest remote commit.

#### CI Integration

**GitHub Actions — freshness check (weekly):**
```yaml
name: Template Freshness
on:
  schedule:
    - cron: '0 9 * * 1'
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install TAG
        run: go install github.com/kaikenlabs/tag@latest
      - name: Check template
        run: tag check --quiet
```

**GitHub Actions — auto-update PR:**
```yaml
name: Template Update
on:
  schedule:
    - cron: '0 9 * * 1'
jobs:
  update:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install TAG
        run: go install github.com/kaikenlabs/tag@latest
      - name: Update template
        run: |
          tag update --accept-theirs --skip-hooks --backup=false
          if [ -n "$(git status --porcelain)" ]; then
            git checkout -b chore/template-update
            git add -A && git commit -m "chore: update project template"
            gh pr create --title "chore: update project template" \
              --body "Automated template update via \`tag update\`."
          fi
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### Matrix Testing

```bash
tag test [template-dir]                   # Test current directory (or specified path)
```

Discovers boolean variables in `tag.template.json`, generates all 2^N combinations, scaffolds each one in an isolated temp directory, and optionally runs validation commands. Useful for verifying templates work correctly with all boolean flag permutations before publishing.

**Flags**:

| Flag | Description |
|------|-------------|
| `-p` / `--parallel N` | Max concurrent test runs (default: 4) |
| `-m` / `--meta key=value` | Required variable override (can be repeated) |
| `--values <file>` | JSON file with variable values |
| `--skip-var <name>` | Exclude boolean var from permutation (can be repeated) |
| `--pin key=value` | Fix a variable to a specific value (can be repeated) |
| `--run <command>` | Validation command, overrides template config (can be repeated) |
| `--filter <expr>` | Filter combinations by index or key=value pairs |
| `--case <name>` | Run only the test case with this name |
| `--fail-fast` | Stop on first failure |
| `--dry-run` | List combinations without running tests |
| `--keep-failed` | Keep scaffolded directories on failure for debugging |
| `--timeout <duration>` | Per-command timeout (default: 5m) |
| `--max-cases N` | Safety limit for total combinations, 0 = unlimited (default: 64) |
| `--format text\|json` | Output format (default: text) |
| `-v` / `--verbose` | Show full command output on failures |
| `--accept-hooks` | Run hooks and template-defined test commands |

**Template test configuration** (`tag.template.json`):

```json
{
  "vars": { "...": "..." },
  "test": {
    "project_name": "test-scaffold",
    "env": { "CGO_ENABLED": "0" },
    "cases": [
      {
        "name": "Full test",
        "filters": { "use_docker": true, "use_postgres": true },
        "commands": ["go build ./...", "go vet ./...", "golangci-lint run ./..."]
      },
      {
        "name": "Light test",
        "commands": ["go build ./..."]
      }
    ]
  }
}
```

- `cases`: Array of named test cases, each with optional `filters` and `commands`.
  - `name`: Identifier for the test case (used with `--case` flag).
  - `filters`: Boolean variable pins for this case (e.g., `{"use_docker": true}` runs only combinations where `use_docker=true`).
  - `commands`: Validation commands run after each successful scaffold. Requires `--accept-hooks` to execute.
- `project_name`: Project name for scaffold output (default: `test-scaffold`), shared across all cases.
- `env`: Environment variables passed to validation commands, shared across all cases.

Use `--run` to provide commands directly without `--accept-hooks`:

```bash
tag test . --run "go build ./..." --run "go vet ./..."
```

**Pinning and skipping**:

```bash
tag test . --pin use_s3=false             # Fix use_s3, permute others
tag test . --skip-var use_clickhouse      # Use default, don't permute
```

**Filtering**:

```bash
tag test . --filter 7                     # Run only combination index 7
tag test . --filter use_postgres=true     # Only combos with postgres enabled
tag test . --filter "use_postgres=true,use_amqp=true"
```

**Exit codes**: `0` = all passed, `1` = failures, `2` = errors (config/setup).

**JSON output**: Use `--format json` for machine-parseable output (no text header, statuses as strings: `"passed"`, `"failed"`, `"errored"`).

### Self-Upgrade

```bash
tag upgrade                            # Download and install latest release
tag version --check                    # Check if update available
```

Downloads the platform-appropriate binary from GitHub Releases, verifies its SHA256 checksum, and replaces the current binary in-place.
