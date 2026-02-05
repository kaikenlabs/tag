# Template Syntax Guide

TAG uses [Gonja](https://github.com/noirbizarre/gonja), a Jinja2-compatible template engine written in pure Go. This guide covers the template syntax available in TAG.

## Variable Output

Use double curly braces to output variables:

```jinja2
{{ vars.project_name }}
{{ vars.author }}
```

### Variable Namespaces

TAG provides two equivalent namespaces:

```jinja2
{# TAG style (recommended) #}
{{ vars.project_name }}

{# Cookiecutter compatible (also works) #}
{{ cookiecutter.project_name }}
```

Both reference the same underlying variables.

### Name Helpers (Generator Templates Only)

> **Note**: The `n` helper object is only available in **generator templates** (used with `tag generate`), not in scaffold templates.

For generator templates, the `n` object provides pre-computed name variants:

```jinja2
{{ n.pascal_case }}    {# MyProject #}
{{ n.camel_case }}     {# myProject #}
{{ n.snake_case }}     {# my_project #}
{{ n.kebab_case }}     {# my-project #}
{{ n.lower_case }}     {# myproject #}
{{ n.upper_case }}     {# MYPROJECT #}
```

In scaffold templates, use filters instead:

```jinja2
{{ vars.project_name|pascal }}
{{ vars.project_name|snake }}
```

## Filters

Apply filters with the pipe (`|`) operator:

```jinja2
{{ vars.project_name|snake }}
{{ vars.author|upper }}
{{ vars.model|plural|pascal }}
```

See [Filter Reference](filters.md) for all available filters.

### Filter Arguments

Some filters accept arguments with colon syntax:

```jinja2
{{ vars.name|default:"Anonymous" }}
{{ vars.text|truncate:50 }}
{{ vars.text|replace:"old":"new" }}
```

## Control Structures

### Conditionals

```jinja2
{% if vars.use_docker %}
FROM golang:1.21
WORKDIR /app
COPY . .
{% endif %}
```

With else:

```jinja2
{% if vars.use_typescript %}
import { Config } from './types';
{% else %}
const Config = require('./types');
{% endif %}
```

With elif:

```jinja2
{% if vars.database == "postgres" %}
import "database/sql"
import _ "github.com/lib/pq"
{% elif vars.database == "mysql" %}
import "database/sql"
import _ "github.com/go-sql-driver/mysql"
{% else %}
// No database configured
{% endif %}
```

### Boolean Operators

```jinja2
{% if vars.use_docker and vars.use_ci %}
# CI with Docker
{% endif %}

{% if vars.use_logging or vars.use_metrics %}
import "github.com/example/observability"
{% endif %}

{% if not vars.skip_tests %}
go test ./...
{% endif %}
```

### Truthiness

The following values are considered **falsy**:
- `false`
- `0`
- Empty string `""`
- `null` / `nil`
- Empty lists `[]`
- Empty dicts `{}`

Everything else is **truthy**.

### Loops

Iterate over lists:

```jinja2
{% for feature in vars.features %}
- {{ feature }}
{% endfor %}
```

With index:

```jinja2
{% for item in vars.items %}
{{ loop.index }}. {{ item }}
{% endfor %}
```

Loop variables:

| Variable | Description |
|----------|-------------|
| `loop.index` | Current iteration (1-indexed) |
| `loop.index0` | Current iteration (0-indexed) |
| `loop.first` | True if first iteration |
| `loop.last` | True if last iteration |
| `loop.length` | Total number of items |

Iterate over maps:

```jinja2
{% for key, value in vars.config %}
{{ key }}: {{ value }}
{% endfor %}
```

### Loop with Else

Handle empty lists:

```jinja2
{% for user in vars.users %}
- {{ user }}
{% else %}
No users defined.
{% endfor %}
```

## Comments

Single-line comments:

```jinja2
{# This is a comment #}
```

Multi-line comments:

```jinja2
{#
  This is a
  multi-line comment
#}
```

Comments are removed from the output.

## Whitespace Control

Control whitespace with `-` at the beginning or end of tags:

```jinja2
{% for item in items -%}
{{ item }}
{%- endfor %}
```

| Syntax | Effect |
|--------|--------|
| `{%-` | Remove whitespace before the tag |
| `-%}` | Remove whitespace after the tag |
| `{{-` | Remove whitespace before output |
| `-}}` | Remove whitespace after output |

### Example

Without whitespace control:

```jinja2
{% for i in [1, 2, 3] %}
{{ i }}
{% endfor %}
```

Output:
```

1

2

3

```

With whitespace control:

```jinja2
{% for i in [1, 2, 3] -%}
{{ i }}
{% endfor %}
```

Output:
```
1
2
3

```

## Template Inheritance

### Base Template

```jinja2
{# base.tmpl #}
<!DOCTYPE html>
<html>
<head>
  <title>{% block title %}Default Title{% endblock %}</title>
</head>
<body>
  {% block content %}{% endblock %}
</body>
</html>
```

### Child Template

```jinja2
{% extends "base.tmpl" %}

{% block title %}{{ vars.page_title }}{% endblock %}

{% block content %}
<h1>Welcome to {{ vars.project_name }}</h1>
{% endblock %}
```

## Macros

Define reusable template fragments:

```jinja2
{% macro input(name, type="text", value="") %}
<input type="{{ type }}" name="{{ name }}" value="{{ value }}">
{% endmacro %}

{{ input("username") }}
{{ input("password", type="password") }}
{{ input("email", type="email", value=vars.default_email) }}
```

### Macro with Body

```jinja2
{% macro card(title) %}
<div class="card">
  <h3>{{ title }}</h3>
  <div class="body">
    {{ caller() }}
  </div>
</div>
{% endmacro %}

{% call card("Welcome") %}
<p>This is the card content.</p>
{% endcall %}
```

## Includes

Include other template files:

```jinja2
{% include "partials/header.tmpl" %}

<main>
  Content here
</main>

{% include "partials/footer.tmpl" %}
```

## Raw Blocks

Output literal Jinja2 syntax without processing:

```jinja2
{% raw %}
This {{ will not }} be processed
{% endraw %}
```

Useful for documenting templates or generating template files.

## Expressions

### String Concatenation

```jinja2
{{ vars.first_name ~ " " ~ vars.last_name }}
```

### Comparisons

```jinja2
{% if vars.count > 10 %}...{% endif %}
{% if vars.name == "admin" %}...{% endif %}
{% if vars.version != "1.0.0" %}...{% endif %}
{% if vars.level >= 5 %}...{% endif %}
```

### Membership Tests

```jinja2
{% if "admin" in vars.roles %}
  Has admin access
{% endif %}

{% if vars.feature not in vars.disabled_features %}
  Feature is enabled
{% endif %}
```

### Ternary Operator

```jinja2
{{ "Yes" if vars.enabled else "No" }}
{{ vars.count if vars.count > 0 else "None" }}
```

## Set Variables

Define variables within templates:

```jinja2
{% set full_name = vars.first_name ~ " " ~ vars.last_name %}
Hello, {{ full_name }}!

{% set items = ["one", "two", "three"] %}
{% for item in items %}...{% endfor %}
```

## Common Patterns

### Conditional File Generation

Create files based on conditions:

```jinja2
{# Only include Docker content if use_docker is true #}
{% if vars.use_docker %}
FROM golang:1.21-alpine
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . .
RUN go build -o main .
CMD ["./main"]
{% endif %}
```

### Dynamic Imports

```jinja2
package main

import (
    "fmt"
{% if vars.use_logging %}
    "log/slog"
{% endif %}
{% if vars.use_http %}
    "net/http"
{% endif %}
)
```

### List Joining

```jinja2
{{ vars.features|join:", " }}
```

### Default Values

```jinja2
{{ vars.author|default:"Anonymous" }}
{{ vars.port|default:8080 }}
```

### Conditional Classes

```jinja2
<div class="container{% if vars.full_width %} full-width{% endif %}">
```

## Gonja vs Jinja2 Differences

TAG uses Gonja, which is mostly compatible with Jinja2 but has minor differences:

| Feature | Jinja2 | Gonja |
|---------|--------|-------|
| Filter arguments | `{{ x\|default('val') }}` | `{{ x\|default:"val" }}` |
| Format filter | `{{ x\|format('%s') }}` | `{{ x\|format:"%s" }}` |
| Dict iteration | `{% for k, v in dict.items() %}` | `{% for k, v in dict %}` |

## See Also

- [Filter Reference](filters.md) - All available filters
- [Template Authoring](authoring.md) - Creating templates
- [Migration Guide](../migration/v1-to-v2.md) - Migrating from Go templates
