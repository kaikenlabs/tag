# tag generate

Run a generator or bundle to add code to an existing project.

## Synopsis

```bash
tag generate <generator|bundle> <name> [args] [flags]
tag generate list
```

## Description

The `generate` command runs a generator (or bundle of generators) to add files to your existing project. Unlike `scaffold` which creates new projects, `generate` is for incremental code generation within an existing codebase.

### Generator Resolution

Generators are resolved using a **library-first, local-fallback** strategy:

1. **Library template**: If the project was scaffolded from a library template (recorded in `.tagconfig.json`), generators from that template's `.tag.templates/` directory are checked first.
2. **Local project**: Generators in the project's `.tag.templates/` directory (configured via `TAG_PATH`) are used as a fallback.
3. **Local wins on collision**: If both sources have a generator with the same name, the local version takes precedence.

Generators can:
- Create new files
- Append to existing files
- Inject content before/after markers in files

### Scaffold Variables

When a project was scaffolded from a template, the scaffold-time variables (e.g., `project_name`, `use_docker`) are automatically available in generator templates via the `vars.*` namespace. Generator `--meta` values override scaffold variables on name collision.

## Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `generator` or `bundle` | Yes | Name of the generator or bundle to run |
| `name` | Yes | Name to pass to the template (e.g., `User`, `OrderService`) |
| `args` | No | Additional arguments accessible as `.Args` in templates |

## Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--bundle` | `-b` | Run a bundle instead of a single generator |
| `--meta <key=value>` | `-m` | Pass metadata to templates (repeatable) |
| `--no-hooks` | | Skip execution of pre and post hooks |
| `--dry-run` | `-d` | Preview output without writing files |

## Global Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--path` | `-tp` | `.tag.templates` | Templates directory path |
| `--shared` | `-sp` | `_shared` | Shared templates directory name |

## Subcommands

### `tag generate list`

List all available generators and bundles for the current project.

```bash
tag generate list
tag generate ls    # alias
```

Output shows generators grouped by source (template library vs local project) and bundles. Each generator's description is read from its `tag.template.json` file (if present).

Example output:
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

## Examples

### Basic Generation

```bash
# Generate a handler named "User"
tag generate handler User

# Generate with arguments
tag generate model User "name:string,email:string,age:int"
```

### Using Metadata

```bash
# Pass single metadata value
tag generate handler User -m package=api

# Pass multiple metadata values
tag generate handler User -m package=api -m version=v1
```

### Running Bundles

```bash
# Run a bundle (collection of generators)
tag generate -b scaffold User

# Bundle with arguments
tag generate -b crud UserProfile "name:string"
```

### Dry Run Mode

```bash
# Preview what would be generated
tag generate handler User --dry-run
```

### Using Different Template Paths

```bash
# Custom templates directory
tag generate handler User --path custom.tag.templates
```

## Template Data

Generators receive the following context variables:

| Variable | Type | Description |
|----------|------|-------------|
| `name` | `string` | The name argument passed to the command |
| `vars` | `map[string]any` | Key-value pairs from scaffold variables + `--meta` flags (meta overrides scaffold) |
| `n.pascal_case` | `string` | Name in PascalCase |
| `n.camel_case` | `string` | Name in camelCase |
| `n.snake_case` | `string` | Name in snake_case |
| `n.kebab_case` | `string` | Name in kebab-case |
| `n.lower_case` | `string` | Name in lowercase |
| `n.upper_case` | `string` | Name in UPPERCASE |

> **Note**: The `args` argument is available in the metadata block but not in the template body context.

### Example Template

```
---
to: internal/handlers/{{ name | snake }}_handler.go
---
package handlers

type {{ n.pascal_case }}Handler struct {}

func New{{ n.pascal_case }}Handler() *{{ n.pascal_case }}Handler {
    return &{{ n.pascal_case }}Handler{}
}
```

## Generator vs Bundle

| Feature | Generator | Bundle |
|---------|-----------|--------|
| Creates | One or more related files | Multiple generators' output |
| Location | `.tag.templates/<name>/` | `_bundles/<name>/<name>.bundle.json` |
| Use case | Single concern (handler, model) | Full feature (CRUD, module) |

### Bundle File Format

```json
{
  "generators": [
    { "name": "model" },
    { "name": "handler" },
    { "name": "service" }
  ]
}
```

## Configuration

Generator behavior is configured via `.tagconfig.json` in your project root. This file is created automatically by `tag init` or `tag scaffold`/`tag run`.

### Scaffolded Project

When a project is scaffolded from a library template, `.tagconfig.json` includes template origin and scaffold-time variables:

```json
{
  "template": {
    "source": "gh:acme/nextjs-starter",
    "name": "nextjs-starter",
    "version": "1.2.0"
  },
  "variables": {
    "project_name": "my-app",
    "use_docker": true,
    "router": "chi"
  },
  "env": {
    "TAG_PATH": ".tag.templates",
    "TAG_SHARED_PATH": "_shared",
    "TAG_BUNDLE_PATH": "_bundles"
  },
  "hooks": {
    "pre": [],
    "post": [
      ["gofmt", "-w", "."],
      ["goimports", "-w", "."]
    ]
  }
}
```

The `template` section tells `tag generate` where to find generators in the library. The `variables` section makes scaffold-time values available to generators via `{{ vars.project_name }}`.

### Locally Initialized Project

When created via `tag init`, the config contains only `env` and `hooks` (no template origin):

```json
{
  "env": {
    "TAG_PATH": ".tag.templates",
    "TAG_SHARED_PATH": "_shared",
    "TAG_BUNDLE_PATH": "_bundles"
  },
  "hooks": {
    "pre": [],
    "post": []
  }
}
```

## Hooks

Hooks defined in `.tagconfig.json` run automatically:

- **Pre-hooks**: Run before generation
- **Post-hooks**: Run after generation (e.g., formatters, linters)

Generate hooks use direct argv execution (no shell interpretation), which is safer than shell-based execution. Each hook has a **5-minute timeout** and output is limited to **1 MB**.

## Template Actions

Templates support three actions via metadata:

### Create (default)
```
---
to: path/to/file.go
---
```

### Append
```
---
to: path/to/file.go
append: true
---
```

### Inject
```
---
to: path/to/file.go
inject: true
after: "// MARKER"
---
```

Or inject before a marker:
```
---
to: path/to/file.go
inject: true
before: "// END MARKER"
---
```

## Error Handling

| Error | Cause | Solution |
|-------|-------|----------|
| "generator not found in template ... or local path" | Generator not found in library template or local `.tag.templates/` | Ensure the template is in the library (`tag lib add <ref>`) and the generator name is correct |
| "generator not found in .tag.templates" | Generator not found locally (no library template configured) | Create the generator in `.tag.templates/` |
| "template not found in library" | `.tagconfig.json` references a template that isn't installed | Run `tag lib add <ref>` to install the template |
| "template version mismatch" | Library template version differs from scaffold-time version | Consider re-scaffolding or running `tag lib update <name>` |
| "cannot open bundle file" | Bundle file not found | Verify bundle exists in `_bundles/` |
| "hook failed" | Pre/post hook returned error | Check hook command and permissions |

## See Also

- [Template Authoring](../templates/authoring.md) - Creating generators
- [Hooks Guide](../templates/hooks.md) - Pre and post hooks
