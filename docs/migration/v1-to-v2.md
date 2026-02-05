# Migration Guide: TAG v1 to v2

This guide covers migrating from TAG v1 (Go `text/template`) to TAG v2 (Jinja2/Gonja).

## Overview

TAG v2 introduces a new template engine based on [Gonja](https://github.com/noirbizarre/gonja), which provides Jinja2-compatible syntax. This change affects:

1. **Template syntax** - Different delimiters and syntax for control flow
2. **Variable access** - How you reference variables
3. **Filters** - Functions become filters
4. **New features** - Template inheritance, macros, and more

## Quick Reference

| Feature | v1 (Go template) | v2 (Jinja2/Gonja) |
|---------|------------------|-------------------|
| Variable | `{{ .Name }}` | `{{ name }}` |
| Meta access | `{{ .Meta.key }}` | `{{ vars.key }}` |
| Name variants | `{{ .N.PascalCase }}` | `{{ n.pascal_case }}` |
| Filter | `{{ .Name \| caseSnake }}` | `{{ name\|snake }}` |
| Conditional | `{{ if .Flag }}...{{ end }}` | `{% if flag %}...{% endif %}` |
| Loop | `{{ range .Items }}...{{ end }}` | `{% for item in items %}...{% endfor %}` |
| Comment | `{{/* comment */}}` | `{# comment #}` |

## Variable Access

### Basic Variables

**v1:**
```go
{{ .Name }}
{{ .Args }}
```

**v2:**
```jinja2
{{ name }}
{{ args }}
```

### Metadata/Custom Variables

**v1:**
```go
{{ .Meta.author }}
{{ .Meta.license }}
```

**v2:**
```jinja2
{{ vars.author }}
{{ vars.license }}
```

### Name Variants

**v1:**
```go
{{ .N.PascalCase }}
{{ .N.CamelCase }}
{{ .N.SnakeCase }}
{{ .N.KebabCase }}
{{ .N.LowerCase }}
{{ .N.UpperCase }}
```

**v2:**
```jinja2
{{ n.pascal_case }}
{{ n.camel_case }}
{{ n.snake_case }}
{{ n.kebab_case }}
{{ n.lower_case }}
{{ n.upper_case }}
```

## Filters (formerly Functions)

### Case Transformations

**v1:**
```go
{{ caseSnake .Name }}
{{ casePascal .Name }}
{{ caseCamel .Name }}
{{ caseKebab .Name }}
{{ caseLower .Name }}
{{ caseTitle .Name }}
```

**v2:**
```jinja2
{{ name|snake }}
{{ name|pascal }}
{{ name|camel }}
{{ name|kebab }}
{{ name|lower }}
{{ name|title }}
```

### Inflections

**v1:**
```go
{{ pluralise .Name }}
{{ singularise .Name }}
{{ ordinalize .Name }}
{{ titleize .Name }}
{{ humanize .Name }}
```

**v2:**
```jinja2
{{ name|plural }}
{{ name|singular }}
{{ name|ordinalize }}
{{ name|titleize }}
{{ name|humanize }}
```

### String Operations

**v1:**
```go
{{ splitByDelimiter .Args "," }}
{{ splitAfterDelimiter .Args "," }}
{{ contains .Name "admin" }}
{{ hasPrefix .Name "user" }}
{{ hasSuffix .Name "Handler" }}
```

**v2:**
```jinja2
{{ args|split:"," }}
{{ args|split:"," }}
{{ name|contains:"admin" }}
{{ name|hasprefix:"user" }}
{{ name|hassuffix:"Handler" }}
```

### Chaining Filters

**v1:**
```go
{{ pluralise (casePascal .Name) }}
```

**v2:**
```jinja2
{{ name|pascal|plural }}
```

## Control Flow

### Conditionals

**v1:**
```go
{{ if .Meta.useDocker }}
  Docker content
{{ end }}

{{ if eq .Meta.license "MIT" }}
  MIT license
{{ else if eq .Meta.license "Apache" }}
  Apache license
{{ else }}
  Other license
{{ end }}
```

**v2:**
```jinja2
{% if vars.use_docker %}
  Docker content
{% endif %}

{% if vars.license == "MIT" %}
  MIT license
{% elif vars.license == "Apache" %}
  Apache license
{% else %}
  Other license
{% endif %}
```

### Boolean Logic

**v1:**
```go
{{ if and .Meta.useDocker .Meta.useCI }}
{{ if or .Meta.useLogging .Meta.useMetrics }}
{{ if not .Meta.skipTests }}
```

**v2:**
```jinja2
{% if vars.use_docker and vars.use_ci %}
{% if vars.use_logging or vars.use_metrics %}
{% if not vars.skip_tests %}
```

### Loops

**v1:**
```go
{{ range $index, $item := .Items }}
  {{ $index }}: {{ $item }}
{{ end }}
```

**v2:**
```jinja2
{% for item in items %}
  {{ loop.index0 }}: {{ item }}
{% endfor %}
```

**Loop Variables in v2:**
- `loop.index` - 1-indexed iteration count
- `loop.index0` - 0-indexed iteration count
- `loop.first` - True on first iteration
- `loop.last` - True on last iteration
- `loop.length` - Total number of items

### Loop with Else

**v1:**
```go
{{ if .Items }}
  {{ range .Items }}...{{ end }}
{{ else }}
  No items
{{ end }}
```

**v2:**
```jinja2
{% for item in items %}
  ...
{% else %}
  No items
{% endfor %}
```

## Comments

**v1:**
```go
{{/* This is a comment */}}
```

**v2:**
```jinja2
{# This is a comment #}
```

## Template Metadata Block

The metadata block syntax remains unchanged:

```
---
to: path/to/output.go
inject: true
after: // marker
---
Template content here
```

However, the template content after `---` now uses Jinja2 syntax.

## New Features in v2

### Template Inheritance

```jinja2
{# base.tmpl #}
<!DOCTYPE html>
<html>
  <head>{% block head %}{% endblock %}</head>
  <body>{% block content %}{% endblock %}</body>
</html>

{# child.tmpl #}
{% extends "base.tmpl" %}
{% block content %}
  <h1>{{ vars.title }}</h1>
{% endblock %}
```

### Macros

```jinja2
{% macro input(name, type="text") %}
<input type="{{ type }}" name="{{ name }}">
{% endmacro %}

{{ input("username") }}
{{ input("password", type="password") }}
```

### Includes

```jinja2
{% include "partials/header.tmpl" %}
```

### Raw Blocks

```jinja2
{% raw %}
This {{ will not }} be processed
{% endraw %}
```

### Set Variables

```jinja2
{% set full_name = vars.first_name ~ " " ~ vars.last_name %}
{{ full_name }}
```

## Migration Steps

### Step 1: Update Variable References

Replace all Go template variable references:

```bash
# Find patterns to update
grep -r "{{ \." _templates/

# Common replacements:
# {{ .Name }} → {{ name }}
# {{ .Meta.X }} → {{ vars.x }}
# {{ .N.PascalCase }} → {{ n.pascal_case }}
```

### Step 2: Convert Functions to Filters

```bash
# Find function calls
grep -r "case\|pluralise\|singularise" _templates/

# Replace patterns:
# {{ caseSnake .X }} → {{ x|snake }}
# {{ pluralise .X }} → {{ x|plural }}
```

### Step 3: Update Control Flow

```bash
# Find Go template control flow
grep -r "{{ if\|{{ range\|{{ end }}" _templates/

# Replace:
# {{ if .X }}...{{ end }} → {% if x %}...{% endif %}
# {{ range .X }}...{{ end }} → {% for x in items %}...{% endfor %}
```

### Step 4: Update Comments

```bash
# Find Go template comments
grep -r "{{/\*" _templates/

# Replace:
# {{/* comment */}} → {# comment #}
```

### Step 5: Test Each Template

```bash
# Test with dry run
tag generate my_generator TestName --dry-run

# Compare output with expected result
```

## Example Migration

### Before (v1)

```go
---
to: internal/handlers/{{ .Name | caseSnake }}_handler.go
---
package handlers

{{/* {{ .N.PascalCase }}Handler handles {{ .Name }} requests */}}

{{ if .Meta.useLogging }}
import "log/slog"
{{ end }}

type {{ .N.PascalCase }}Handler struct {
    {{ if .Meta.useDB }}
    db *Database
    {{ end }}
}

{{ range $method := splitByDelimiter .Args "," }}
func (h *{{ $.N.PascalCase }}Handler) {{ $method | casePascal }}() error {
    {{ if $.Meta.useLogging }}
    slog.Info("{{ $method }} called")
    {{ end }}
    return nil
}
{{ end }}
```

### After (v2)

```jinja2
---
to: internal/handlers/{{ name | snake }}_handler.go
---
package handlers

{# {{ n.pascal_case }}Handler handles {{ name }} requests #}

{% if vars.use_logging %}
import "log/slog"
{% endif %}

type {{ n.pascal_case }}Handler struct {
    {% if vars.use_db %}
    db *Database
    {% endif %}
}

{% for method in args|split:"," %}
func (h *{{ n.pascal_case }}Handler) {{ method|pascal }}() error {
    {% if vars.use_logging %}
    slog.Info("{{ method }} called")
    {% endif %}
    return nil
}
{% endfor %}
```

## Troubleshooting

### Common Errors

| Error | Cause | Solution |
|-------|-------|----------|
| `undefined variable` | Variable not in context | Check variable name spelling and case |
| `unknown filter` | Filter name changed | Use new filter name (e.g., `snake` not `caseSnake`) |
| `unexpected token` | Mixed v1/v2 syntax | Complete the migration for the file |

### Getting Help

If you encounter issues:

1. Check the [Template Syntax Guide](../templates/syntax.md)
2. Review the [Filter Reference](../templates/filters.md)
3. Test with `--dry-run` to see output without writing files

## Backwards Compatibility

- **Generators**: Existing generators need syntax migration
- **Bundles**: Bundle files (JSON) are unchanged
- **Configuration**: `.tagconfig.json` format is unchanged
- **Hooks**: Hook syntax in `.tagconfig.json` is unchanged

## See Also

- [Template Syntax](../templates/syntax.md) - Complete v2 syntax guide
- [Filter Reference](../templates/filters.md) - All available filters
- [Template Authoring](../templates/authoring.md) - Creating new templates
