---
name: tag-authoring
description: Create and manage TAG templates, generators, bundles, and scaffold configurations. Use when creating project templates, writing code generators, setting up bundles, converting Cookiecutter templates, or scaffolding new projects with TAG.
---

# TAG Authoring

TAG is a CLI for two workflows: **scaffolding** (creating projects from templates) and **code generation** (evolving projects with generators/bundles).

**Mental model**: Scaffold creates the project. Generators evolve it.

## Quick Start

```bash
# Scaffold a new project (alias: tag s)
tag scaffold gh:user/template my-project

# Initialize generators in an existing project
tag template init
tag template new generator service
tag generate service payment        # alias: tag g service payment

# Run a bundle (multiple generators)
tag generate resource product       # alias: tag g resource product
```

## Decision Tree

```
Preview a template?
  → tag template info <template>

Create a new project?
  → tag scaffold <template> [project-name]
  → tag scaffold                              (no args = interactive picker)

Generate code in existing project?
  Single file? → generator
  Multiple related files? → bundle
  Distributable package? → self-contained bundle

Validate template before publishing?
  → tag template lint [path]

Rename a template variable everywhere?
  → tag template rename-var --dry-run <old> <new>   (preview first)
  → tag template rename-var <old> <new>

Which generator creates what, and in what order?
  → tag template graph                              (creates, injects, markers, bundle order)
  → tag template graph --format dot | dot -Tpng -o graph.png

Test all boolean combinations?
  → tag test [template-dir]

Convert Cookiecutter?
  → tag convert cookiecutter <src> -o <dst>
  → Or just scaffold directly (auto-detects)
```

## Generator Anatomy

A generator is a directory under `.tag/` containing template files with frontmatter + body.

Files keep their **natural extension** (`.go`, `.ts`, `.py`) — NOT `.tmpl`.

```
---
to: app/services/{{ name | snake }}.go
---
package services

type {{ name | pascal }}Service struct{}
```

### Frontmatter Fields

| Field | Required | Description |
|-------|----------|-------------|
| `to` | Yes | Output path. Supports `{{ name }}`, `{{ vars.x }}`, filters. |
| `action` | No | `openapi` to structurally merge into an OpenAPI YAML spec file |
| `inject` | No | `true` to inject into existing file |
| `before` | No | Marker string — inject BEFORE this line. Requires `inject: true`. |
| `after` | No | Marker string — inject AFTER this line. Requires `inject: true`. |
| `append` | No | `true` to append to end of file |
| `validate` | No | `true` to run OpenAPI validation after merge. Requires `action: openapi`. |
| `desc` | No | Description for `tag generate list` |
| `notes` | No | Message displayed after generation |

**Frontmatter is NOT YAML** — a simple one-line-per-`key: value` parser. No nesting, no
arrays, and **no block scalars**: `notes: |` followed by an indented line fails with
`malformed metadata line ... (missing colon)`. Keep every value on its own single line.

### Actions

- **Create** (default): Write new file. Fails if file already exists (use `--on-existing` to change behavior).
- **OpenAPI** (`action: openapi`): Structurally merge rendered YAML fragment into an existing OpenAPI spec file. Inserts new paths and schemas, skips identical content (idempotent), errors on conflicts. Preserves comments, anchors, and indentation.
- **Inject**: Insert at marker (`after:` or `before:` with `inject: true`). Indentation-aware: injected content is automatically aligned to the marker's leading whitespace.
- **Append**: Add to end of file (`append: true`).

Execution order: Create → OpenAPI → Inject → Append (files exist before injection/merge).

### Variables

- `{{ name }}` — the CLI argument from `tag generate <gen> <name>`
- `{{ vars.x }}` — meta values from `--meta`/`-m` flags or `.tagconfig.json`
- `{{ vars.operation.* }}` — one OpenAPI operation via `--openapi`/`--operation`; or `{{ vars.operations }}` (a list) via `--operations`/`--operation-tag` (see reference.md → "OpenAPI input")
- `{{ now("20060102150405") }}` — current timestamp (Go format layout). No args = RFC3339.
- **Never bare names**: `{{ project_name }}` does NOT work. Always `{{ vars.project_name }}`.

### Name Shortcuts

The `n.*` namespace carries **case transforms only** — the exact keys below. An
unknown attribute (`n.humanize`, `n.plural`, `n.snake`) renders **empty and silent**,
not an error. For inflection (plural, humanize, singular, …) apply a filter to `name`:
`{{ name | plural }}`.

| Shortcut | Equivalent | Example (`name = "user_service"`) |
|----------|------------|-----------------------------------|
| `{{ n.snake_case }}` | `{{ name \| snake }}` | `user_service` |
| `{{ n.pascal_case }}` | `{{ name \| pascal }}` | `UserService` |
| `{{ n.camel_case }}` | `{{ name \| camel }}` | `userService` |
| `{{ n.kebab_case }}` | `{{ name \| kebab }}` | `user-service` |
| `{{ n.lower_case }}` | `{{ name \| lower }}` | `user_service` |
| `{{ n.upper_case }}` | `{{ name \| upper }}` | `USER_SERVICE` |
| `{{ n.past }}` | `{{ name \| past }}` | `user_serviced` |

## Bundle Anatomy

Bundles run multiple generators sequentially. Stored as JSON in `.tag/_bundles/`.

```json
{
  "name": "resource",
  "generators": [
    { "name": "model" },
    { "name": "service" },
    { "name": "handler" }
  ]
}
```

### Bundle Default Variables

Bundles can define default variables passed to all generators via `vars`. CLI `-m` flags override bundle defaults. Precedence: `.tagconfig.json` variables (base) ← bundle `vars` ← CLI `--meta` (highest).

```json
{
  "name": "crud-tenant",
  "vars": {
    "domain": "tenant"
  },
  "generators": [
    { "name": "model" },
    { "name": "repository" }
  ]
}
```

### Bundle Prerequisites

Bundles and generators can declare `requires` — a list of `.tagconfig.json` variable names that must be present and truthy. If unmet, `tag generate` aborts with an error listing the missing variables.

```json
{
  "name": "crud",
  "vars": { "domain": "tenant" },
  "requires": ["use_db"],
  "generators": [
    { "name": "model" },
    { "name": "repository" }
  ]
}
```

For generators, add `requires` in the generator's `tag.template.json`:

```json
{
  "requires": ["use_docker"],
  "vars": { "port": 8080 }
}
```

`tag generate list` hides generators/bundles with unmet requirements by default. Use `--all` to show everything.

Generators and bundles are **auto-resolved** — no `--bundle` flag needed.

### Self-Contained Bundles

Generators live inside the bundle directory (not root `.tag/`). Distributable and isolated.

```json
{ "name": "examples", "self_contained": true, "generators": [...] }
```

```bash
tag template new bundle examples --self-contained
tag template new generator hello --in-bundle examples
tag generate examples world
```

## Dialect Type-Mapping

Dialects map canonical type names to language-specific types via the `to()` filter, enabling a single template to target multiple languages.

```jinja
{{ field.type | to("go") }}       → time.Time (for datetime)
{{ field.type | to("postgres") }} → UUID (for uuid)
```

**Built-in dialects:** `go`, `postgres`, `mysql`, `typescript`, `openapi`, `protobuf`

**Canonical types:** `string`, `text`, `int`, `int32`, `int64`, `float`, `float32`, `float64`, `bool`, `byte`, `bytes`, `uuid`, `datetime`, `date`, `decimal`, `json`

### Three-Tier Loading

1. **Built-in** — 6 embedded dialects (always available)
2. **User-global** — `~/.local/share/tag/dialects/*.yaml` (personal overrides)
3. **Template-local** — `_dialects/*.yaml` within a template (project-specific)

Later tiers override individual type mappings (deep merge). Unknown types or dialects produce template rendering errors (not silent passthrough).

### Dialect Override Example

```yaml
# _dialects/go.yaml — override Go's uuid mapping
name: go
types:
  uuid: uuid.UUID  # override built-in "string" mapping
```

### CLI

```bash
tag dialect list                        # Show all available dialects
tag dialect show postgres               # Show type mappings for a dialect
tag dialect list --format json          # {"dialects": [{"name","description"}]} — no "source" field
tag dialect show postgres --format json # Bare {"name","description","types"}
```

## Scaffold Templates

For full template syntax, variable types, hooks, and remote references, see [reference.md](reference.md).

For real-world patterns (CRUD bundles, React components, inject patterns), see [recipes.md](recipes.md).

## Project Structure

```
my-project/
├── .tag/
│   ├── _shared/             # Shared fragments ({% include "file.tmpl" %})
│   ├── _bundles/            # Bundle definitions
│   │   └── resource.json
│   ├── my-generator/        # Generator directory
│   │   └── my-generator.go  # Template file
│   └── ...
└── .tagconfig.json          # Created by scaffold (template origin)
```

## CLI Quick Reference

| Command | Description |
|---------|-------------|
| `tag scaffold [template] [name]` | Create project (no args = picker). Alias: `tag s` |
| `tag generate <gen-or-bundle> <name>` | Run generator or bundle (`--format json`). Alias: `tag g` |
| `tag extract --name <n> --as <gen> <file>` | Extract a generator template from an existing source file (`--format json`, `-i` not supported with `--format json`) |
| `tag generate list [--all]` | List generators and bundles (`--format json`). Alias: `tag g list` |
| `tag generate info <name>` | Show JSON metadata for a generator or bundle |
| `tag generate agent-file <format>` | Generate AI agent reference file |
| `tag template init` | Initialize `.tag/` structure |
| `tag template new generator <name>` | Create generator (`--in-bundle`, `--lib`) |
| `tag template new bundle <name>` | Create bundle (`--self-contained`, `--lib`) |
| `tag template info <template>` | Show template details (`--format json`) |
| `tag template lint [path]` | Validate template (schema, syntax, vars) (`--format json`) |
| `tag template variables [path]` | Audit variable declarations vs usage (`--format json`, `--strict`) |
| `tag template rename-var <old> <new> [path]` | Rename a variable across config and templates (`--dry-run`) |
| `tag template graph [path]` | Visualize generator dependencies (`--format text\|json\|dot`) |
| `tag convert cookiecutter <src> -o <dst>` | Convert Cookiecutter template (`--format json`) |
| `tag dialect list` | List available type-mapping dialects (`--format json`) |
| `tag dialect show <name>` | Show type mappings for a dialect (`--format json`) |
| `tag lib add <ref>` | Install template to library |
| `tag lib ls` | List installed templates (`--format json`) |
| `tag lib search [query]` | Search GitHub for templates (`--format json`, `--limit`, `--sort`, `--order`) |
| `tag lib rm <name>` | Remove template |
| `tag lib update [name]` | Update from source |
| `tag test [template-dir]` | Matrix-test all boolean variable combinations (`--format json`) |
| `tag undo` | Revert the last generation (`--format json` requires `--yes`) |
| `tag undo --list` | Show generation history (`--format json` → `{"generations":[...]}`) |
| `tag undo --id <id>` | Revert a specific generation |
| `tag doctor` | Diagnose environment/project/template/library health (`--format json`). Exit 0/1/2 = pass/warn/fail |
| `tag check [--quiet] [--ref REF]` | Check if upstream template changed (`--format json`). Exit 0 = up-to-date, 1 = updates available |
| `tag diff [--stat] [--no-color]` | Show what would change if you ran `tag update` (read-only) (`--format json`) |
| `tag update [--set k=v] [--skip-hooks]` | Apply upstream template changes via 3-way merge (`--format json`) |
| `tag update --continue` | Resume after manual conflict resolution |
| `tag update --abort` | Abort update and restore from backup |
| `tag upgrade` | Self-upgrade to latest release |
| `tag version [--check]` | Print version, check for updates (`--format json`) |

### Key Flags

| Flag | Commands | Description |
|------|----------|-------------|
| `-m key=value` | scaffold, generate | Set variable values |
| `--openapi <path>` | generate | OpenAPI 3.x spec to expose to templates (needs a selector below) |
| `--operation <selector>` | generate | Single operation → `vars.operation.*`: an `operationId` or `"METHOD /path"` |
| `--operations` | generate | All operations → `vars.operations[]` (excludes `--operation`) |
| `--operation-tag <name>` | generate | Operations with this tag → `vars.operations[]` (excludes `--operation`) |
| `--values <file>` | scaffold | Load variables from JSON |
| `--no-input` | scaffold | Skip prompts |
| `--replay` | scaffold | Reuse saved inputs |
| `--accept-hooks` | scaffold | Run hooks without prompting |
| `-l` / `--lib` | template new | Target library template |
| `-B` / `--in-bundle` | template new generator | Create inside bundle |
| `-s` / `--self-contained` | template new bundle | Self-contained bundle |
| `--skip-hooks` | update | Skip all hook execution during update |
| `--accept-hooks` | scaffold, update, test | Run hooks and template-defined test commands |
| `--backup` | update | Create backup before applying (default: true) |
| `--dry-run` / `-d` | scaffold, generate, convert, update, test | Preview without writing |
| `--all` | generate list, template list | Show all generators/bundles, including those with unmet requirements |
| `--on-existing` | generate | Existing file policy: `fail` (default), `skip`, `overwrite` |
| `-v` / `--verbose` | generate | Print per-file operation details in summary |
| `--force` | scaffold | Overwrite existing output |
| `--format text\|json` | doctor, version, check, diff, cache ls, lib ls, lib search, dialect list, dialect show, template info, template lint, template variables, template graph (also `dot`), generate list, template list, test, generate, extract, undo, undo --list, update, convert cookiecutter | Output format (default: `text`). Unknown value = usage error, exit 2 |

## Pitfalls

- **Gonja, not Jinja2**: No `{% do %}` tag. No `tojson`/`wordwrap`/`batch` filters. TAG registers its own filter set.
- **Namespace rules**: Generators use `{{ name }}` + `{{ vars.x }}`. Scaffolds use only `{{ vars.x }}`.
- **Frontmatter is not YAML**: Simple line parser. No nesting, no arrays, no multi-line.
- **File extensions**: Keep natural extensions (`.go`, `.ts`). Never `.tmpl`.
- **Derived variable ordering**: If A depends on B, B must appear first in `tag.template.json`.
- **Empty output**: If template body renders empty, file is still created as empty.
- **Include resolution**: `{% include %}` resolves by basename — `_shared/header.tmpl` → `"header.tmpl"`.
- **Dash-prefixed positional args**: An unrecognised `-x`-style token is a usage error, not a swallowed positional. To pass one literally, put it after another argument and a `--` separator, e.g. `tag lib search go -- -language:java`. A leading `--` (`tag lib search -- -x`) does not work — urfave/cli consumes it before TAG sees it.
