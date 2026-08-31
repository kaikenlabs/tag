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

CLI: `tag dialect list` (`--format json` → `{"dialects": [{"name","description"}]}`, no `source` field — the registry doesn't track provenance), `tag dialect show <name>` (`--format json` → bare `{"name","description","types"}`)

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
5. The positional `[project-name]` argument (sets `project_name` only)
6. `--meta` / `-m` flags (highest priority)

**The positional `[project-name]` has two roles**: it *defaults* the `project_name`
variable (an explicit `-m project_name=...` overrides it, per the priority above), and it
*drives the output directory* when `--output` is not given. So
`tag scaffold ./tmpl out3 -m project_name="Demo App"` scaffolds into `out3/` while
`project_name` is `Demo App`.

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
- Only the template root's `.tagignore` is read and excluded from output; patterns match against paths relative to the template root, including the wrapper segment of a project-wrapper template
- A `.tagignore`, `tag.template.json` or `_meta.json` placed inside the wrapper instead of at the template root is content, not metadata — it gets generated into the project
- Directories matching a pattern are pruned entirely

### Bundled Generators

Templates can include generators in `_generators/`. What happens to them at scaffold time depends on whether the scaffold recorded a library origin in `.tagconfig.json`:

- **No library origin** (local template with no `--add-to-lib`, or `--no-library`): `_generators/` is copied into the scaffolded project's `.tag/`, so the project is self-contained.
- **Library origin** (the common case — a remote scaffold, or `--add-to-lib`, records `template.name`): generators are NOT copied. `tag generate` resolves them directly from the library entry, which `tag lib add` stores verbatim (it never rewrites `_generators/` to `.tag/`). Resolution probes the library entry's `.tag/` directory first, then `_generators/`; a candidate only counts as a match if it holds at least one template file (an empty or template-less directory is skipped in favor of the other root, or a same-named bundle), and `.tag/` wins a same-name collision between the two when both actually hold one. The `_shared/` templates directory is probed the same way, independently of which root the generator itself matched in. `tag template new generator --lib` / `tag template new bundle --lib` write into whichever of those two roots already exists (`.tag/` if present, else `_generators/`, else `.tag/` is created).

See [Generator Resolution](../docs/commands/generate.md#generator-resolution) for the full library-vs-local precedence rules `tag generate` uses.

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
| `TAG_PROJECT_NAME` | Value of `project_name` variable (omitted when the template declares none) |
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

Applies to `pre_scaffold`/`post_scaffold` only, and keys on interactivity — not on whether the
template is local or remote:

- Interactive: user prompted once for the whole set; declining, or having no TTY, skips them
- `--accept-hooks`: accepted without prompting (still not executed under `--dry-run`)
- `--no-input` (implied by `--format json`) without `--accept-hooks`: hooks skipped

`pre_generate`/`post_generate` are never confirmed — they come from the project's own
`.tagconfig.json`. `tag generate` runs them unless `--no-hooks` is passed.

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
tag lib add gh:user/template           # Install; auto-derives "template-<12-hex-digest>"
tag lib add gh:user/template --as name # Custom name, no digest — follow-on commands use "name"
tag lib add --as name gh:user/template # Flags may come before or after the ref
tag lib ls                             # List (NAME column is never truncated)
tag lib ls --format json               # Machine-readable output
tag lib edit name                      # Open in editor
tag lib edit name --editor vim         # Flags may come before or after the name too
tag lib update name                    # Re-fetch
tag lib update                         # Update all
tag lib rm name                        # Remove
```

Without `--as`, the digest makes the name collision-free: two refs that share a basename (two
orgs each publishing `service-template`) land in different slots. The digest excludes the
version, so `repo@v1` and `repo@v2` derive the same name and `tag lib update` re-fetches that one
slot in place — `--as` is how you keep two versions side by side. A local ref
(`tag lib add ./my-template`) is unaffected: its name is still the bare directory basename. See
[docs/commands/lib.md#library-naming](../docs/commands/lib.md#library-naming) for the full rule.

Cookiecutter templates are auto-detected and converted when added.

### Template Search

```bash
tag lib search kubernetes                    # Search GitHub for templates
tag lib search kubernetes --limit 5          # Cap results (default: 10, max 100)
tag lib search --sort updated --order asc    # Sort by stars/forks/updated (default: stars desc)
tag lib search --format json                 # Machine-readable output ({"results": [...]})
```

The query is variadic (all non-flag arguments are joined with spaces), so flags may appear before or after it: `tag lib search foo --limit 5` treats `--limit` as a flag, not as part of the query text.

A dash-prefixed token that doesn't match a known flag is a usage error rather than being folded into the query. To search for a literal dash-prefixed term, put it after another argument and a `--` separator: `tag lib search go -- -language:java`. A leading `--` (`tag lib search -- -x`) does not work — urfave/cli consumes it before TAG sees it.

Exit codes: `0` = success (including zero results), `1` = search request failed, `2` = usage error (unsupported `--format` value, unrecognised flag).

### Template Info

```bash
tag template info <template>                # Human-readable metadata
tag template info gh:user/awesome-template  # Works with any template reference (local, library, remote)
tag template info <template> --update       # Force refresh of cached remote templates
tag template info <template> --format json  # Machine-readable output
tag template info --format json <template>  # Flags may come before or after the template argument
```

Bare JSON object (no envelope): `{"name","description","version","variables":[{"name","type","prompt","default","required","options","secret","prompted","derived","private","default_is_expression"}],"hooks":{"pre_scaffold":[],"post_scaffold":[]},"has_readme","has_howto"}`.

```json
{
  "name": "go-api",
  "description": "Go REST API template",
  "version": "v1.2.0",
  "variables": [
    { "name": "_build_stamp", "type": "string", "default": "{{ vars.author }}", "required": false, "secret": false,
      "prompted": false, "derived": true, "private": true, "default_is_expression": true },
    { "name": "author", "type": "string", "prompt": "Author name", "required": true, "secret": false,
      "prompted": true, "derived": false, "private": false, "default_is_expression": false },
    { "name": "license", "type": "choice", "required": true, "options": ["MIT", "Apache-2.0", "GPL-3.0"], "secret": false,
      "prompted": true, "derived": false, "private": false, "default_is_expression": false },
    { "name": "port", "type": "number", "default": 8080, "required": false, "secret": false,
      "prompted": true, "derived": false, "private": false, "default_is_expression": false },
    { "name": "service_name", "type": "string", "default": "{{ vars.author }}-svc", "required": false, "secret": false,
      "prompted": false, "derived": true, "private": false, "default_is_expression": true }
  ],
  "hooks": {
    "pre_scaffold": [],
    "post_scaffold": ["go mod tidy", "git init"]
  },
  "has_readme": true,
  "has_howto": false
}
```

`variables` is sorted by name and reports the resolved variable definitions — the same values the text output shows — not the raw declarations from `tag.template.json`. `hooks` always carries both `pre_scaffold` and `post_scaffold`, `[]` when a phase has no hooks. `has_readme`/`has_howto` are booleans only: README/HOWTO content is never included, and the glamour-rendered ANSI of the text view never appears in JSON. There is deliberately no `source` field. `--update` works the same in JSON mode as in text mode. Each variable also carries four booleans describing what TAG does with it at scaffold time, so a form generator can decide whether to render an input: `private` (name starts with `_`), `derived` (the default is a template expression and no explicit `prompt` is declared), `prompted` (`!private && !derived` — the variable is asked for interactively), and `default_is_expression` (the `default` is raw template source rather than a literal, whether the variable is derived or has a prompt with an evaluated default). All four are always present, never omitted when `false`. `default` is reported verbatim: `info` has no values to resolve expressions against, so `default_is_expression` is what tells a consumer not to render the default literally.

`tag template info` takes exactly one template argument; a second positional is a usage error (exit `2`), not silently ignored.

On failure, `--format json` writes a single JSON error document to stdout instead of the success document above — see [Conventions](#conventions) below and [Error documents](../docs/reference/json-contract.md#error-documents) for the shape and the `error.code` vocabulary.

### Template Linting

```bash
tag template lint                      # Lint current directory
tag template lint ./path/to/template   # Lint specific template
tag template lint --format json        # Machine-readable output for CI
tag template lint ./path --format json # Flags may come before or after the path
```

Validates:
- `tag.template.json` against JSON Schema
- Gonja template syntax (parse-only, no execution)
- `{{ vars.* }}` references against declared variables

Comments (`{# ... #}`), `{% raw %}...{% endraw %}` bodies, and string literals inside a `{{ }}` / `{% %}` block are never scanned for references — `{{ replace("{{ vars.ghost }}") }}` does not reference `ghost`. A `{% raw %}` tag's opening tag is scanned normally; only its body is skipped. Same reference rules as `rename-var` below: dot access (`vars.name`) and literal subscript access (`vars["name"]` / `vars['name']`) both count as references, a non-literal subscript (`vars[expr]`) does not, and `vars.0` is index access, not a variable named `0`.

Exit codes: `0` = pass, `1` = lint errors, `2` = usage error.

```json
{
  "issues": [
    { "file": "bad.txt", "line": 1, "severity": "error", "message": "undefined variable \"undefined_thing\"", "rule": "undefined-variable" }
  ]
}
```

`{"issues":[...]}`, `[]` on a clean template. `line`/`column` are `omitempty` — a schema or
config-parse issue has no line to point to, so they're absent rather than `0`. `severity` is
always `"error"` or `"warning"`; `rule` is the machine-readable name shown in parentheses in the
text output.

### Variable Auditing

```bash
tag template variables                 # Audit current directory
tag template vars ./path/to/template   # Audit specific template (vars is an alias)
tag template variables --format json   # Machine-readable output
tag template variables --strict        # Non-zero exit on issues (for CI)
tag template variables ./path --format json # Flags may come before or after the path
```

Cross-references declared variables in `tag.template.json` with usage in templates:
- Lists declared variables with usage counts and file locations
- Detects undeclared variables used in templates
- Detects declared but unused variables
- Scans generator-level configs inside `_generators/`
- Comments, `{% raw %}...{% endraw %}` bodies, and string literals inside a `{{ }}` / `{% %}` block are never scanned — a variable referenced only inside one of them counts as unused, not used. Same reference rules as `lint` above and `rename-var` below.

Exit codes: `0` = no issues (or non-strict), `1` = issues found (`--strict`), `2` = usage error.

Bare object, no envelope, keyed by scope:

```json
{
  "root": {
    "scope": "root",
    "declared": [
      {
        "name": "project_name",
        "type": "string",
        "default": "demo-proj",
        "file_count": 2,
        "reference_count": 2,
        "references": [
          { "file": "README.md", "line": 1, "expression": "hello {{ vars.project_name }}" }
        ]
      }
    ],
    "undeclared": [
      { "name": "feature", "references": [{ "file": "cond/{% if vars.feature %}feat.txt{% endif %}", "line": 0, "expression": "" }] }
    ],
    "unused": [],
    "summary": { "declared": 1, "undeclared": 1, "unused": 0 }
  },
  "generators": []
}
```

`root` is a `ScopeResult`; `generators` is `[]ScopeResult`, one entry per generator-level config under `_generators/`, `[]` when none exist. `declared`/`undeclared`/`unused`/`generators` are always arrays, `[]` when empty, never `null`. An `undeclared` reference found in a path placeholder (rather than a template body) reports `line: 0` and an empty `expression`, as in the example above. `required`/`default`/`options`/`derived`/`private` on a `declared` entry are all `omitempty` — present only when true/non-zero. `summary` mirrors the array lengths so a script can gate on it without counting the arrays itself.

### Variable Renaming

```bash
tag template rename-var --dry-run old_name new_name   # Preview every change
tag template rename-var old_name new_name             # Apply in current directory
tag template rename-var old_name new_name ./template  # Apply to a specific template
tag template rename-var old_name new_name --dry-run   # Flags may come before or after the positionals
```

Rewrites the declaration in `tag.template.json`, derived defaults, hook commands, bundle and generator `requires` entries, all `{{ vars.* }}` / `{% ... vars.* ... %}` expressions, and file/directory name placeholders (renamed on disk).

Left untouched: plain text, comments (`{# ... #}`), the body of `{% raw %}` blocks (the opening `{% raw %}` tag itself is an ordinary block and is rewritten normally), string literals inside expressions, `.tagignore`d files, `_dialects/`, symlinks *inside the template*, and binary files. `_generators/` and `.tag/` are included.

Planning is read-only, so `--dry-run` cannot write. A failed apply rolls back every file and path already changed.

Dot access (`vars.old_name`) and literal subscript access (`vars["old_name"]` / `vars['old_name']`, whitespace-tolerant) are both rewritten; a non-literal subscript (`vars[expr]`) is left alone since its key is not statically known. A name must start with a letter or underscore: `vars.0` is index access, not a variable named `0`, and is never renamed.

Exit codes: `0` = applied or previewed, `1` = rename error (undeclared, name taken, path collision), `2` = usage error.

### Dependency Graph

```bash
tag template graph                     # Analyze current directory
tag template graph ./path/to/template  # Analyze a specific template
tag template graph --format json       # Machine-readable output
tag template graph ./path --format json # Flags may come before or after the path
tag template graph --format dot | dot -Tpng -o graph.png
```

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
tag cache list --format json           # Machine-readable output ({"entries": [{"key","meta"}]})
tag cache clear                        # Clear expired entries
tag cache clear --all                  # Clear entire cache
```

```json
{
  "entries": [
    {
      "key": "gh_acme_go-api-template",
      "meta": {
        "original_ref": "gh:acme/go-api-template",
        "resolved_url": "https://github.com/acme/go-api-template.git",
        "version": "v1.2.0",
        "commit_sha": "6d8a871b7e14d750a02830d4a3688d7a4b08c4b0",
        "fetched_at": "2026-08-19T16:49:14.467828+02:00",
        "expires_at": "2026-08-20T16:49:15.796465+02:00"
      }
    }
  ]
}
```

`entries` is `[]`, never `null`, on an empty cache. `meta` is `null` if the entry's metadata file is
missing or corrupt, rather than the command erroring. `version`/`commit_sha` are `omitempty`
(zip/local sources have no commit SHA); `expires_at` is `omitempty` — absent means the entry is
pinned and never expires. `cache list --format json` redacts query strings from cached URLs
(`original_ref`/`resolved_url` become `...?[redacted]`) so presigned-URL credentials aren't
printed; the text table never showed these URLs at all, so it is unaffected.

The cache directory defaults to `~/.tag/cache` and can be overridden with the
`TAG_CACHE_DIR` environment variable — must be an absolute path, or TAG errors naming the
variable. It is checked before `$HOME` is resolved, so it also works when `$HOME` is unset
or unwritable (containers/sandboxes). No directory is created until the first cache write —
constructing the resolver alone touches nothing on disk. `tag cache clear --all` only removes
directories TAG itself wrote (identified by a `_meta.json` file), so pointing `TAG_CACHE_DIR`
at a directory holding other data will not delete that data. It also leaves alone any
`.staging-*` directory less than 24 hours old — that's an in-progress cache write (from this or
another concurrent `tag` process), not a bug. An older `.staging-*` directory is debris from a
crashed run and is removed.

**Multi-tenant deployments** (e.g. a shared service running `tag` on behalf of multiple
tenants, such as a Backstage scaffolder integration): a missing `TAG_CACHE_DIR` silently
falls back to the single shared cache, and with it cross-tenant template disclosure — one
tenant's cached remote template can be served to another. Set `TAG_CACHE_DIR` explicitly per
tenant and fail the caller's own startup if it is unset.

The replay-file directory defaults to `~/.tag/replay` and follows the identical contract via
`TAG_REPLAY_DIR`: absolute path or hard error, read before `$HOME` is resolved. See
[docs/commands/scaffold.md](../docs/commands/scaffold.md#replay-system).

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

`tag generate list --format json` and `tag template list --format json` emit the identical shape: `{"generators":[{"name","description","requirements_met"}],"bundles":[{"name","description","generators":[...],"requirements_met"}]}`. `generators` and `bundles` are always arrays, `[]` when empty, never `null`. A hidden (unmet-requirements) entry is absent from the array exactly as it is from the text listing; `--all` includes it with `requirements_met:false`, mirroring the text output's `[requires: x]` suffix.

There is deliberately no per-generator `"bundle"` field. The data model records bundle → members, never the reverse, so a single owning bundle can't be substantiated — a generator may belong to several bundles or none. Which bundle owns a generator is derivable exactly from `bundles[].generators`, the same reasoning that keeps a `source` field out of `dialect list --format json`.

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
tag generate agent-file claude              # Writes CLAUDE.md
tag generate agent-file claude -o docs/AGENTS.md   # Override output path
tag generate agent-file -o docs/AGENTS.md claude   # Flags may come before or after the format
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

### Cookiecutter Conversion

```bash
tag convert cookiecutter ./cookiecutter-myproject -o ./myproject-tag
tag convert cookiecutter ./cookiecutter-myproject --dry-run
tag convert cookiecutter ./cookiecutter-myproject -o ./output --force
tag convert cookiecutter ./cookiecutter-myproject --format json
```

Converts `cookiecutter.json` to `tag.template.json`, rewrites `{{ cookiecutter.var }}`
path/content references to `{{ vars.var }}` (in place — there is no `__var__`
double-underscore form), and reports any Jinja2 constructs with no Gonja equivalent as
`incompatibilities` for manual review.

**`--format json`**: bare `convert.Result`, no envelope. Beyond the existing counters
(`variables_converted`, `dirs_renamed`, `files_renamed`, `files_processed`,
`hooks_copied`), it adds two arrays: `files` (each converted file's `from`/`to` path) and
`variables` (each variable's `name`, `original_type`, `tag_type`, and — for choice/private
variables — `is_choice`/`is_private`). All array fields (`incompatibilities`, `warnings`,
`files`, `variables`) are always present as `[]`, never omitted or `null`, even when
empty. `severity` on an incompatibility is a plain string (`"info"`/`"warning"`/`"error"`).
`internal/convert` performs no output of its own — the JSON is written by the command
layer either way, so nothing needs redacting.

### Code Generation Flags

| Flag | Description |
|------|-------------|
| `--on-existing <policy>` | How to handle create-action files that already exist: `fail` (default — atomic, no writes if any conflict), `skip` (silently skip existing files), `overwrite` (replace existing files) |
| `-v` / `--verbose` | Print per-file operation details (created/skipped/overwritten/modified) after generation |
| `--dry-run` / `-d` | Preview what would be written without touching the filesystem. Behavior differs by command — see below. |
| `-m key=value` | Set variable values inline |

**`--dry-run` is a global flag**, declared on the app rather than on `generate` itself, so
it must appear *before* the subcommand (`tag --dry-run generate screen Settings`) or
*after* the positional arguments (`tag generate screen Settings --dry-run`) — never
between the subcommand name and the positionals (`tag generate --dry-run screen Settings`
fails with `flag provided but not defined: -dry-run`, because that is still where
urfave/cli's own parser looks for command-declared flags only). The trailing form works
because `generate` rescans the tail of the argument list for recognised flags — the same
mechanism that makes a trailing `--format json` work. Same rules apply to `-d`.

**`--dry-run` behavior by command**:

- **`tag generate --dry-run`**: Renders templates and displays a colored unified diff for each file (green `+` additions, red `-` deletions). On a TTY, each diff is followed by a `[y]es/[n]o/[a]ll/[q]uit` prompt. `y`/`n` advance to the next file; `a` skips remaining prompts; `q` exits immediately. Nothing is written regardless of input. Hooks are not executed.
- **`tag scaffold --dry-run`**: Lists each file path that would be created as `(dry-run) would write: <path>`, including binary files. No output directory is created. A remote scaffold (or a local one with `--add-to-lib`) that would otherwise add the template to the shared library instead prints `(dry-run) would add template to library as "<name>"` and writes no library entry — text mode only, see below. A remote scaffold that would otherwise create or refresh `<cwd>/.tag/lock.json` instead prints `(dry-run) would pin <ref> in .tag/lock.json` to stderr — in both text and JSON mode, since this line always goes to stderr rather than through a JSON-mode writer — and a checksum mismatch against an existing entry still fails the run, same as a real run. A detected Cookiecutter template is refused outright, on every path including a TTY, pointing at `tag convert cookiecutter <ref>` instead of prompting to convert.

**`--on-existing` behavior**:

- `fail` (default): Pre-scans all create-action targets before writing anything. If any conflict is found, the entire generation is aborted with no files written (atomic).
- `skip`: Writes new files, silently skips files that already exist.
- `overwrite`: Replaces existing files. Overwrites are recorded in generation history with a pre-modification backup, enabling `tag undo`.

**`tag generate --format json`**: bare object, no envelope.

```bash
tag generate model User --format json
tag generate --dry-run crud Product --format json
```

```json
{
  "files": [
    { "path": "internal/model/widget.go", "action": "create" },
    { "path": "internal/router.go", "action": "inject" }
  ],
  "created": 1,
  "skipped": 0,
  "overwritten": 0,
  "modified": 1,
  "dry_run": false
}
```

`action` is the real per-file action — `inject` and `append` stay distinct, unlike the
`--verbose` text summary's `modified` word which deliberately collapses both. `files` is
`[]`, never `null`, on a run that touches nothing. In JSON mode, `--dry-run` never
prompts on stdin regardless of terminal state (the interactive diff review is a
text-mode-only feature), hook output and a failed post-generation hook's warning are
written to stderr instead of stdout, and the `--verbose` text summary is not printed. On
`--on-existing=fail` conflicts, the document still gets written — with a `conflicts`
array of the paths that already existed — and the command still exits non-zero; any
other failure exits non-zero with an empty stdout instead.

**`tag scaffold --format json`**: bare object, no envelope.

```bash
tag scaffold ./tmpl my-project --format json
tag scaffold ./tmpl my-project --dry-run --format json
```

```json
{
  "output_dir": "/abs/path/my-project",
  "project_root": "/abs/path/my-project",
  "template": "./tmpl",
  "files": [
    { "path": "README.md", "action": "create" },
    { "path": "src/main.go", "action": "create" }
  ],
  "created": 2,
  "dry_run": false
}
```

`project_root` is the directory that actually holds the generated project — hand that one
to anything that publishes or `cd`s into the result. It equals `output_dir` except for a
project-wrapper template (root is a single directory named by an expression, e.g.
`{{ vars.project_name }}/`) combined with an explicit `--output`, which deliberately does
not unwrap: the files land one level down and `output_dir` names the parent. `files[].path`
stays relative to `output_dir` in both shapes, so join file paths onto `output_dir`, never
onto `project_root`. A wrapper only unwraps when it holds *all* of the template's generated
content: a root with files beside the wrapper is written whole instead (nothing dropped),
`project_root` stays equal to `output_dir`, and scaffold warns naming the siblings — add them
to `.tagignore` to restore unwrapping. Walk `files[]` to publish a result rather than
archiving `project_root` wholesale.

`action` is always `"create"` — scaffold writes a fresh project tree, so it has no
inject/append/overwrite cases. `files` is identical whether `--dry-run` is set or not
(same paths, same action), because both paths record an entry at the same point right
after a file is processed; only whether the file actually lands on disk differs. A dry
run also skips the write to the shared template library (see the `--dry-run` behavior
by command list above), but under `--format json` that skip is silent: no "would add"
line, no field in the document.
`--format json` forces `--no-input` (defaults and `-m` overrides apply, prompts never
fire) and turns the no-template-argument interactive picker into a usage error (exit 2)
instead. Hook output, the "Add template to library?" prompt/messages, and the
post-scaffold summary/README render are all suppressed or rerouted to stderr — stdout
carries only the JSON document. On failure, that document is a JSON error document instead
of the success document above — see [Conventions](#conventions) below and
[Error documents](../docs/reference/json-contract.md#error-documents) for the shape and the
`error.code` vocabulary.

**`tag extract --format json`**: bare object.

```bash
tag extract --name user --as handler internal/handler/user_handler.go --format json
tag extract --name user --as handler --dry-run internal/handler/user_handler.go --format json
```

```json
{ "template_path": ".tag/handler/user_handler.go", "to_path": "internal/handler/{{ name }}_handler.go", "replacements": 4 }
```

`content` (the generated template body) is included only in `--dry-run` — nothing is on
disk yet, so it is the only way to see the result; a real run omits it, since the file is
already on disk and the field would otherwise be an unbounded duplicate of it.
`-i`/`--interactive` is rejected as a usage error under `--format json` rather than
silently running non-interactively.

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

`tag undo` takes no positional arguments — select a generation with `--id`, not a bare token. A stray positional (e.g. `tag undo gen_1741000000_a3f2bc`) is a usage error (exit `2`), not a silent fallback to the last generation.

**Manifest location**: `.tag/history.json` — generated automatically, do not edit manually.

**Backup location**: `.tag/history/backups/<generation-id>/` — stores pre-modification copies for inject/append/overwrite/openapi-merge operations; `undo` restores from these for all four.

**`tag undo --format json`**: requires `--yes` — JSON mode never implies consent for a
destructive operation, so `tag undo --format json` alone is a usage error, not an
auto-confirm. Bare object:

```bash
tag undo --yes --format json
tag undo --yes --partial --format json
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

On a conflict (a file was modified after generation and neither `--force` nor
`--partial` was passed), the document is still written — with `files` empty and
`conflicts` populated — and the command still exits non-zero. Under `--partial`,
skipped-but-conflicting files show up in both `conflicts` and as `"reverted": false`
entries in `files`. `--list` also emits JSON, wrapped under a noun key:
`tag undo --list --format json` → `{"generations":[{"id","template","command","file_count"}]}` (a count — `files` is reserved for the per-file array in `tag undo`'s own document)
(`files` here is the file *count*, not a per-file array).

### Environment Diagnostics

```bash
tag doctor                             # Human-readable health report
tag doctor --format json               # Machine-readable output ({"status","sections":[{"name","checks":[{"label","status","message"}]}]})
```

Runs checks across four sections — ENVIRONMENT, PROJECT, TEMPLATES, LIBRARIES — each check reporting a `pass`, `warn`, or `fail` verdict. `status` always serialises as one of those three strings, never as a number. `message` is omitted from a check's JSON when empty.

The top-level `status` is the worst status across every check, so a CI job can gate on that one field instead of walking `sections`.

Exit codes are unchanged by `--format` and fire after the JSON is written, so a JSON consumer always gets both a parseable body and the correct status: `0` = all checks pass, `1` = one or more warnings, `2` = one or more failures.

### Template Lifecycle (Check, Diff, Update)

Commands for keeping scaffolded projects in sync with upstream template changes.

```bash
tag check                              # Check if upstream has newer commits (exit 0/1)
tag check --quiet                      # CI mode: exit code only, no output
tag check --ref main                   # Check against a specific branch/tag
tag check --format json                # Machine-readable output ({"up_to_date","current_sha","latest_sha","source"})

tag diff                               # Show unified diff of proposed changes
tag diff --stat                        # Show compact diffstat summary
tag diff --no-color                    # Pipe-friendly (no ANSI)
tag diff --ref v2.0.0                  # Diff against a specific ref
tag diff --format json                 # Machine-readable output ({"old_sha","new_sha","source","files":[{"path","op","conflicted","is_binary","added","deleted"}]})

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

`tag update` takes no positional arguments; a stray token (e.g. `tag update stray`) is a usage error (exit `2`), the same rejection pattern as `tag diff` and `tag undo`.

**How update works**: Renders the template at the old commit SHA and the new commit SHA (both with your variables), reads your current project files, then performs a 3-way merge (base=old template, ours=your files, theirs=new template). Conflicts are written with standard `<<<<<<<`/`=======`/`>>>>>>>` markers.

**Variable changes**: When the template introduces new variables, optional ones use their defaults automatically. New required variables without defaults need values via `--set key=value`. Removed variables are cleaned from `.tagconfig.json`. Default-only changes keep your stored value.

**Hook changes**: When template hooks are added or modified, the update displays their content and prompts for execution. Use `--skip-hooks` to suppress all execution, or `--accept-hooks` to auto-execute. Non-interactive mode skips hooks by default. Changed hooks receive `TAG_UPDATE_MODE=true` in their environment.

**Binary files**: Binary files (detected by null bytes in first 8KB) cannot be text-merged. When both sides modify a binary file, the update prompts for a choice. Use `--accept-ours` or `--accept-theirs` to auto-resolve. Binary changes are identified by SHA256 hash.

**Backup/rollback**: By default, `tag update` creates a backup in `.tag/backup/{timestamp}/` with a `manifest.json` tracking which files were modified, deleted, or added. On `--abort`, modified/deleted files are restored and newly-added files are removed. Backups older than 30 days are auto-cleaned. Use `--backup=false` to skip backup creation.

**Conflict workflow**: If conflicts occur, resolve them manually in the affected files, then run `tag update --continue` to finalize. Or run `tag update --abort` to restore from backup.

**`.tagconfig.json` tracking**: After a successful update, `.tagconfig.json` is updated with the new `commit` SHA. The `tag check` command compares this SHA against the latest remote commit.

**`tag check --quiet` and `--format json`**: `--quiet` suppresses all output — including the JSON document — in both formats; only the exit code is left to inspect. Exit codes are unchanged by `--format`: `1` still fires when updates are available, after the JSON (when not `--quiet`) has been written.

**`tag diff --format json`**: bare object, no envelope.

```json
{
  "old_sha": "a1b2c3d",
  "new_sha": "e4f5a6b",
  "source": "gh:user/go-api-template",
  "files": [
    { "path": "main.go", "op": "update", "conflicted": false, "is_binary": false, "added": 3, "deleted": 1 },
    { "path": "assets/logo.png", "op": "update", "conflicted": false, "is_binary": true, "added": 0, "deleted": 0 },
    { "path": "config.yaml", "op": "conflict", "conflicted": true, "is_binary": false, "added": 0, "deleted": 0 }
  ]
}
```

`files` is `[]`, never `null`, when the project is already up to date, and the JSON is written unconditionally in that case rather than the text "Already up to date." sentence. `op` is one of `add`/`delete`/`update`/`conflict`/`prompt`; files needing no change (`keep`, user-added) are omitted from the array. Binary files are flagged with `is_binary` and always report `0`/`0` for `added`/`deleted` rather than having their bytes counted or dumped — no file contents ever appear in the output. `added`/`deleted` count the same +/- lines the text diff prints for that file, so a file whose content ends in a newline counts a trailing empty line; this is one more than `git diff --numstat` would report for the same file and is expected, not a bug. `--stat` and `--no-color` are accepted but have no effect under `--format json`. Exit codes are unchanged from text: unaffected by whether the project is up to date, non-zero only on error (e.g. `2` for a rejected positional argument — `tag diff` takes none).

**`tag update --format json`**: bare object. `mode` distinguishes `apply`/`continue`/`abort` — `--continue` and `--abort` never populate a file list, so `--abort` carries only `mode`/`dry_run` and `--continue` adds the SHAs.

```bash
tag update --format json
tag update --dry-run --format json
tag update --continue --format json
tag update --abort --format json
```

```json
{
  "mode": "apply",
  "dry_run": false,
  "old_sha": "a1b2c3d",
  "new_sha": "e4f5a6b",
  "up_to_date": false,
  "files": [
    { "path": "main.go", "op": "update" },
    { "path": "config.yaml", "op": "conflict" }
  ],
  "new_files": 0,
  "updated_files": 1,
  "deleted_files": 0,
  "conflicts": {
    "conflicted_files": ["config.yaml"],
    "prompt_files": [],
    "skipped": []
  }
}
```

`op` keeps `update`'s own vocabulary (`keep`/`add`/`delete`/`update`/`conflict`/`user-added`/`prompt`) rather than the `fileaction` vocabulary `generate`/`undo` use — a 3-way merge decision is a different concept from "what TAG wrote to this file". `conflicts` (reusing the `conflicted_files`/`prompt_files` names from `.tag/conflicts.json`, plus a new `skipped` list) is present only when there are real conflicts or prompts, and its presence is exactly when the command exits non-zero — the document is still written on that exit. No file body ever appears in the JSON: neither the merged content nor the base/ours/theirs content of a conflicted file, which are unbounded copies of your own source.

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
tag test ./my-template --format json      # Flags may come before or after the path
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

### Version

```bash
tag version                            # Print version only, no network call
tag version --check                    # Check for updates (network access required)
tag version --format json              # {"version","dev_build"}
tag version --check --format json      # Adds "latest" and "update_available" once the check runs
```

Without `--check`, the JSON is just `{"version","dev_build"}` — `latest` and `update_available` are omitted entirely, not `false`, because no check ran to produce a value for them. Plain `tag version --format json` never makes a network call.

A dev build (empty version, `"dev"`, or a `"dev-"` prefix) reports `dev_build:true`. Under `--check`, a dev build answers `update_available:false` without touching the network — there is no real "latest" to compare a dev build against — and `latest` stays omitted.

If `--check` cannot reach the network, the command aborts: a text error on stderr, a non-zero exit, and no JSON at all. It does not downgrade to reporting the failure inline (e.g. a `check_error` field) — deliberately, so a JSON consumer never has to distinguish "checked, nothing found" from "the check itself failed" inside the document.

## JSON Output Shapes

22 commands support `--format json`: `cache ls`, `check`, `convert cookiecutter`, `dialect
list`, `dialect show`, `diff`, `doctor`, `extract`, `generate`, `generate list`, `lib ls`, `lib
search`, `scaffold`, `template graph` (also `--format dot`), `template info`, `template lint`,
`template list`, `template variables`, `test`, `undo`, `update`, `version`. This list is the
golden fixture in `TestUT_FormatCommands_SurfaceMatchesGolden`
(`internal/commands/format_conformance_test.go`) — a command gaining or losing `--format` fails
that test, so it can't drift from this section silently.

### Conventions

These decisions were made once, across the whole epic, rather than per command:

- **No envelope, no `schema_version`.** A command that returns a list wraps it under a noun key
  matching its content (`{"entries":[...]}`, `{"templates":[...]}`, `{"dialects":[...]}`,
  `{"generations":[...]}`, `{"results":[...]}`, `{"generators":[...],"bundles":[...]}`). A command
  that returns one object or report emits it bare — no `{"ok":...,"data":...}` wrapper.
- **Empty arrays serialize as `[]`, never `null`.** `entries`, `templates`, `results`, `dialects`,
  `generations`, `files`, `incompatibilities`, `warnings`, `variables` — every array field holds to
  this, checked on the raw bytes in tests because `null` and `[]` both unmarshal to a nil Go slice.
- **`--format` is recognised on either side of a command's positional arguments.** `tag template
  info ./tpl --format json` and `tag template info --format json ./tpl` are equivalent — a
  trailing flag used to be silently dropped, so a command that takes positionals reparses the
  tail of the argument list for flags it didn't see the first time.
- **`--format json` implies non-interactive.** It selects the noop prompter: a command that would
  otherwise prompt errors instead. `scaffold --format json` forces `--no-input` semantics, never
  shows the interactive template picker (no template argument is a usage error instead), never
  prompts to convert a detected Cookiecutter template, and a required variable with no value is an
  error rather than a blocked prompt. `undo --format json` requires `--yes` outright — JSON mode
  is never read as implicit consent for a destructive operation.
- **Stdout carries exactly one JSON document.** Progress lines, hook output, and confirmation
  previews go to `c.App.ErrWriter` (stderr) in JSON mode, never to the real `os.Stdout`. A failing
  command still writes its document before returning the exit-code-carrying error where there is
  a meaningful partial result to report (a conflict, a warning list); a command that fails before
  producing one writes nothing — **except `tag template info` and `tag scaffold`, which always
  write an error document on failure instead of nothing; see the next bullet.**
- **Errors stay text on stderr; exit codes don't change with `--format`.** For 20 of the 22
  commands, `--format json` never turns an error into a JSON error object, and the process exit
  code for a given failure is the same in both formats.
- **`tag template info` and `tag scaffold` are the two exceptions: a failure in `--format json`
  mode writes an error document instead of nothing.** The document carries `schema_version`,
  `tag_version`, and an `error` object (`code`, `message`, `exit_code`); the same human-readable
  message is also written to stderr as a plain `tag error: <message>` line, without the
  `[HH:MM:SS.mmm]` prefix the text-mode logger normally adds (the JSON seam already reported it,
  so `main()` does not log it a second time). The process exit code is unchanged by `--format` for
  these two commands too. `error.code` is one of a fixed vocabulary — see
  `docs/reference/json-contract.md`.
- **An unknown `--format` value is a usage error, exit `2`, and it is validated first.**
  `resolveFormat` runs before a command validates its own arguments, so `tag template lint
  ./does-not-exist --format bogus` reports the format error, not "template not found" — a command
  never needs its own fixture set up correctly just to reject a bad `--format`.
- **Two file-action vocabularies, deliberately not unified.** `generate`, `scaffold`, and `undo`
  share `fileaction`'s outcome vocabulary (`create`/`inject`/`append`/`overwrite`/`openapi-merge`/`skip`)
  because all three report "what TAG wrote to a file". `update` reports its own `MergeOp`
  vocabulary (`keep`/`add`/`delete`/`update`/`conflict`/`prompt`/`user-added`) instead, because a
  3-way merge decision is a different kind of fact — which side won, or whether the two sides
  disagreed — not a write outcome.

### Shape index

Every command below emits a bare object/report unless noted as wrapped under a key. See the
linked section for the full field list and worked example.

| Command | Shape | Documented in |
|---------|-------|----------------|
| `cache ls` | `{"entries":[{"key","meta"}]}` | [Cache Management](#cache-management) |
| `check` | bare `{"up_to_date","current_sha","latest_sha","source"}` | [Template Lifecycle (Check, Diff, Update)](#template-lifecycle-check-diff-update) |
| `convert cookiecutter` | bare `convert.Result` | [Cookiecutter Conversion](#cookiecutter-conversion) |
| `dialect list` | `{"dialects":[{"name","description"}]}` | [Dialect Type-Mapping](#dialect-type-mapping) |
| `dialect show` | bare `{"name","description","types"}` | [Dialect Type-Mapping](#dialect-type-mapping) |
| `diff` | bare `{"old_sha","new_sha","source","files":[...]}` | [Template Lifecycle (Check, Diff, Update)](#template-lifecycle-check-diff-update) |
| `doctor` | bare `{"status","sections":[...]}` | [Environment Diagnostics](#environment-diagnostics) |
| `extract` | bare `{"template_path","to_path","replacements"}` (+ `content` under `--dry-run`) | [Code Generation Flags](#code-generation-flags) |
| `generate` | bare `{"files":[...],"created","skipped","overwritten","modified","dry_run"}` | [Code Generation Flags](#code-generation-flags) |
| `generate list` | `{"generators":[...],"bundles":[...]}` (identical to `template list`) | [Bundle Prerequisites](#bundle-prerequisites) |
| `lib ls` | `{"templates":[{"name","source","version","added_at","updated_at"}]}` | [Library Management](#library-management) |
| `lib search` | `{"results":[{"name","full_name","description","url","stars","updated_at","language"}]}` | [Template Search](#template-search) |
| `scaffold` | bare `{"output_dir","project_root","template","files":[...],"created","dry_run"}` | [Code Generation Flags](#code-generation-flags) |
| `template graph` | `{"generators":[...],"bundles":[...],"markers":[...],"warnings":[...]}` (also `--format dot`) | [Dependency Graph](#dependency-graph) |
| `template info` | bare `{"name","description","version","variables":[...],"hooks","has_readme","has_howto"}` | [Template Info](#template-info) |
| `template lint` | `{"issues":[{"file","severity","message","rule"}]}` | [Template Linting](#template-linting) |
| `template list` | `{"generators":[...],"bundles":[...]}` (identical to `generate list`) | [Bundle Prerequisites](#bundle-prerequisites) |
| `template variables` | bare `{"root":{...},"generators":[{...}]}` — one `ScopeResult` per scope | [Variable Auditing](#variable-auditing) |
| `test` | bare `{"cases":[...],"passed","failed","errored","total_cases","duration","template_dir"}` | [Matrix Testing](#matrix-testing) |
| `undo` | bare `{"gen_id","files":[...],"reverted","skipped"}` (`--list` → `{"generations":[...]}`) | [Generation History & Undo](#generation-history--undo) |
| `update` | bare `{"mode","dry_run","old_sha","new_sha","up_to_date","files":[...],"new_files","updated_files","deleted_files","conflicts"}` | [Template Lifecycle (Check, Diff, Update)](#template-lifecycle-check-diff-update) |
| `version` | bare `{"version","dev_build"}` (+ `"latest"`/`"update_available"` under `--check`) | [Version](#version) |
