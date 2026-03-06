# tag template

Template management commands.

## Synopsis

```bash
tag template <subcommand> [args] [flags]
```

## Description

The `tag template` command group provides tools for managing templates, generators, and bundles. It consolidates template-related operations under a single namespace.

## Subcommands

| Subcommand | Description |
|------------|-------------|
| `tag template init` | Initialize a TAG directory structure |
| `tag template new generator <name>` | Create a new generator |
| `tag template new bundle <name>` | Create a new bundle |
| `tag template info <template>` | Show template metadata and details |
| `tag template lint [path]` | Validate template syntax, schema, and variable references |
| `tag template variables [path]` | Audit variable declarations and usage across template files |
| `tag template list` | List available generators and bundles |

---

### `tag template init`

Initialize the `.tag/` directory structure in the current project. Use this when adding generators to an existing project that was not scaffolded by TAG.

```bash
tag template init
```

Creates:
```
.tag/
├── _shared/      # Shared template fragments
└── _bundles/     # Bundle definitions
```

See [tag template init](init.md) for full details.

---

### `tag template new generator`

Create a new generator template file.

```bash
tag template new generator <name> [flags]
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--package` | `-p` | `mypackage` | Package name for the generated Go file |
| `--lib` | `-l` | `false` | Create in the library template referenced by `.tagconfig.json` |
| `--in-bundle` | `-B` | | Create generator inside a bundle directory (for self-contained bundles) |

**Examples:**

```bash
# Create a generator locally
tag template new generator handler

# Create with a custom package name
tag template new generator handler -p api

# Create in a library template
tag template new generator handler --lib

# Create inside a self-contained bundle
tag template new generator handler --in-bundle my-bundle
```

See [tag template new](new.md) for full details.

---

### `tag template new bundle`

Create a new bundle definition file.

```bash
tag template new bundle <name> [flags]
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--lib` | `-l` | `false` | Create in the library template referenced by `.tagconfig.json` |
| `--self-contained` | `-s` | `false` | Create bundle with `self_contained: true` (generators inside the bundle) |

**Examples:**

```bash
# Create a bundle locally
tag template new bundle feature

# Create a self-contained bundle
tag template new bundle examples --self-contained

# Create in a library template
tag template new bundle crud --lib
```

See [tag template new](new.md) for full details.

---

### `tag template info`

Show detailed information about a template, including variables, hooks, and metadata.

```bash
tag template info <template>
```

The `<template>` argument can be a local path, remote reference, or library template name.

**Example output:**

```
Name:        go-api
Source:       gh:user/go-api-template
Path:         /Users/you/.local/share/tag/templates/go-api
Version:      v1.2.0
Description:  Go REST API template

Variables:
  author               (string)
  license              (choice: [MIT Apache-2.0 GPL-3.0])
  port                 = 8080
  project_name         (string)
  use_docker           (boolean)

Hooks:
  post_scaffold:
    - go mod tidy
    - git init

Generators:
  handler              Create a request handler
  model                Create a data model
  service              Create a service layer

Bundles:
  crud                 Model + handler + service
```

---

### `tag template lint`

Validate a scaffold template for correctness. Checks `tag.template.json` against the JSON Schema, parses all template files for Gonja syntax errors, and verifies that `{{ vars.* }}` references match declared variables.

```bash
tag template lint [path] [flags]
```

If `[path]` is omitted, the current directory is used.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--format` | `-f` | `text` | Output format: `text` or `json` |

**Exit codes:**

| Code | Meaning |
|------|---------|
| `0` | All checks passed |
| `1` | Lint errors found |
| `2` | Usage error (bad arguments, missing config) |

**Examples:**

```bash
# Lint the current directory
tag template lint

# Lint a specific template
tag template lint ./path/to/template

# Machine-readable output for CI
tag template lint --format json
```

**Example text output:**

```
  tag.template.json  ERROR  config parse error: invalid JSON  (config-parse)
  main.go.tmpl:5  ERROR  undefined variable "db_name"  (undefined-variable)

  2 error(s)
```

**What is checked:**

- **Schema validation** — `tag.template.json` is validated against the TAG JSON Schema.
- **Config parsing** — The config file must be valid JSON and match the expected structure.
- **Template syntax** — All text files are parsed by the Gonja engine (parse-only, no execution).
- **Variable cross-reference** — Every `{{ vars.X }}` and `{% ... vars.X ... %}` reference is checked against the variables declared in `tag.template.json`.
- **Derived variable defaults** — Derived variables referencing other undefined variables are flagged.
- **Path placeholders** — Directory and file names containing `{{ vars.* }}` are checked.
- **Binary files** — Automatically skipped.
- **`.tagignore` patterns** — Ignored files are excluded from linting.
- **Comments** — Template comments (`{# ... #}`) are stripped before variable scanning.

---

### `tag template variables`

Audit variable declarations and usage across all template files. Cross-references `tag.template.json` declarations with `{{ vars.* }}` references in templates. Also scans generator-level configs inside `_generators/`.

```bash
tag template variables [path] [flags]
tag template vars [path] [flags]   # alias
```

If `[path]` is omitted, the current directory is used.

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `text` | Output format: `text` or `json` |
| `--strict` | `false` | Exit with non-zero status when undeclared or unused variables are found |

**Exit codes:**

| Code | Meaning |
|------|---------|
| `0` | Analysis completed (or no issues when `--strict`) |
| `1` | Issues found (`--strict` mode only) |
| `2` | Usage error (bad arguments, missing config) |

**Examples:**

```bash
# Audit the current directory
tag template variables

# Audit a specific template
tag template variables ./my-template

# Machine-readable output for CI
tag template variables --format json

# Fail CI when issues found
tag template variables --strict
```

**Example text output:**

```
Variables declared in tag.template.json:
  author          (string, required)              — used in 3 file(s), 4 reference(s)
  port            (number, default: 8080)         — used in 4 file(s), 6 reference(s)
  project_name    (string, required)              — used in 12 file(s), 23 reference(s)
  use_docker      (boolean, default: true)        — used in 2 file(s), 3 reference(s)
  _slug           (derived)                       — used in 8 file(s), 15 reference(s)

No undeclared variables.

No unused variables.

Summary: 5 declared, 0 undeclared, 0 unused
```

**What is scanned:**

- `{{ vars.x }}` references in template file bodies
- `{{ vars.x }}` references in directory name placeholders
- `{% if vars.x %}` conditional references
- `{% for item in vars.x %}` iteration references
- Derived variable default expressions (`"default": "{{ vars.other }}"`)
- Generator-level `tag.template.json` configs
- Template comments (`{# ... #}`) are stripped before scanning
- Binary files and `.tagignore` patterns are honored

---

### `tag template list`

List all available generators and bundles for the current project.

```bash
tag template list
tag template ls    # alias
```

This is equivalent to `tag generate list` — both show the same output.

Output shows generators grouped by source (template library vs local project) and bundles.

**Example output:**

```
Generators for this project (template: gh:acme/nextjs-starter@v1.2.0)

  TEMPLATE GENERATORS (nextjs-starter)
  component            Create a React component
  page                 Create a new page/route
  api                  Create an API endpoint

  PROJECT GENERATORS
  custom-hook          Custom React hook generator

  BUNDLES
  feature              Component + page + test (template)

Run: tag generate <name> <target> [args]
```

## See Also

- [tag generate](generate.md) - Run generators and bundles
- [tag scaffold](scaffold.md) - Create new projects from templates
- [tag lib](lib.md) - Manage the template library
- [Template Authoring](../templates/authoring.md) - Creating templates
