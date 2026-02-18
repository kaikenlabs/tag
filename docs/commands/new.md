# tag template new

Create new generators and bundles for code generation.

## Synopsis

```bash
tag template new generator <generator-name> [flags]
tag template new bundle <bundle-name> [flags]
```

## Description

The `tag template new generator` command creates a new generator template file. The `tag template new bundle` command creates a new bundle definition file.

Generators are the building blocks of code generation in TAG. Each generator is a template file with YAML frontmatter that specifies where and how to write output. Bundles group multiple generators to run them together.

## Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `generator-name` / `bundle-name` | Yes | Name of the generator or bundle to create |

## Flags

### Generator flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--package` | `-k` | `mypackage` | Package name for the generated Go file |
| `--lib` | `-l` | `false` | Create in the library template referenced by `.tagconfig.json` |
| `--in-bundle` | `-B` | | Create generator inside a bundle directory (for self-contained bundles) |

### Bundle flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--lib` | `-l` | `false` | Create in the library template referenced by `.tagconfig.json` |
| `--self-contained` | `-s` | `false` | Create bundle with `self_contained: true` (generators stored inside the bundle) |

## tag template new generator

Creates a generator file at `.tag/<name>/<name>.go` with a starter template including all available frontmatter options.

### Generated File

Running `tag template new generator handler -k api` creates `.tag/handler/handler.go`:

```
---
to: api/{{ name | snake }}.go
# inject: true
# before: "// marker"
# after: "// marker"
# append: true
# notes: "generator notes"
---
package api

func myFunction() {}
```

The frontmatter includes all available options as comments. Uncomment and configure the ones you need.

### Frontmatter Reference

The `---` block at the top of every generator file controls how and where the output is written.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `to` | string | Yes | Output file path (supports template expressions) |
| `inject` | bool | No | Enable inject mode (insert content at a marker) |
| `before` | string | No | Marker string to inject *before* (requires `inject: true`) |
| `after` | string | No | Marker string to inject *after* (requires `inject: true`) |
| `append` | bool | No | Append content to the end of an existing file |
| `notes` | string | No | Message displayed after generation completes |

Lines starting with `#` are treated as comments and ignored by the parser.

### Actions

There are three mutually exclusive actions. If none is specified, **create** is the default.

**Create** (default) — write a new file:
```yaml
---
to: internal/handlers/{{ name | snake }}.go
---
```

**Append** — add content to the end of an existing file:
```yaml
---
to: internal/routes/routes.go
append: true
---
```

**Inject** — insert content before or after a marker in an existing file:
```yaml
---
to: internal/routes/routes.go
inject: true
after: "// TAG:routes"
---
```

Or inject before a marker:
```yaml
---
to: internal/routes/routes.go
inject: true
before: "// END routes"
---
```

### Validation Rules

- `to` is always required
- `inject` and `append` cannot both be `true`
- `inject: true` requires either `before` or `after` with a non-empty value
- `before` or `after` without `inject: true` are silently ignored

### Template Variables

The `to` field and the template body have access to these variables:

| Variable | Type | Description |
|----------|------|-------------|
| `name` | `string` | The name argument passed to `tag generate` |
| `vars` | `map[string]any` | Scaffold variables + `--meta` flag values |
| `n.pascal_case` | `string` | Name in PascalCase |
| `n.camel_case` | `string` | Name in camelCase |
| `n.snake_case` | `string` | Name in snake_case |
| `n.kebab_case` | `string` | Name in kebab-case |
| `n.lower_case` | `string` | Name in lowercase |
| `n.upper_case` | `string` | Name in UPPERCASE |

## tag template new bundle

Creates a bundle definition file at `.tag/_bundles/<name>/<name>.json`.

### Generated File

Running `tag template new bundle scaffold` creates `.tag/_bundles/scaffold/scaffold.json`:

```json
{"name":"scaffold","generators":[{"name":"myGenerator"}]}
```

Edit this file to list the generators the bundle should run.

### Bundle Format

```json
{
  "name": "my-bundle",
  "generators": [
    { "name": "model" },
    { "name": "handler" },
    { "name": "service" }
  ]
}
```

### Self-Contained Bundles

Use `--self-contained` to create a bundle where generators live inside the bundle directory instead of the project's `.tag/` root:

```bash
# Create a self-contained bundle
tag template new bundle examples --self-contained

# Add generators inside the bundle
tag template new generator hello --in-bundle examples
tag template new generator greet --in-bundle examples
```

This creates generators at `.tag/_bundles/examples/hello/` and `.tag/_bundles/examples/greet/` instead of `.tag/hello/`. Self-contained bundles are distributable and don't depend on project-level generators.

See [Self-Contained Bundles](../../TAG_REFERENCE.md#self-contained-bundles) in the TAG Reference for details.

## --lib Flag

Both subcommands support `--lib` / `-l` to create generators or bundles inside a library template instead of the local project. This requires a `.tagconfig.json` with a `template` section pointing to an installed library template.

```bash
# Create generator in the library template
tag template new generator handler --lib

# Create bundle in the library template
tag template new bundle crud --lib
```

The library template is resolved from `cfg.Template.Name` in `.tagconfig.json`, and the generator/bundle is created in that template's `.tag/` directory within the library.

## Examples

```bash
# Create a basic generator
tag template new generator handler

# Create a generator with a custom package
tag template new generator handler -k handlers

# Create a generator in a library template
tag template new generator handler --lib

# Create a generator inside a self-contained bundle
tag template new generator handler --in-bundle my-bundle

# Create a bundle
tag template new bundle feature

# Create a self-contained bundle
tag template new bundle examples --self-contained

# Create a bundle in a library template
tag template new bundle crud --lib
```

## See Also

- [Template Command](template.md) - Template management overview
- [Generate Command](generate.md) - Running generators and bundles
- [Template Authoring](../templates/authoring.md) - Creating scaffold templates
- [Filter Reference](../templates/filters.md) - Available template filters
