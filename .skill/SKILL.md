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
| `inject` | No | `true` to inject into existing file |
| `before` | No | Marker string — inject BEFORE this line. Requires `inject: true`. |
| `after` | No | Marker string — inject AFTER this line. Requires `inject: true`. |
| `append` | No | `true` to append to end of file |
| `desc` | No | Description for `tag generate list` |
| `notes` | No | Message displayed after generation |

**Frontmatter is NOT YAML** — simple `key: value` line parser. No nesting, no arrays.

### Actions

- **Create** (default): Write new file. Fails if file already exists (use `--on-existing` to change behavior).
- **Inject**: Insert at marker (`after:` or `before:` with `inject: true`).
- **Append**: Add to end of file (`append: true`).

Execution order: Create → Inject → Append (files exist before injection).

### Variables

- `{{ name }}` — the CLI argument from `tag generate <gen> <name>`
- `{{ vars.x }}` — meta values from `--meta`/`-m` flags or `.tagconfig.json`
- **Never bare names**: `{{ project_name }}` does NOT work. Always `{{ vars.project_name }}`.

### Name Shortcuts

| Shortcut | Equivalent | Example (`name = "user_service"`) |
|----------|------------|-----------------------------------|
| `{{ n.snake }}` | `{{ name \| snake }}` | `user_service` |
| `{{ n.pascal }}` | `{{ name \| pascal }}` | `UserService` |
| `{{ n.camel }}` | `{{ name \| camel }}` | `userService` |
| `{{ n.kebab }}` | `{{ name \| kebab }}` | `user-service` |
| `{{ n.plural }}` | `{{ name \| plural }}` | `user_services` |
| `{{ n.singular }}` | `{{ name \| singular }}` | `user_service` |
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
| `tag generate <gen-or-bundle> <name>` | Run generator or bundle. Alias: `tag g` |
| `tag generate list` | List generators and bundles. Alias: `tag g list` |
| `tag template init` | Initialize `.tag/` structure |
| `tag template new generator <name>` | Create generator (`--in-bundle`, `--lib`) |
| `tag template new bundle <name>` | Create bundle (`--self-contained`, `--lib`) |
| `tag template info <template>` | Show template details |
| `tag template lint [path]` | Validate template (schema, syntax, vars) |
| `tag convert cookiecutter <src> -o <dst>` | Convert Cookiecutter template |
| `tag lib add <ref>` | Install template to library |
| `tag lib ls` | List installed templates |
| `tag lib rm <name>` | Remove template |
| `tag lib update [name]` | Update from source |
| `tag undo` | Revert the last generation |
| `tag undo --list` | Show generation history |
| `tag undo --id <id>` | Revert a specific generation |
| `tag update` | Self-update to latest release |
| `tag version [--check]` | Print version, check for updates |

### Key Flags

| Flag | Commands | Description |
|------|----------|-------------|
| `-m key=value` | scaffold, generate | Set variable values |
| `--values <file>` | scaffold | Load variables from JSON |
| `--no-input` | scaffold | Skip prompts |
| `--replay` | scaffold | Reuse saved inputs |
| `--accept-hooks` | scaffold | Run hooks without prompting |
| `-l` / `--lib` | template new | Target library template |
| `-B` / `--in-bundle` | template new generator | Create inside bundle |
| `-s` / `--self-contained` | template new bundle | Self-contained bundle |
| `--dry-run` / `-d` | generate, convert | Preview without writing |
| `--on-existing` | generate | Existing file policy: `fail` (default), `skip`, `overwrite` |
| `-v` / `--verbose` | generate | Print per-file operation details in summary |
| `--force` | scaffold | Overwrite existing output |

## Pitfalls

- **Gonja, not Jinja2**: No `{% do %}` tag. No `tojson`/`wordwrap`/`batch` filters. TAG registers its own filter set.
- **Namespace rules**: Generators use `{{ name }}` + `{{ vars.x }}`. Scaffolds use only `{{ vars.x }}`.
- **Frontmatter is not YAML**: Simple line parser. No nesting, no arrays, no multi-line.
- **File extensions**: Keep natural extensions (`.go`, `.ts`). Never `.tmpl`.
- **Derived variable ordering**: If A depends on B, B must appear first in `tag.template.json`.
- **Empty output**: If template body renders empty, file is still created as empty.
- **Include resolution**: `{% include %}` resolves by basename — `_shared/header.tmpl` → `"header.tmpl"`.
