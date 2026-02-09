# tag generate

Run a generator or bundle to add code to an existing project.

## Synopsis

```bash
tag generate <generator|bundle> <name> [args] [flags]
```

## Description

The `generate` command runs a generator (or bundle of generators) to add files to your existing project. Unlike `scaffold` which creates new projects, `generate` is for incremental code generation within an existing codebase.

Generators are defined in the `.tag.templates/` directory and can:
- Create new files
- Append to existing files
- Inject content before/after markers in files

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
| `--dry-run` | `-d` | Preview output without writing files |

## Global Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--path` | `-tp` | `.tag.templates` | Templates directory path |
| `--shared` | `-sp` | `_shared` | Shared templates directory name |

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
| `vars` | `map[string]any` | Key-value pairs from `--meta` flags |
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

Generator behavior can be configured via `.tagconfig.json` in your project root:

```json
{
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
| "generator not found" | Generator directory doesn't exist | Check `.tag.templates/` directory |
| "cannot open bundle file" | Bundle file not found | Verify bundle exists in `_bundles/` |
| "hook failed" | Pre/post hook returned error | Check hook command and permissions |

## See Also

- [Template Authoring](../templates/authoring.md) - Creating generators
- [Hooks Guide](../templates/hooks.md) - Pre and post hooks
