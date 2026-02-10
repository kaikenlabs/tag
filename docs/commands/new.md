# tag new / tag new-bundle

Create new generators and bundles for code generation.

## Synopsis

```bash
tag new <generator-name> [flags]
tag new-bundle <bundle-name> [flags]
tag nb <bundle-name> [flags]          # alias
```

## Description

The `new` command creates a new generator template file. The `new-bundle` command (alias `nb`) creates a new bundle definition file.

Generators are the building blocks of code generation in TAG. Each generator is a template file with YAML frontmatter that specifies where and how to write output. Bundles group multiple generators to run them together.

## Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `generator-name` / `bundle-name` | Yes | Name of the generator or bundle to create |

## Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--package` | `-k` | `mypackage` | Package name for the generated Go file (`tag new` only) |
| `--lib` | `-l` | `false` | Create in the library template referenced by `.tagconfig.json` |

## tag new

Creates a generator file at `.tag/<name>/<name>.go` with a starter template including all available frontmatter options.

### Generated File

Running `tag new handler -k api` creates `.tag/handler/handler.go`:

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

## tag new-bundle

Creates a bundle definition file at `.tag/_bundles/<name>/<name>.json`.

### Generated File

Running `tag new-bundle scaffold` creates `.tag/_bundles/scaffold/scaffold.json`:

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

## --lib Flag

Both commands support `--lib` / `-l` to create generators or bundles inside a library template instead of the local project. This requires a `.tagconfig.json` with a `template` section pointing to an installed library template.

```bash
# Create generator in the library template
tag new handler --lib

# Create bundle in the library template
tag new-bundle crud --lib
```

The library template is resolved from `cfg.Template.Name` in `.tagconfig.json`, and the generator/bundle is created in that template's `.tag/` directory within the library.

## Examples

```bash
# Create a basic generator
tag new handler

# Create a generator with a custom package
tag new handler -k handlers

# Create a generator in a library template
tag new handler --lib

# Create a bundle
tag new-bundle feature
tag nb feature          # same thing

# Create a bundle in a library template
tag nb crud --lib
```

## See Also

- [Generate Command](generate.md) - Running generators and bundles
- [Template Authoring](../templates/authoring.md) - Creating scaffold templates
- [Filter Reference](../templates/filters.md) - Available template filters
